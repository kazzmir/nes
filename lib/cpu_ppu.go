package lib

import "fmt"

// cpuPPUStep is scratch storage, not emulated state. Keep it on CPUState to
// avoid allocating a synchronization context for every instruction.
type cpuPPUStep struct {
	active    bool
	screen    VirtualScreen
	busCycles uint64
	clocked   uint64
	drawn     bool
}

// clockPPUBus counts the accesses implemented by the instruction interpreter.
// Catch the PPU up BEFORE a CPU-visible PPU register access, not after its
// side effects. In particular LDA $2002 reads on its fourth CPU cycle.
func (cpu *CPUState) clockPPUBus(address uint16) {
	if !cpu.ppuStep.active {
		return
	}
	cpu.ppuStep.busCycles++
	if address >= 0x2000 && address < 0x4000 {
		cpu.advancePPU(cpu.ppuStep.busCycles)
	}
}

func (cpu *CPUState) advancePPU(cycles uint64) {
	step := &cpu.ppuStep
	if cycles > step.clocked {
		_, drawn := cpu.PPU.Run((cycles-step.clocked)*3, step.screen)
		step.drawn = step.drawn || drawn
		step.clocked = cycles
	}
}

// Step executes one instruction, one DMA stall cycle, or one interrupt entry,
// and advances the PPU by the elapsed CPU cycles. Callers must not also run the
// PPU or dispatch its NMI themselves. APU execution remains the caller's job.
//
// NMI is edge-latched, then polled before the instruction's final cycle. Late
// edges remain pending until the next instruction's poll; early ones do not
// incur an unconditional extra-instruction delay.
//
// This synchronizes implemented PPU register accesses; it is not a complete
// CPU microcode implementation (e.g. all internal/dummy cycles and interrupt
// hijacking are not yet modeled).
func (cpu *CPUState) Step(table InstructionTable, screen VirtualScreen) (bool, error) {
	if cpu.ppuStep.active {
		return false, fmt.Errorf("recursive CPU Step")
	}
	cpu.ppuStep = cpuPPUStep{active: true, screen: screen}
	defer func() { cpu.ppuStep = cpuPPUStep{} }()
	start := cpu.Cycle
	interrupt := false
	var err error
	switch {
	case cpu.StallCycles > 0:
		err = cpu.Run(table)
	case cpu.NMIReady:
		cpu.NMIReady = false
		interrupt = true
		cpu.ppuStep.busCycles = 2 // Interrupt entry's two initial dummy cycles.
		cpu.NMI()
	case !cpu.GetInterruptDisableFlag() && cpu.IsIRQAsserted():
		interrupt = true
		cpu.ppuStep.busCycles = 2
		cpu.Interrupt()
	default:
		err = cpu.Run(table)
	}
	if err != nil {
		return cpu.ppuStep.drawn, err
	}
	elapsed := cpu.Cycle - start
	if cpu.ppuStep.clocked > elapsed {
		return cpu.ppuStep.drawn, fmt.Errorf("PPU register access exceeds CPU instruction cycles at $%04x", cpu.PC)
	}
	cpu.advancePPU(elapsed)

	// One CPU cycle is three PPU dots. The edge must have arrived by the
	// penultimate cycle's poll. Do not discard a latched edge merely because
	// a subsequent $2002 read or $2000 write deasserted the NMI output.
	if !interrupt && cpu.PPU.NMIPending && cpu.PPU.DotClock-cpu.PPU.NMIEdgeDot >= 3 {
		cpu.NMIReady = true
		cpu.PPU.NMIPending = false
	}
	return cpu.ppuStep.drawn, nil
}

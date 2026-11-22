package lib

import (
	"encoding/json"
	"fmt"
	"testing"
)

func cpuPPUFixture(program ...byte) CPUState {
	prg := make([]byte, 0x8000)
	for i := range prg {
		prg[i] = 0xea // NOP
	}
	copy(prg, program)
	// NMI handler: preserve A, acknowledge vblank, restore A, RTI.
	copy(prg[0x1000:], []byte{0x48, 0xad, 2, 0x20, 0x68, 0x40})
	copy(prg[0x7ffa:], []byte{0, 0x90, 0, 0x80, 0, 0xa0})
	cpu := StartupState()
	cpu.SetMapper(MakeMapper0(prg))
	cpu.PC = 0x8000
	cpu.Cycle = 40000 // Past the PPU power-up write-inhibit interval.
	return cpu
}

// Arrange for vblank to start on the specified dot of the next Step.
func vblankInDots(cpu *CPUState, dots int) {
	position := 241*341 + 1 - (dots - 1)
	cpu.PPU.Scanline = position / 341
	cpu.PPU.ScanlineCycle = position % 341
	cpu.PPU.SetControllerFlags(0x80)
}

func stepCPUForTest(t *testing.T, cpu *CPUState) {
	t.Helper()
	before, dot := cpu.Cycle, cpu.PPU.DotClock
	if _, err := cpu.Step(MakeInstructionDescriptiontable(), MakeVirtualScreen(256, 240)); err != nil {
		t.Fatal(err)
	}
	if got, want := cpu.PPU.DotClock-dot, (cpu.Cycle-before)*3; got != want {
		t.Fatalf("PPU advanced %d dots, want %d", got, want)
	}
	if cpu.ppuStep.active {
		t.Fatal("Step leaked its runtime synchronization context")
	}
}

func TestStepReadsPPUStatusOnDataCycle(t *testing.T) {
	cpu := cpuPPUFixture(0xad, 2, 0x20) // LDA $2002: four CPU cycles.
	vblankInDots(&cpu, 5)               // Flag rises after opcode fetch, before data read.
	stepCPUForTest(t, &cpu)
	if cpu.A&0x80 == 0 || !cpu.GetNegativeFlag() || cpu.PPU.IsVerticalBlankFlagSet() {
		t.Fatal("LDA did not observe and acknowledge vblank on its data-read cycle")
	}
	if !cpu.NMIReady || cpu.PPU.NMIPending {
		t.Fatal("status read lost an already latched, eligible NMI edge")
	}
}

func TestStepEarlyAndLateNMIEdges(t *testing.T) {
	for _, dots := range []int{1, 3, 4, 6} {
		t.Run(fmt.Sprintf("edge_dot_%d", dots), func(t *testing.T) {
			cpu := cpuPPUFixture() // Two-cycle NOPs.
			vblankInDots(&cpu, dots)
			sp := cpu.SP
			stepCPUForTest(t, &cpu)
			early := dots <= 3
			if cpu.NMIReady != early || cpu.PPU.NMIPending == early || cpu.SP != sp {
				t.Fatal("edge was not classified at the penultimate-cycle poll")
			}
			if !early {
				stepCPUForTest(t, &cpu)
				if cpu.PC != 0x8002 || !cpu.NMIReady {
					t.Fatal("late edge did not allow exactly the next instruction to complete")
				}
			}
			before := cpu.Cycle
			stepCPUForTest(t, &cpu)
			if cpu.PC != 0x9000 || cpu.SP != sp-3 || cpu.Cycle-before != 7 || cpu.NMIReady {
				t.Fatal("NMI entry did not take seven cycles without also executing handler code")
			}
		})
	}
}

func TestStepEnablingNMIDuringVblank(t *testing.T) {
	cpu := cpuPPUFixture(0x8d, 0, 0x20, 0xea) // STA $2000; NOP.
	cpu.A = 0x80
	cpu.PPU.Scanline = 250
	cpu.PPU.SetVerticalBlankFlag(true)
	stepCPUForTest(t, &cpu)
	if cpu.NMIReady || !cpu.PPU.NMIPending || cpu.PPU.NMIEdgeDot != 12 {
		t.Fatal("$2000 enable did not latch an edge at the store's final cycle")
	}
	stepCPUForTest(t, &cpu)
	if !cpu.NMIReady || cpu.PC != 0x8004 {
		t.Fatal("NMI enabled during vblank did not wait for the following poll")
	}
	cpu.PPU.SetControllerFlags(0)
	stepCPUForTest(t, &cpu)
	if cpu.PC != 0x9000 {
		t.Fatal("deasserting the output erased an already sampled NMI")
	}
}

func TestStepNMIWaitsForDMAAndPrecedesIRQ(t *testing.T) {
	cpu := cpuPPUFixture()
	cpu.NMIReady = true
	cpu.Stall(3)
	cpu.SetInterruptDisableFlag(false)
	cpu.APU.FrameIRQAsserted = true
	for range 3 {
		stepCPUForTest(t, &cpu)
		if cpu.PC != 0x8000 || !cpu.NMIReady {
			t.Fatal("interrupt serviced during DMA stall")
		}
	}
	stepCPUForTest(t, &cpu)
	if cpu.PC != 0x9000 {
		t.Fatal("pending NMI did not take priority over IRQ after DMA")
	}
}

func TestStepIRQEntryAdvancesPPU(t *testing.T) {
	cpu := cpuPPUFixture()
	cpu.SetInterruptDisableFlag(false)
	cpu.APU.FrameIRQAsserted = true
	before := cpu.Cycle
	stepCPUForTest(t, &cpu)
	if cpu.PC != 0xa000 || cpu.Cycle-before != 7 {
		t.Fatal("IRQ entry executed handler code or failed to advance seven cycles")
	}
}

func TestStepVblankPollingWithAcknowledgingNMI(t *testing.T) {
	cpu := cpuPPUFixture(
		0xad, 2, 0x20, 0x10, 0xfb, // LDA $2002; BPL back to LDA.
		0xa9, 0x42, 0x8d, 0, 2, // LDA #$42; STA $0200.
	)
	vblankInDots(&cpu, 5)
	for range 16 {
		stepCPUForTest(t, &cpu)
		if cpu.Ram[0x200] == 0x42 {
			return
		}
	}
	t.Fatalf("vblank poll starved behind NMI acknowledgement: PC=$%04x", cpu.PC)
}

func TestStepIndirectPPUAccessCycles(t *testing.T) {
	cpu := cpuPPUFixture(0xa1, 0x10) // LDA ($10,X), six cycles including dummy read.
	cpu.X = 4
	cpu.Ram[0x14], cpu.Ram[0x15] = 2, 0x20
	vblankInDots(&cpu, 16)
	stepCPUForTest(t, &cpu)
	if cpu.A&0x80 == 0 || cpu.Cycle != 40006 {
		t.Fatal("indexed-indirect PPU read occurred before its sixth cycle")
	}
	cpu = cpuPPUFixture(0x91, 0x10) // STA ($10),Y, crossing into PPU registers.
	cpu.Ram[0x10], cpu.Ram[0x11] = 0xff, 0x1f
	cpu.Y = 2 // $2001 (PPUMASK).
	cpu.A = 0x1e
	stepCPUForTest(t, &cpu)
	if cpu.PPU.Mask != 0 || cpu.PPU.PendingMask != 0x1e || cpu.PPU.MaskWriteDots != 3 || cpu.Cycle != 40006 {
		t.Fatal("page-crossing indirect store did not queue PPUMASK on its sixth cycle")
	}
}

func TestPPUNMIEdgeIsNotLevelTriggered(t *testing.T) {
	ppu := MakePPU()
	ppu.SetControllerFlags(0x80)
	ppu.SetVerticalBlankFlag(true)
	if !ppu.NMIPending {
		t.Fatal("rising edge was not latched")
	}
	ppu.NMIPending = false // CPU consumed the edge while output remained asserted.
	ppu.SetVerticalBlankFlag(true)
	ppu.SetControllerFlags(0x80)
	if ppu.NMIPending {
		t.Fatal("steady NMI level retriggered an interrupt")
	}
	ppu.SetControllerFlags(0)
	ppu.SetControllerFlags(0x80)
	if !ppu.NMIPending {
		t.Fatal("deassert/reassert did not generate another edge")
	}
}

func TestStepLegacySaveHasNoSpuriousNMI(t *testing.T) {
	cpu := cpuPPUFixture()
	cpu.PPU.Scanline = 250
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "nmiready")
	var ppu map[string]json.RawMessage
	if err := json.Unmarshal(fields["ppu"], &ppu); err != nil {
		t.Fatal(err)
	}
	delete(ppu, "dotclock")
	delete(ppu, "nmipending")
	delete(ppu, "nmiedgedot")
	ppu["flags"], ppu["status"] = json.RawMessage("128"), json.RawMessage("128")
	fields["ppu"], err = json.Marshal(ppu)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var decoded CPUState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	restored := StartupState()
	restored.Load(&decoded)
	restored.Restore()
	stepCPUForTest(t, &restored)
	if restored.NMIReady || restored.PPU.NMIPending || restored.PC != 0x8001 {
		t.Fatal("loading a legacy save with NMI output high invented a new edge")
	}
}

func TestStepPendingNMICopyAndSaveState(t *testing.T) {
	for _, early := range []bool{false, true} {
		t.Run(fmt.Sprintf("ready_%v", early), func(t *testing.T) {
			cpu := cpuPPUFixture()
			dots := 4
			if early {
				dots = 1
			}
			vblankInDots(&cpu, dots)
			stepCPUForTest(t, &cpu)
			copied := StartupState()
			copied.Load(&cpu)
			data, err := json.Marshal(cpu)
			if err != nil {
				t.Fatal(err)
			}
			var decoded CPUState
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			restored := StartupState()
			restored.Load(&decoded)
			restored.Restore()
			for _, state := range []*CPUState{&copied, &restored} {
				if state.NMIReady != cpu.NMIReady || state.PPU.NMIPending != cpu.PPU.NMIPending || state.PPU.DotClock != cpu.PPU.DotClock || state.PPU.NMIEdgeDot != cpu.PPU.NMIEdgeDot {
					t.Fatal("copy/save state lost pending NMI timing")
				}
				if !early {
					stepCPUForTest(t, state)
				}
				stepCPUForTest(t, state)
				if state.PC != 0x9000 {
					t.Fatal("restored pending edge did not enter NMI")
				}
			}
			if cpu.PC != 0x8001 {
				t.Fatal("clocking a copy affected the original")
			}
		})
	}
}

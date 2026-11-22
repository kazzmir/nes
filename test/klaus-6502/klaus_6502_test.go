package klaus6502

import (
	"path/filepath"
	"testing"

	nes "github.com/kazzmir/nes/lib"
)

func TestBRKTakesSevenCycles(t *testing.T) {
	prg := make([]byte, 0x4000)
	prg[0x3ffd] = 0x80 // Reset to $8000 (BRK).
	prg[0x3fff] = 0x90 // BRK vector at $9000.
	cpu := nes.StartupState()
	cpu.SetMapper(nes.MakeMapper0(prg))
	cpu.Reset()
	before := cpu.Cycle
	if err := cpu.Run(nes.MakeInstructionDescriptiontable()); err != nil {
		t.Fatal(err)
	}
	if cpu.PC != 0x9000 || cpu.Cycle-before != 7 {
		t.Fatalf("BRK ended at $%04X after %d cycles, want $9000 after 7", cpu.PC, cpu.Cycle-before)
	}
}

func TestKlaus6502FunctionalROM(t *testing.T) {
	rom, err := nes.ParseNesFile(filepath.Join("..", "..", "test-roms", "klaus-6502", "6502_functional_test.nes"), false)
	if err != nil {
		t.Fatal(err)
	}
	mapper, err := rom.MakeMapper()
	if err != nil {
		t.Fatal(err)
	}
	cpu := nes.StartupState()
	cpu.SetMapper(mapper)
	cpu.Reset()

	// The ROM exercises CPU semantics including software BRK/RTI. It does
	// not need a running PPU or asynchronous APU interrupts.
	const pass = 0xfff0
	const cycleLimit = 100_000_000
	table := nes.MakeInstructionDescriptiontable()
	for cpu.Cycle < cycleLimit {
		pc := cpu.PC
		if pc == pass {
			t.Logf("PASS at $%04X after %d cycles", pc, cpu.Cycle)
			return
		}
		if err := cpu.Run(table); err != nil {
			t.Fatalf("PC=$%04X test case=$%02X: %v", pc, cpu.Ram[0x200], err)
		}
		if cpu.PC == pc {
			t.Fatalf("failure loop at $%04X, test case=$%02X (A=%02X X=%02X Y=%02X P=%02X)", pc, cpu.Ram[0x200], cpu.A, cpu.X, cpu.Y, cpu.Status)
		}
	}
	t.Fatalf("cycle limit exceeded at $%04X, test case=$%02X", cpu.PC, cpu.Ram[0x200])
}

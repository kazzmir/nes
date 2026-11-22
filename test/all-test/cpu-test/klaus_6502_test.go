package cpu

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
	if err := doKlaus6502Test(filepath.Join("..", "..", "..", klaus6502ROM)); err != nil {
		t.Fatal(err)
	}
}

package lib

import (
	"bytes"
	"log"
	"testing"
)

func TestNSFBankRegistersReadOpenBusWithoutWarnings(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)

	cpu := StartupState()
	mapper := MakeNSFMapper(nil, 0x8000, make([]byte, 8), 0, nil)
	cpu.SetMapper(mapper)
	// The music driver uses STA $5FF8,X, which reads the write-only
	// register before storing even when the address doesn't cross a page.
	copy(cpu.Ram, []byte{0x9d, 0xf8, 0x5f})
	table := MakeInstructionDescriptiontable()
	for bank := range mapper.Banks {
		address := 0x5ff8 + uint16(bank)
		cpu.Databus = 0xa5
		if got := cpu.LoadMemory(address); got != 0xa5 {
			t.Fatalf("read $%04x returned $%02x, want open bus $a5", address, got)
		}
		cpu.PC, cpu.X, cpu.A = 0, byte(bank), byte(bank+8)
		if err := cpu.Run(table); err != nil {
			t.Fatal(err)
		}
		if !mapper.UseBankSwitch || mapper.Banks[bank] != cpu.A || cpu.Databus != cpu.A {
			t.Fatalf("indexed store to $%04x did not update bank and drive the bus", address)
		}
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected diagnostics: %s", &output)
	}
}

package main

import (
	"bytes"
	"testing"

	nes "github.com/kazzmir/nes/lib"
)

// Validate the buffered $2007 probe separately so a broken probe cannot be
// mistaken for an MMC2 failure. No cartridge or latch implementation is needed.
func TestReadPrefix(t *testing.T) {
	cpu := nes.StartupState()
	cpu.Cycle = 30000
	cpu.StoreMemory(0x2000, 0)
	chr := make([]byte, 0x2000)
	for i := range chr {
		chr[i] = byte(i*17 + i/256)
	}
	cpu.PPU.CopyCharacterRom(0, chr, true)
	for _, base := range []uint16{0, 0x1000, 0, 0x1000} {
		trigger(&cpu, 0x1FE8) // Leave unrelated data in the read buffer.
		got := readPrefix(&cpu, base)
		want := chr[int(base) : int(base)+probeSize]
		if !bytes.Equal(got, want) {
			t.Fatalf("incorrect buffered probe at $%04X", base)
		}
	}
}

func TestDistinctBanks(t *testing.T) {
	chr := make([]byte, 5*0x1000)
	// Banks 0 and 1 are duplicates; banks 2..4 differ only deep in the prefix.
	for offset := range 3 {
		bank := offset + 2
		chr[bank*0x1000+0x800] = byte(bank)
	}
	banks, err := distinctBanks(chr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(banks, []byte{0, 2, 3, 4}) {
		t.Fatalf("selected banks %v, expected [0 2 3 4]", banks)
	}
	if _, err := distinctBanks(make([]byte, 4*0x1000)); err == nil {
		t.Fatal("accepted indistinguishable CHR banks")
	}
}

// Run the same latch regression as the CLI without depending on a local ROM.
func TestMMC2Latches(t *testing.T) {
	chr := make([]byte, 4*0x1000)
	for i := range chr {
		chr[i] = byte(i / 0x1000)
	}
	rom := nes.NESFile{Mapper: 9, ProgramRom: make([]byte, 0x20000), CharacterRom: chr}
	_, err := checkMMC2(rom, func(name string, err error) {
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRejectOtherMapper(t *testing.T) {
	_, err := checkMMC2(nes.NESFile{Mapper: 0}, func(string, error) {
		t.Fatal("ran assertions for a non-MMC2 cartridge")
	})
	if err == nil {
		t.Fatal("accepted mapper 0")
	}
}

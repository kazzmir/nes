package screenshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kazzmir/nes/cmd/nes/common"
	nes "github.com/kazzmir/nes/lib"
)

type castlevaniaIIIInput struct{ frame int }

func (input *castlevaniaIIIInput) Get() nes.ButtonMapping {
	out := nes.ButtonMapping{}
	for _, frame := range []int{650, 900, 1050, 1250, 1450} {
		out[nes.ButtonIndexStart] = out[nes.ButtonIndexStart] || (input.frame >= frame && input.frame < frame+4)
	}
	out[nes.ButtonIndexA] = (input.frame >= 1200 && input.frame < 1204) || (input.frame >= 1300 && input.frame < 1304)
	out[nes.ButtonIndexRight] = input.frame >= 1700 && input.frame < 1820
	out[nes.ButtonIndexB] = input.frame >= 1850 && input.frame < 1858
	return out
}

func TestCastlevaniaIIIReachesGameplay(t *testing.T) {
	path := filepath.Join("..", "..", "roms", "Castlevania III - Dracula s Curse (U) .nes")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("local Castlevania III ROM not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	rom, err := nes.ParseNesFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	cpu, err := common.SetupCPU(rom, false, false)
	if err != nil {
		t.Fatal(err)
	}
	input := &castlevaniaIIIInput{}
	cpu.Input = nes.MakeInput(input)
	screen := nes.MakeVirtualScreen(256, 240)
	table := nes.MakeInstructionDescriptiontable()
	var before byte
	for input.frame < 1900 && cpu.Cycle < 60_000_000 {
		last := cpu.Cycle
		drawn, err := cpu.Step(table, screen)
		if err != nil {
			t.Fatal(err)
		}
		cpu.APU.Run(float64(cpu.Cycle-last)/2, 100, &cpu)
		if drawn {
			input.frame++
			if input.frame == 1650 {
				before = cpu.PPU.OAM[3]
			}
		}
	}
	// In the first stage these OAM entries track Trevor. Verify the input
	// moved him, and that the gameplay screen wasn't blank or a static menu.
	colors := map[uint32]bool{}
	for _, pixel := range screen.Buffer {
		colors[pixel] = true
	}
	if input.frame != 1900 || before != 0x18 || cpu.PPU.OAM[3] < 0x60 || len(colors) < 10 {
		t.Fatalf("first-stage movement not observed: frame=%d x=%02x->%02x colors=%d", input.frame, before, cpu.PPU.OAM[3], len(colors))
	}
}

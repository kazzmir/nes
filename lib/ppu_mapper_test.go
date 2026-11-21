package lib

import (
	"encoding/json"
	"testing"
)

// This cartridge deliberately is not a Mapper9. Embedding Mapper supplies the
// unrelated CPU-side methods, which these PPU tests must never call.
type patternMapperStub struct {
	Mapper
	reads []uint16
}

func (mapper *patternMapperStub) ReadCharacter(_ *PPUState, address uint16) byte {
	mapper.reads = append(mapper.reads, address)
	return byte(address)
}

type scanlineMapperStub struct {
	Mapper
	clocks int
}

func (mapper *scanlineMapperStub) Scanline() {
	mapper.clocks++
}

func TestPPUScanlineMapper(t *testing.T) {
	ppu := MakePPU()
	mapper := &scanlineMapperStub{}
	ppu.setMapper(mapper)
	ppu.Mask = 8
	screen := MakeVirtualScreen(256, 240)
	for _, scanline := range []int{0, 260} {
		ppu.Scanline = scanline
		ppu.ScanlineCycle = 340
		ppu.Run(1, screen)
	}
	if mapper.clocks != 2 {
		t.Fatalf("got %d scanline clocks, want 2", mapper.clocks)
	}
	ppu.setMapper(MakeMapper0(make([]byte, 0x8000)))
	if ppu.characterMapper != nil || ppu.scanlineMapper != nil {
		t.Fatal("switching cartridges did not clear optional mapper bindings")
	}
	ppu.updateMapperScanline()
	if mapper.clocks != 2 {
		t.Fatal("clocked the previous cartridge")
	}
}

func TestPPUScanlineMapperCopyAndRestore(t *testing.T) {
	cpu := StartupState()
	mapper := MakeMapper4(make([]byte, 0x8000), make([]byte, 0x2000)).(*Mapper4)
	mapper.IrqCounter = 1
	mapper.IrqEnabled = true
	cpu.SetMapper(mapper)
	copied := cpu.Copy()
	copyMapper := copied.Mapper.Mapper.(*Mapper4)
	if copied.PPU.scanlineMapper != copyMapper {
		t.Fatal("copy did not bind its own scanline mapper")
	}
	copied.PPU.updateMapperScanline()
	if !copyMapper.IrqPending || mapper.IrqPending {
		t.Fatal("copy's scanline clock did not affect only its own mapper")
	}

	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	restoredMapper := restored.Mapper.Mapper.(*Mapper4)
	if restored.PPU.scanlineMapper != restoredMapper {
		t.Fatal("Restore did not bind the restored scanline mapper")
	}
	restored.PPU.updateMapperScanline()
	if !restoredMapper.IrqPending || mapper.IrqPending {
		t.Fatal("restored scanline clock did not affect only its own mapper")
	}
}

func TestPPUCharacterMapper(t *testing.T) {
	ppu := MakePPU()
	mapper := &patternMapperStub{}
	ppu.setMapper(mapper)
	if got := ppu.readPatternMemory(0x1234); got != 0x34 {
		t.Fatalf("pattern read returned $%02x, want $34", got)
	}
	ppu.VideoMemory[0x2000] = 0x56
	if got := ppu.readPatternMemory(0x2000); got != 0x56 || len(mapper.reads) != 1 {
		t.Fatal("non-pattern read was routed to the character mapper")
	}

	ppu.Flags = 8
	ppu.Scanline = 1
	ppu.CurrentSprites = []Sprite{{Tile: 2, Y: 1}}
	ppu.fetchSpritePatterns()
	if len(mapper.reads) != 3 || mapper.reads[1] != 0x1020 || mapper.reads[2] != 0x1028 {
		t.Fatalf("sprite fetch addresses: %v", mapper.reads)
	}
	low, high := ppu.spritePatternBytes(&ppu.CurrentSprites[0], 0x1020)
	if low[0] != 0x20 || high[0] != 0x28 || len(mapper.reads) != 3 {
		t.Fatal("sprite pixels did not use the cached mapper reads")
	}

	// Switching to a cartridge without the capability clears the old binding.
	ppu.setMapper(MakeMapper0(make([]byte, 0x8000)))
	ppu.VideoMemory[0x1234] = 0x78
	if got := ppu.readPatternMemory(0x1234); got != 0x78 || len(mapper.reads) != 3 {
		t.Fatal("ordinary mapper did not restore the pattern-memory fallback")
	}
}

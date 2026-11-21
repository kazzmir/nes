package lib

import "testing"

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

package lib

import (
	"encoding/json"
	"testing"
)

func mmc2CPU(t *testing.T) CPUState {
	t.Helper()
	chr := make([]byte, 4*0x1000)
	for i := range chr {
		chr[i] = byte(0x10 + i/0x1000)
	}
	cpu := StartupState()
	cpu.Cycle = 30000
	cpu.SetMapper(MakeMapper9(make([]byte, 0x20000), chr))
	for i := 0; i < 4; i++ {
		cpu.StoreMemory(uint16(0xb000+i*0x1000), byte(i))
	}
	return cpu
}

func TestMMC2TriggerReadUsesOldBank(t *testing.T) {
	cpu := mmc2CPU(t)
	for _, step := range []struct {
		address uint16
		oldBank byte
		newBank byte
	}{
		{0x0fd8, 0x11, 0x10},
		{0x0fe8, 0x10, 0x11},
		{0x1fd8, 0x13, 0x12},
		{0x1fe8, 0x12, 0x13},
	} {
		if got := cpu.PPU.readPatternMemory(step.address); got != step.oldBank {
			t.Errorf("trigger $%04x returned $%02x, want old bank byte $%02x", step.address, got, step.oldBank)
		}
		if got := cpu.PPU.readPatternMemory(step.address & 0x1000); got != step.newBank {
			t.Errorf("after trigger $%04x: got $%02x, want $%02x", step.address, got, step.newBank)
		}
	}
}

func TestMMC2RenderingFetches(t *testing.T) {
	cpu := mmc2CPU(t)
	cpu.PPU.StoreNametableMemory(0x2000, 0xfd)
	cpu.PPU.VideoAddress = 0
	cpu.PPU.LoadBackgroundTile() // The high plane at $0FD8 selects lower FD.
	mapper := cpu.Mapper.Mapper.(*Mapper9)
	if mapper.Latches[0] != 0xfd {
		t.Fatal("background fetch did not select FD")
	}

	cpu.PPU.Flags = 8 // 8x8 sprites in the upper pattern table.
	cpu.PPU.Scanline = 1
	cpu.PPU.CurrentSprites = []Sprite{
		{Tile: 0xfd, Y: 1, X: 255},
		{Tile: 0, Y: 1},
	}
	cpu.PPU.fetchSpritePatterns()
	if mapper.Latches[1] != 0xfd {
		t.Fatal("sprite fetch did not select FD")
	}
	first, second := cpu.PPU.CurrentSprites[0], cpu.PPU.CurrentSprites[1]
	if first.Pattern[0] != 0x13 || first.Pattern[8] != 0x13 {
		t.Fatal("triggering sprite row did not retain old-bank pattern bytes")
	}
	if second.Pattern[0] != 0x12 || second.Pattern[8] != 0x12 {
		t.Fatal("following sprite did not use the new bank")
	}
	left, right := cpu.PPU.spritePatternBytes(&first, 0x1fd0)
	if left[0] != 0x13 || right[0] != 0x13 {
		t.Fatal("pixel reads lost the cached old-bank row")
	}
}

func TestMMC2CopyAndSaveState(t *testing.T) {
	cpu := mmc2CPU(t)
	cpu.PPU.readPatternMemory(0x0fd8)
	cpu.PPU.readPatternMemory(0x1fd8)
	copyCPU := cpu.Copy()
	if copyCPU.PPU.characterMapper == cpu.PPU.characterMapper {
		t.Fatal("copy shares the original mapper binding")
	}
	copyCPU.PPU.readPatternMemory(0x0fe8)
	if cpu.Mapper.Mapper.(*Mapper9).Latches[0] != 0xfd {
		t.Fatal("copy's latch change affected original")
	}

	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Mapper.Mapper.(*Mapper9).Latches != [2]byte{0xfd, 0xfd} {
		t.Fatal("save state lost latch selections")
	}
	// Restore must rebind the PPU before any register reads or rendering.
	restored.Restore()
	if restored.PPU.characterMapper != restored.Mapper.Mapper.(*Mapper9) {
		t.Fatal("Restore did not bind the PPU to the restored mapper")
	}
	restored.LoadMemory(0x2002)
	restored.StoreMemory(0x2006, 0x0f)
	restored.StoreMemory(0x2006, 0xe8)
	restored.LoadMemory(0x2007)
	if restored.Mapper.Mapper.(*Mapper9).Latches[0] != 0xfe {
		t.Fatal("restored CPU's $2007 read did not update its mapper")
	}
}

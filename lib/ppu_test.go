package lib

import (
	"encoding/json"
	"testing"
)

func TestPPUHiddenSpriteDoesNotWrapToTop(t *testing.T) {
	for _, control := range []byte{0, 0x20} { // 8x8 and 8x16 sprites.
		ppu := MakePPU()
		ppu.SetControllerFlags(control)
		ppu.SetMask(0x14) // Sprites enabled, including the leftmost eight pixels.
		ppu.VideoMemory[0] = 0x80
		ppu.OAM[0] = 0xff
		sprites := ppu.GetSprites()
		if sprites[0].Y != 256 {
			t.Fatalf("hidden sprite decoded to Y=%d", sprites[0].Y)
		}
		if pixel, _, _ := ppu.getSpritePixel(0, 0, sprites); pixel != nil {
			t.Fatalf("hidden sprite rendered on the top line with PPUCTRL=$%02x", control)
		}
		ppu.OAM[0] = 0
		if pixel, _, spriteZero := ppu.getSpritePixel(0, 1, ppu.GetSprites()); pixel == nil || !spriteZero {
			t.Fatalf("visible sprite did not render one line below OAM Y with PPUCTRL=$%02x", control)
		}
	}
}

func TestPPUMaskWriteDoesNotClockVerticalScrollOnDot256(t *testing.T) {
	ppu := MakePPU()
	ppu.Scanline = 14
	ppu.ScanlineCycle = 256
	ppu.VideoAddress = 0x0260
	ppu.TemporaryVideoAddress = 0x0260
	ppu.WriteMemory(PPUMASK, 0x08, 100_000)
	if ppu.IsBackgroundEnabled() || ppu.Databus != 0x08 {
		t.Fatal("expected immediate bus update but delayed rendering enable")
	}

	screen := MakeVirtualScreen(256, 240)
	ppu.Run(2, screen)
	if ppu.IsBackgroundEnabled() || ppu.VideoAddress != 0x0260 {
		t.Fatalf("write took effect too early: mask=$%02x v=$%04x", ppu.Mask, ppu.VideoAddress)
	}
	ppu.Run(1, screen)
	if !ppu.IsBackgroundEnabled() {
		t.Fatal("background not enabled on third dot")
	}
}

func TestPPUMaskDisableStillAllowsPendingScrollClock(t *testing.T) {
	ppu := MakePPU()
	ppu.SetMask(0x08)
	ppu.ScanlineCycle = 256
	ppu.VideoAddress = 0x0260
	ppu.WriteMemory(PPUMASK, 0, 100_000)
	ppu.Run(1, MakeVirtualScreen(256, 240))
	if !ppu.IsBackgroundEnabled() || ppu.VideoAddress != 0x1260 {
		t.Fatalf("write disabled rendering too early: mask=$%02x v=$%04x", ppu.Mask, ppu.VideoAddress)
	}
}

func TestPPUPendingMaskCopyAndSaveState(t *testing.T) {
	ppu := MakePPU()
	ppu.WriteMemory(0x2009, 0x18, 100_000) // Mirror of PPUMASK.
	screen := MakeVirtualScreen(256, 240)
	ppu.Run(1, screen)
	copied := ppu.Copy()
	data, err := json.Marshal(ppu)
	if err != nil {
		t.Fatal(err)
	}
	var restored PPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	for _, state := range []*PPUState{&copied, &restored} {
		state.Run(1, screen)
		if state.Mask != 0 {
			t.Fatal("pending mask applied early after copy/restore")
		}
		state.Run(1, screen)
		if state.Mask != 0x18 || state.MaskWriteDots != 0 {
			t.Fatal("copy/restore lost pending mask write")
		}
	}
	if ppu.Mask != 0 || ppu.MaskWriteDots != 2 {
		t.Fatal("running the copies affected the original")
	}
}

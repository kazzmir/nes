package lib

import (
	"encoding/json"
	"testing"
)

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

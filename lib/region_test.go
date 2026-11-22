package lib

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCartridgeRegion(t *testing.T) {
	for _, tc := range []struct {
		name          string
		nes2          bool
		byte9, byte12 byte
		want          Region
	}{
		{"Game (E).nes", false, 0, 0, RegionPAL},
		{"Game (Europe).nes", false, 0, 0, RegionPAL},
		{"Game (UE) [!].nes", false, 0, 0, RegionNTSC},
		{"Game.nes", false, 1, 0, RegionPAL},
		{"Game.nes", false, 0, 1, RegionNTSC}, // Legacy byte 12 is not timing metadata.
		{"Game (E).nes", true, 0, 0, RegionNTSC},
		{"Game.nes", true, 0, 1, RegionPAL},
		{"Game (PAL).nes", true, 0, 2, RegionPAL},
		{"Game (Europe).nes", true, 0, 3, RegionNTSC}, // No Dendy mode yet.
	} {
		h := make([]byte, 16+16384)
		copy(h, "NES\x1a")
		h[4] = 1
		if tc.nes2 {
			h[7] = 8
		}
		h[9], h[12] = tc.byte9, tc.byte12
		rom, err := ParseNes(bytes.NewReader(h), false, tc.name)
		if err != nil || rom.Region != tc.want {
			t.Errorf("%+v: region=%v err=%v", tc, rom.Region, err)
		}
	}
}

func TestRegionOverride(t *testing.T) {
	for _, input := range []string{"auto", "ntsc", "pal", "unknown"} {
		r, err := ParseRegionOverride(input)
		switch input {
		case "auto":
			if err != nil || r != nil {
				t.Fatal("auto must leave the detected region alone")
			}
		case "ntsc", "pal":
			if err != nil || r == nil || (*r == RegionPAL) != (input == "pal") {
				t.Fatalf("incorrect override for %s", input)
			}
		default:
			if err == nil {
				t.Fatal("invalid region accepted")
			}
		}
	}
}

func TestPALPPURatioAndState(t *testing.T) {
	cpu := StartupState()
	cpu.SetRegion(RegionPAL)
	screen := MakeVirtualScreen(256, 240)
	cpu.ppuStep.screen = screen
	cpu.advancePPU(2)
	if cpu.PPU.DotClock != 6 || cpu.PPU.CPUClockRemainder != 2 {
		t.Fatal("wrong initial PAL phase")
	}
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restore CPUState
	if err := json.Unmarshal(data, &restore); err != nil {
		t.Fatal(err)
	}
	for _, state := range []CPUState{cpu.Copy(), restore} {
		state.ppuStep.screen = screen
		state.advancePPU(3)
		if state.PPU.DotClock != 16 || state.PPU.CPUClockRemainder != 0 || state.APU.Region != RegionPAL || state.CPUFrequency() != CPUSpeedPAL {
			t.Fatal("PAL timing or fractional phase lost after copy/save state")
		}
	}
}

func TestPALNMIPollAccountsForFourDotFinalCycle(t *testing.T) {
	for _, dot := range []int{3, 4} {
		cpu := cpuPPUFixture() // Two-cycle NOP.
		cpu.SetRegion(RegionPAL)
		cpu.PPU.CPUClockRemainder = 3 // The NOP advances 3 dots, then 4.
		vblankInDots(&cpu, dot)
		if _, err := cpu.Step(MakeInstructionDescriptiontable(), MakeVirtualScreen(256, 240)); err != nil {
			t.Fatal(err)
		}
		if cpu.PPU.DotClock != 7 || cpu.NMIReady != (dot == 3) {
			t.Fatalf("edge at dot %d: clocks=%d NMI ready=%v", dot, cpu.PPU.DotClock, cpu.NMIReady)
		}
	}
}

func TestPALPPUVblankAndFrameLength(t *testing.T) {
	ppu := MakePPU()
	ppu.Region = RegionPAL
	screen := MakeVirtualScreen(256, 240)
	ppu.Run(241*341+1, screen)
	if ppu.IsVerticalBlankFlagSet() {
		t.Fatal("PAL vblank set early")
	}
	ppu.Run(70*341, screen)
	if !ppu.IsVerticalBlankFlagSet() || ppu.Scanline != 311 || ppu.ScanlineCycle != 1 {
		t.Fatal("PAL vblank did not last to pre-render")
	}
	ppu.Run(1, screen)
	if ppu.IsVerticalBlankFlagSet() {
		t.Fatal("PAL vblank did not clear at pre-render dot 1")
	}
	ppu.Run(339, screen)
	if ppu.Scanline != 0 || ppu.ScanlineCycle != 0 || ppu.DotClock != 312*341 {
		t.Fatal("PAL frame did not wrap after 312 lines")
	}
}

func TestPALAPUFrameTiming(t *testing.T) {
	for _, fourStep := range []bool{true, false} {
		apu := MakeAPU()
		apu.Region, apu.FrameMode = RegionPAL, fourStep
		apu.Pulse1.Length.Length = 2
		first, last := 16627, 41565
		if fourStep {
			last = 33253
		}
		apu.runFrameCounter(float64(first-1)/2, true)
		if apu.Pulse1.Length.Length != 2 {
			t.Fatal("first half-frame clock too early")
		}
		apu.runFrameCounter(0.5, true)
		if apu.Pulse1.Length.Length != 1 {
			t.Fatal("missing PAL half-frame clock at 16627")
		}
		apu.runFrameCounter(float64(last-first-1)/2, true)
		if apu.Pulse1.Length.Length != 1 || apu.FrameIRQAsserted != fourStep {
			t.Fatal("unexpected PAL state before final half-frame")
		}
		apu.runFrameCounter(0.5, true)
		if apu.Pulse1.Length.Length != 0 {
			t.Fatal("missing last PAL half-frame clock")
		}
	}
}

func TestPALAPURateTables(t *testing.T) {
	apu := MakeAPU()
	apu.Region = RegionPAL
	apu.WriteNoiseMode(0x8f)
	apu.WriteDMCEnable(0x0f)
	if apu.Noise.Mode != 1 || apu.Noise.Timer.Period()+1 != 3778 || apu.DMC.Frequency != 25 {
		t.Fatal("PAL noise or DMC period incorrect")
	}
	apu.WriteNoiseMode(2)
	apu.WriteDMCEnable(0)
	if apu.Noise.Timer.Period()+1 != 14 || apu.DMC.Frequency != 199 {
		t.Fatal("PAL noise or DMC period incorrect")
	}
}

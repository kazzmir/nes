package lib

import (
	"encoding/json"
	"testing"
)

func runAPUCPU(apu *APUState, cycles float64) {
	cpu := StartupState()
	apu.Run(cycles/2, 100, &cpu)
}

func frameResetFixture() APUState {
	apu := MakeAPU()
	apu.FrameMode = true // Four-step sequence, just before a half-frame clock.
	apu.Clock = 1
	apu.Cycles = 3728.25
	apu.EnablePulse1 = true
	apu.Pulse1.Length.Length = 2
	return apu
}

func TestAPUFrameResetPreservesPrecedingClock(t *testing.T) {
	apu := frameResetFixture()
	apu.WriteFrameCounter(0x80, 1) // Five-step reset in four CPU cycles.
	runAPUCPU(&apu, 2)
	if !apu.FrameMode || apu.Clock != 2 || apu.Pulse1.Length.Length != 1 || apu.UpdatedFrameCounter != 2 {
		t.Fatal("pending reset discarded the old sequence's preceding half-frame clock")
	}
	runAPUCPU(&apu, 2)
	if apu.FrameMode || apu.Clock != 0 || apu.Pulse1.Length.Length != 0 || apu.UpdatedFrameCounter != 0 {
		t.Fatal("delayed five-step reset did not issue its own half-frame clock")
	}
}

func TestAPUFrameResetBoundaryOrdering(t *testing.T) {
	apu := frameResetFixture()
	apu.Cycles = 3726.75 // The next half-frame clock coincides with reset.
	apu.WriteFrameCounter(0x80, 1)
	runAPUCPU(&apu, 4)
	if apu.Pulse1.Length.Length != 1 || apu.Clock != 0 || apu.Cycles != 0 {
		t.Fatal("coincident reset double-clocked the old and new sequences")
	}
	apu = frameResetFixture()
	runAPUCPU(&apu, 1)
	if apu.Pulse1.Length.Length != 1 || apu.Clock != 2 {
		t.Fatal("ordinary frame clock at the end of Run was lost")
	}
}

func TestAPUFrameResetKeepsRemainingCycles(t *testing.T) {
	apu := MakeAPU()
	apu.FrameMode = true
	apu.Cycles = 100
	apu.WriteFrameCounter(0, 0) // Four-step reset in three CPU cycles.
	runAPUCPU(&apu, 10)
	if apu.Cycles != 3.5 || apu.Clock != 0 || apu.UpdatedFrameCounter != 0 || !apu.FrameMode {
		t.Fatalf("time after reset was discarded: phase=%v clock=%d delay=%v", apu.Cycles, apu.Clock, apu.UpdatedFrameCounter)
	}
}

func TestAPUFrameResetDelayAndMode(t *testing.T) {
	for _, cycle := range []uint64{0, 1} {
		apu := MakeAPU()
		apu.FrameMode = true
		apu.FrameIRQAsserted = true
		apu.Pulse1.Length.Length = 2
		apu.WriteFrameCounter(0xc0, cycle)
		if apu.FrameIRQAsserted || !apu.InterruptInhibit {
			t.Fatal("IRQ inhibition did not take effect immediately")
		}
		delay := float64(3 + cycle)
		if apu.UpdatedFrameCounter != delay {
			t.Fatal("incorrect parity-dependent frame reset delay")
		}
		runAPUCPU(&apu, delay-1)
		if !apu.FrameMode || apu.Pulse1.Length.Length != 2 {
			t.Fatal("five-step mode/clock took effect before the reset delay")
		}
		runAPUCPU(&apu, 1)
		if apu.FrameMode || apu.Pulse1.Length.Length != 1 || apu.HasPendingFrameMode {
			t.Fatal("five-step mode/clock did not take effect when reset became due")
		}
	}
}

func TestAPUFrameResetBatching(t *testing.T) {
	batched := frameResetFixture()
	batched.WriteFrameCounter(0x80, 1)
	stepped := batched.Copy()
	runAPUCPU(&batched, 12)
	for range 12 {
		runAPUCPU(&stepped, 1)
	}
	if batched.Cycles != stepped.Cycles || batched.Clock != stepped.Clock || batched.FrameMode != stepped.FrameMode || batched.UpdatedFrameCounter != stepped.UpdatedFrameCounter || batched.Pulse1.Length.Length != stepped.Pulse1.Length.Length {
		t.Fatal("frame reset depends on how elapsed CPU cycles are batched")
	}
}

func TestAPUFrameResetReplacementAndSaveState(t *testing.T) {
	apu := frameResetFixture()
	apu.WriteFrameCounter(0x80, 1)
	runAPUCPU(&apu, 1)
	copied := apu.Copy()
	data, err := json.Marshal(apu)
	if err != nil {
		t.Fatal(err)
	}
	var restored APUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	for _, state := range []*APUState{&copied, &restored} {
		runAPUCPU(state, 3)
		if state.FrameMode || state.Pulse1.Length.Length != 0 {
			t.Fatal("copy/save state lost the pending mode or old-sequence timing")
		}
	}
	if !apu.FrameMode || apu.UpdatedFrameCounter != 3 {
		t.Fatal("copy/save-state clocks affected the original")
	}
	// A later $4017 write replaces a reset that has not yet happened.
	apu = MakeAPU()
	apu.FrameMode = true
	apu.Pulse1.Length.Length = 2
	apu.WriteFrameCounter(0x80, 1)
	runAPUCPU(&apu, 1)
	apu.WriteFrameCounter(0, 0)
	runAPUCPU(&apu, 3)
	if !apu.FrameMode || apu.Pulse1.Length.Length != 2 {
		t.Fatal("superseded five-step reset still clocked the length counter")
	}
}

func TestAPUFrameResetLegacySaveState(t *testing.T) {
	apu := MakeAPU()
	apu.FrameMode = true
	apu.UpdatedFrameCounter = 1 // Old saves already put the requested mode here.
	apu.Pulse1.Length.Length = 2
	data, err := json.Marshal(apu)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "pendingframemode")
	delete(fields, "haspendingframemode")
	data, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var restored APUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	runAPUCPU(&restored, 1)
	if !restored.FrameMode || restored.Pulse1.Length.Length != 2 {
		t.Fatal("legacy pending four-step reset was misread as five-step mode")
	}
}

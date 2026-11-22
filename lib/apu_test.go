package lib

import (
	"encoding/json"
	"testing"
)

func TestAPULengthCounterEnableAndHalt(t *testing.T) {
	for channel, name := range []string{"pulse1", "pulse2", "triangle", "noise"} {
		t.Run(name, func(t *testing.T) {
			cpu := StartupState()
			mask := byte(1 << channel)
			control := uint16(0x4000 + 4*channel)
			halt := byte(0x20)
			if name == "triangle" {
				halt = 0x80
			}
			loadLength := func() { cpu.StoreMemory(control+3, 0x18) } // Two half-frame clocks.
			checkActive := func(want bool) {
				t.Helper()
				if got := cpu.LoadMemory(APUStatus)&mask != 0; got != want {
					t.Fatalf("length counter active = %v, want %v", got, want)
				}
			}

			loadLength()
			checkActive(false)
			cpu.StoreMemory(APUStatus, mask)
			checkActive(false)
			cpu.StoreMemory(control, 0x10) // Bit 4 does not halt the counter.
			loadLength()
			cpu.APU.HalfFrame()
			checkActive(true)
			cpu.APU.HalfFrame()
			checkActive(false)

			cpu.StoreMemory(control, halt)
			loadLength()
			cpu.APU.HalfFrame()
			cpu.APU.HalfFrame()
			checkActive(true)
			cpu.StoreMemory(APUStatus, 0)
			checkActive(false)
		})
	}
}

func TestAPUStatusReadExposesIRQFlags(t *testing.T) {
	cpu := StartupState()
	cpu.Databus = 0x20
	cpu.APU.FrameIRQAsserted = true
	cpu.APU.DMC.IRQAsserted = true
	cpu.APU.Pulse1.Length.Length = 1

	if got := cpu.LoadMemory(APUStatus); got != 0xe1 {
		t.Fatalf("first $4015 read = $%02x, want $e1", got)
	}
	if cpu.Databus != 0x20 {
		t.Fatalf("$4015 read changed external data bus to $%02x", cpu.Databus)
	}
	if cpu.APU.FrameIRQAsserted || !cpu.APU.DMC.IRQAsserted {
		t.Fatal("$4015 read must acknowledge frame IRQ but leave DMC IRQ asserted")
	}
	if got := cpu.LoadMemory(APUStatus); got != 0xa1 {
		t.Fatalf("second $4015 read = $%02x, want $a1", got)
	}
	cpu.Databus = 0xc0
	cpu.APU.DMC.IRQAsserted = false
	if got := cpu.LoadMemory(APUStatus); got != 0x01 {
		t.Fatalf("$4015 read with IRQs clear = $%02x, want $01", got)
	}
}

func TestAPUTimerClocksAfterPeriodPlusOne(t *testing.T) {
	timer := Timer{Low: 0x9c}
	timer.Reset()
	if timer.Period() != 0x9c || timer.Run(0x9c) != 0 || timer.Run(1) != 1 || timer.Run(0x9d) != 1 {
		t.Fatal("timer did not clock every period+1 input cycles")
	}
}

func TestAPUPulseSweepZeroShiftPreservesPeriod(t *testing.T) {
	for _, pulse1 := range []bool{true, false} {
		for _, value := range []byte{0x88, 0x80} {
			pulse := Pulse{}
			pulse.ParseSweep(value)
			pulse.Timer.SetPeriod(0x180)
			for range 4 {
				pulse.Sweep.Tick(pulse1, &pulse.Timer)
				if got := pulse.Timer.Period(); got != 0x180 {
					t.Fatalf("pulse1=%v sweep=%#02x changed period to %#x with zero shift", pulse1, value, got)
				}
			}
		}
	}
}

func TestAPUPulseSweepNonzeroShiftUpdatesPeriod(t *testing.T) {
	for _, pulse1 := range []bool{true, false} {
		pulse := Pulse{}
		pulse.ParseSweep(0x89)
		pulse.Timer.SetPeriod(0x180)
		for range 2 {
			pulse.Sweep.Tick(pulse1, &pulse.Timer)
		}
		want := uint16(0xc0)
		if pulse1 {
			want-- // Pulse 1 uses one's-complement negation.
		}
		if got := pulse.Timer.Period(); got != want {
			t.Fatalf("pulse1=%v period=%#x, want %#x", pulse1, got, want)
		}
	}
}

func runAPUCPU(apu *APUState, cycles float64) {
	cpu := StartupState()
	apu.Run(cycles/2, 100, &cpu)
}

func frameResetFixture() APUState {
	apu := MakeAPU()
	apu.FrameMode = true // Four-step sequence, just before a half-frame clock.
	apu.Clock = 1
	apu.Cycles = 3727.5
	apu.EnablePulse1 = true
	apu.Pulse1.Length.Length = 2
	return apu
}

func TestAPUFourStepFrameSchedule(t *testing.T) {
	apu := MakeAPU()
	apu.FrameMode = true
	apu.Pulse1.Length.Length = 4
	apu.Triangle.ControlFlag = true
	apu.Triangle.LinearCounter = 10
	last := 0
	for _, event := range []struct {
		cycle int
		half  bool
	}{
		{7457, false}, {14913, true}, {22371, false}, {29829, true},
		{37287, false}, // 29830 (wrap) + 7457.
	} {
		runAPUCPU(&apu, float64(event.cycle-last-1))
		length, linear := apu.Pulse1.Length.Length, apu.Triangle.LinearCounter
		runAPUCPU(&apu, 1)
		if event.half {
			length--
		}
		if apu.Pulse1.Length.Length != length || apu.Triangle.LinearCounter != linear-1 {
			t.Fatalf("incorrect frame clocks at CPU cycle %d", event.cycle)
		}
		last = event.cycle
	}
}

func TestAPUNTSCFrameIRQWindow(t *testing.T) {
	for _, inhibit := range []bool{false, true} {
		apu := MakeAPU()
		apu.FrameMode = true
		apu.InterruptInhibit = inhibit
		for sequence := range 2 {
			runAPUCPU(&apu, 29827)
			if apu.ReadStatus()&0x40 != 0 {
				t.Fatalf("sequence %d: frame IRQ before cycle 29828", sequence)
			}
			for cycle := 29828; cycle <= 29830; cycle++ {
				runAPUCPU(&apu, 1)
				if got := apu.ReadStatus()&0x40 != 0; got != !inhibit {
					t.Fatalf("sequence %d, cycle %d: frame IRQ = %v, inhibit = %v", sequence, cycle, got, inhibit)
				}
				if apu.ReadStatus()&0x40 != 0 {
					t.Fatalf("sequence %d, cycle %d: status read did not clear frame IRQ", sequence, cycle)
				}
			}
		}
		runAPUCPU(&apu, 1)
		if apu.ReadStatus()&0x40 != 0 {
			t.Fatal("frame IRQ reasserted after cycle 29830")
		}
	}
}

func TestAPUFourStepHalfFramePrecedesDelayedReset(t *testing.T) {
	apu := MakeAPU()
	apu.FrameMode = true
	apu.Pulse1.Length.Length = 2
	runAPUCPU(&apu, 29826)
	if apu.Pulse1.Length.Length != 1 {
		t.Fatal("expected one clock before the last half-frame event")
	}
	apu.WriteFrameCounter(0, 1) // Reset at 29830; the half-frame clock is at 29829.
	runAPUCPU(&apu, 4)
	if apu.Pulse1.Length.Length != 0 || apu.Clock != 0 || apu.Cycles != 0 {
		t.Fatal("delayed reset lost the half-frame clock immediately before it")
	}
}

func TestAPUFiveStepFrameSchedule(t *testing.T) {
	apu := MakeAPU()
	apu.FrameMode = false
	apu.Pulse1.Length.Length = 5
	apu.Triangle.ControlFlag = true
	apu.Triangle.LinearCounter = 10
	last := 0
	for _, event := range []struct {
		cycle, length, linear int
	}{
		{7457, 5, 9}, {14913, 4, 8}, {22371, 4, 7},
		{29829, 4, 7}, {37281, 3, 6}, {44739, 3, 5},
	} {
		runAPUCPU(&apu, float64(event.cycle-last-1))
		length, linear := apu.Pulse1.Length.Length, apu.Triangle.LinearCounter
		runAPUCPU(&apu, 1)
		if int(apu.Pulse1.Length.Length) != event.length || apu.Triangle.LinearCounter != event.linear ||
			(event.cycle != 29829 && linear != event.linear+1) ||
			(event.cycle == 29829 && (int(length) != event.length || linear != event.linear)) {
			t.Fatalf("CPU cycle %d: length %d->%d, linear %d->%d", event.cycle, length, apu.Pulse1.Length.Length, linear, apu.Triangle.LinearCounter)
		}
		if apu.FrameIRQAsserted {
			t.Fatal("five-step sequencer asserted a frame IRQ")
		}
		last = event.cycle
	}
}

func TestAPUFiveStepResetNearHalfFrame(t *testing.T) {
	for _, gap := range []int{0, 1, 2} {
		apu := MakeAPU()
		apu.FrameMode = false
		apu.Clock = 4
		// Final half-frame event is 3726 APU cycles after the idle slot.
		apu.Cycles = 3726 - float64(4-gap)/2
		apu.Pulse1.Length.Length = 3
		apu.WriteFrameCounter(0x80, 1) // Reset in four CPU cycles.
		stepped := apu.Copy()
		runAPUCPU(&apu, 4)
		for range 4 {
			runAPUCPU(&stepped, 1)
		}
		want := byte(2)
		if gap == 2 {
			want = 1 // Two CPU cycles apart: both clocks must occur.
		}
		if apu.Pulse1.Length.Length != want || stepped.Pulse1.Length.Length != want {
			t.Fatalf("gap=%d CPU cycles: length batched=%d stepped=%d, want %d", gap, apu.Pulse1.Length.Length, stepped.Pulse1.Length.Length, want)
		}
	}
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
	apu.Cycles = 3726 // The next half-frame clock coincides with reset.
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

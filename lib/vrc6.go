package lib

// https://www.nesdev.org/wiki/VRC6_audio

import (
    "log"
)

// memory addresses to control the VRC6 audio chip
const VRC6FrequencyControl = 0x9003
const VRC6Pulse1Control = 0x9000
const VRC6Pulse1FrequencyLow = 0x9001
const VRC6Pulse1FrequencyHigh = 0x9002

const VRC6Pulse2Control = 0xA000
const VRC6Pulse2FrequencyLow = 0xA001
const VRC6Pulse2FrequencyHigh = 0xA002

const VRC6SawVolume = 0xb000
const VRC6SawFrequencyLow = 0xB001
const VRC6SawFrequencyHigh = 0xB002

type VRC6Pulse struct {
    Divider Divider
    DutyCycle int
    Duty int
    Volume byte
    Mode bool
    Enabled bool
}

func (pulse *VRC6Pulse) Run(x16 bool, x256 bool) {
    if !pulse.Enabled {
        return
    }

    var amount int16 = 1
    if x256 {
        amount = 256
    } else if x16 {
        amount = 16
    }

    // divider counts down to 0 and then duty cycle is clocked
    if pulse.Divider.Clock(amount) {
        pulse.DutyCycle -= 1
        if pulse.DutyCycle < 0 {
            pulse.DutyCycle = 15
        }
    }
}

func (pulse *VRC6Pulse) SetEnable(enable bool) {
    if pulse.Enabled != enable {
        pulse.DutyCycle = 15
    }

    pulse.Enabled = enable
}

func (pulse *VRC6Pulse) SetFrequencyLow(frequency uint16) {
    high := pulse.Divider.ClockPeriod & 0xf00
    pulse.Divider.ClockPeriod = high | frequency
}

func (pulse *VRC6Pulse) SetFrequencyHigh(frequency uint16) {
    low := pulse.Divider.ClockPeriod & 0xff
    pulse.Divider.ClockPeriod = ((frequency & 0xf) << 8) | low

    log.Printf("Pulse period is now %v", pulse.Divider.ClockPeriod)
}

func (pulse *VRC6Pulse) GenerateSample() byte {
    if !pulse.Enabled {
        return 0
    }

    if pulse.Mode {
        return pulse.Volume
    }

    if pulse.DutyCycle <= pulse.Duty {
        return pulse.Volume
    }

    return 0
}

type VRC6Saw struct {
    Divider Divider
    Enabled bool
    Counter int
    Rate uint8
    Accumulator uint8
    AccumulatorCount int
}

func (saw *VRC6Saw) SetEnable(enable bool) {
    saw.Enabled = enable
    saw.Accumulator = 0
}

func (saw *VRC6Saw) Run(x16 bool, x256 bool) {
    if !saw.Enabled {
        return
    }

    var amount int16 = 1
    if x256 {
        amount = 256
    } else if x16 {
        amount = 16
    }

    if saw.Divider.Clock(amount) {
        saw.Counter += 1
        // clock the accumulator every other cycle
        if saw.Counter >= 2 {
            saw.Counter = 0
            saw.Accumulator += saw.Rate
            saw.AccumulatorCount += 1
            if saw.AccumulatorCount >= 7 {
                saw.AccumulatorCount = 0
                saw.Accumulator = 0
            }
        }
    }
}

func (saw *VRC6Saw) SetFrequencyLow(frequency uint16) {
    high := saw.Divider.ClockPeriod & 0xf00
    saw.Divider.ClockPeriod = high | frequency
}

func (saw *VRC6Saw) SetFrequencyHigh(frequency uint16) {
    low := saw.Divider.ClockPeriod & 0xff
    saw.Divider.ClockPeriod = ((frequency & 0xf) << 8) | low

    log.Printf("Saw period is now %v", saw.Divider.ClockPeriod)
}

func (saw *VRC6Saw) SetVolume(volume int) {
    saw.Rate = uint8(volume) & 0b11_1111
}

func (saw *VRC6Saw) GenerateSample() byte {
    if !saw.Enabled {
        return 0
    }

    return saw.Accumulator >> 3
    // return ((((saw.Accumulator >> 3) & 0x1f)) << 4) * 6 / 8
}

type VRC6Audio struct {
    Pulse1 VRC6Pulse
    Pulse2 VRC6Pulse
    Saw VRC6Saw
    SampleBuffer []float32
    SamplePosition int
    SampleCycles float64

    Halt bool
    X16 bool
    X256 bool
}

func MakeVRC6Audio() *VRC6Audio {
    return &VRC6Audio{
        Pulse1: VRC6Pulse{
            Divider: Divider{
                ClockPeriod: 1 << 12,
                Count: 1 << 12,
            },
            DutyCycle: 15,
            Enabled: true,
        },
        Pulse2: VRC6Pulse{
            Divider: Divider{
                ClockPeriod: 1 << 12,
                Count: 1 << 12,
            },
            DutyCycle: 15,
            Enabled: true,
        },
        Saw: VRC6Saw{
            Enabled: true,
            Divider: Divider{
                ClockPeriod: 1 << 12,
                Count: 1 << 12,
            },
        },
        SampleBuffer: make([]float32, 1024),
    }
}

func (vr6 *VRC6Audio) GenerateSample() float32 {
    pulse1 := vr6.Pulse1.GenerateSample()
    pulse2 := vr6.Pulse2.GenerateSample()
    saw := vr6.Saw.GenerateSample()

    total := pulse1 + pulse2 + saw
    // total = saw
    // output is a 6-bit value

    return float32(total) / float32(1 << 6)
}

func (vrc6 *VRC6Audio) Run(cycles float64, cyclesPerSample float64) []float32 {
    if vrc6.Halt {
        return nil
    }

    vrc6.SampleCycles += cycles

    for cycles > 0 {
        cycles -= 1

        vrc6.Pulse1.Run(vrc6.X16, vrc6.X256)
        vrc6.Pulse2.Run(vrc6.X16, vrc6.X256)
        vrc6.Saw.Run(vrc6.X16, vrc6.X256)
    }

    var out []float32
    if vrc6.SampleCycles >= cyclesPerSample {
        sample := vrc6.GenerateSample()
        for vrc6.SampleCycles >= cyclesPerSample {
            vrc6.SampleCycles -= cyclesPerSample
            vrc6.SampleBuffer[vrc6.SamplePosition] = sample
            vrc6.SamplePosition += 1
            if vrc6.SamplePosition >= len(vrc6.SampleBuffer) {
                out = make([]float32, len(vrc6.SampleBuffer))
                copy(out, vrc6.SampleBuffer)
                vrc6.SamplePosition = 0
            }
        }
    }

    return out
}

// returns true if the address is a VRC6 audio address
func (vrc6 *VRC6Audio) HandleWrite(address uint16, value uint8) bool {
    switch address {
        case VRC6FrequencyControl:
            log.Printf("vrc6 frequency control: 0x%x\n", value)

            halt := (value & 0x1) == 0x1
            x16 := (value & 0x2) == 0x2
            x256 := (value & 0x4) == 0x4

            vrc6.FrequencyControl(halt, x16, x256)

            return true
        case VRC6Pulse1Control:
            log.Printf("vrc6 pulse1 control: 0x%x\n", value)

            volume := byte(value & 0xf)
            duty := byte((value >> 4) & 0x7)
            mode := byte((value >> 7) & 0x1)

            vrc6.Pulse1Control(volume, duty, mode)

            return true
        case VRC6Pulse1FrequencyLow:
            log.Printf("vrc6 pulse1 frequency low: 0x%x\n", value)

            frequency := uint16(value)
            vrc6.Pulse1FrequencyLow(frequency)

            return true
        case VRC6Pulse1FrequencyHigh:
            log.Printf("vrc6 pulse1 frequency high: 0x%x\n", value)

            frequency := uint16(value & 0xf)
            enable := ((value >> 7) & 0x1) == 0x1

            vrc6.Pulse1FrequencyHigh(frequency)
            vrc6.Pulse1SetEnable(enable)

            return true

        case VRC6Pulse2Control:
            log.Printf("vrc6 pulse2 control: 0x%x\n", value)

            volume := byte(value & 0xf)
            duty := byte((value >> 4) & 0x7)
            mode := byte((value >> 7) & 0x1)

            vrc6.Pulse2Control(volume, duty, mode)

            return true
        case VRC6Pulse2FrequencyLow:
            log.Printf("vrc6 pulse2 frequency low: 0x%x\n", value)

            frequency := uint16(value)
            vrc6.Pulse2FrequencyLow(frequency)

            return true
        case VRC6Pulse2FrequencyHigh:
            log.Printf("vrc6 pulse2 frequency high: 0x%x\n", value)

            frequency := uint16(value & 0xf)
            enable := ((value >> 7) & 0x1) == 0x1

            vrc6.Pulse2FrequencyHigh(frequency)
            vrc6.Pulse2SetEnable(enable)

            return true

        case VRC6SawVolume:
            log.Printf("vrc6 saw volume: 0x%x\n", value)

            mask := uint8((1 << 6) - 1)
            volume := int(value & mask)
            vrc6.SawVolume(volume)

            return true
        case VRC6SawFrequencyLow:
            log.Printf("vrc6 saw frequency low: 0x%x\n", value)

            frequency := uint16(value)
            vrc6.SawFrequencyLow(frequency)

            return true
        case VRC6SawFrequencyHigh:
            log.Printf("vrc6 saw frequency high: 0x%x\n", value)

            frequency := uint16(value & 0xf)
            enable := ((value >> 7) & 0x1) == 0x1

            vrc6.SawFrequencyHigh(frequency)
            vrc6.SawSetEnable(enable)

            return true
    }

    return false
}

func (vrc6 *VRC6Audio) FrequencyControl(halt bool, x16 bool, x256 bool) {
    vrc6.Halt = halt
    vrc6.X16 = x16
    vrc6.X256 = x256
}

func (vrc6 *VRC6Audio) Pulse1Control(volume byte, duty byte, mode byte) {
    log.Printf("pulse1 duty %v", duty)
    vrc6.Pulse1.Duty = int(duty)
    vrc6.Pulse1.Volume = volume
    vrc6.Pulse1.Mode = mode == 1
}

func (vrc6 *VRC6Audio) Pulse2Control(volume byte, duty byte, mode byte) {
    log.Printf("pulse2 duty %v", duty)
    vrc6.Pulse2.Duty = int(duty)
    vrc6.Pulse2.Volume = volume
    vrc6.Pulse2.Mode = mode == 1
}

// set the lowest 8 bits of the divider frequency
func (vrc6 *VRC6Audio) Pulse1FrequencyLow(frequency uint16) {
    vrc6.Pulse1.SetFrequencyLow(frequency)
}

// set the highest 4 bits of the divider frequency
func (vrc6 *VRC6Audio) Pulse1FrequencyHigh(frequency uint16) {
    vrc6.Pulse1.SetFrequencyHigh(frequency)
}

func (vrc6 *VRC6Audio) Pulse1SetEnable(enable bool) {
    vrc6.Pulse1.SetEnable(enable)
}   

func (vrc6 *VRC6Audio) Pulse2FrequencyLow(frequency uint16) {
    vrc6.Pulse2.SetFrequencyLow(frequency)
}

func (vrc6 *VRC6Audio) Pulse2FrequencyHigh(frequency uint16) {
    vrc6.Pulse2.SetFrequencyHigh(frequency)
}

func (vrc6 *VRC6Audio) Pulse2SetEnable(enable bool) {
    vrc6.Pulse2.SetEnable(enable)
}   

func (vrc6 *VRC6Audio) SawFrequencyLow(frequency uint16) {
    vrc6.Saw.SetFrequencyLow(frequency)
}

func (vrc6 *VRC6Audio) SawFrequencyHigh(frequency uint16) {
    vrc6.Saw.SetFrequencyHigh(frequency)
}

func (vrc6 *VRC6Audio) SawVolume(volume int) {
    vrc6.Saw.SetVolume(volume)
}

func (vrc6 *VRC6Audio) SawSetEnable(enable bool) {
    vrc6.Saw.SetEnable(enable)
}

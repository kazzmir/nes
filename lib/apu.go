package lib

import (
    "log"
    "math"
)

var ApuDebug int = 0

type Divider struct {
    /* how many input clocks must pass before an output clock is generated */
    ClockPeriod uint16
    Count int16
}

func (divider *Divider) Reset() {
    divider.Count = int16(divider.ClockPeriod)
}

/* returns true if an output clock was generated */
func (divider *Divider) Clock() bool {
    if divider.Count >= 0 {
        divider.Count -= 1
        if divider.Count == -1 {
            divider.Reset()
            return true
        }
    }

    return false
}

type Timer struct {
    Divider Divider
    Cycles float64
    Low uint16
    High uint16
}

func (timer *Timer) Period() uint16 {
    return timer.Divider.ClockPeriod
}

func (timer *Timer) SetPeriod(value uint16){
    timer.Divider.ClockPeriod = value
}

func (timer *Timer) Run(cycles float64) int {
    timer.Cycles += cycles
    count := 0
    for timer.Cycles > 0 {
        if timer.Divider.Clock() {
            count += 1
        }
        timer.Cycles -= 1
    }

    return count
}

func (timer *Timer) Reset(){
    value := (timer.High << 8) | timer.Low
    timer.Divider.ClockPeriod = value + 1
    timer.Divider.Reset()
}

type EnvelopeGenerator struct {
    Divider Divider
    Loop bool
    Disable bool
    Value byte
    Counter byte
}

func (envelope *EnvelopeGenerator) Volume() byte {
    if envelope.Disable {
        return envelope.Value
    }

    return envelope.Counter
}

func (envelope *EnvelopeGenerator) Reset() {
    envelope.Counter = 15
}

func (envelope *EnvelopeGenerator) Tick() {
    clock := envelope.Divider.Clock()
    if clock {
        if envelope.Loop {
            if envelope.Counter == 0 {
                envelope.Counter = 15
            } else {
                envelope.Counter -= 1
            }
        } else {
            if envelope.Counter > 0 {
                envelope.Counter -= 1
            }
        }
    }
}

func (envelope *EnvelopeGenerator) Set(loop bool, disable bool, value byte){
    envelope.Divider.ClockPeriod = uint16(value + 1)
    // envelope.Divider.Reset()
    envelope.Loop = loop
    envelope.Disable = disable
    envelope.Value = value
    envelope.Counter = 15
}

type SquareSequencer struct {
    Duty byte
    Position byte
}

func (sequencer *SquareSequencer) SetDuty(duty byte){
    sequencer.Duty = duty
    // sequencer.Position = 0
}

func (sequencer *SquareSequencer) Run(clocks int){
    value := int(sequencer.Position)
    value -= clocks
    for value < 0 {
        value += 8
    }
    sequencer.Position = byte(value)
}

func (sequencer *SquareSequencer) Value() byte {
    var table []byte
    switch sequencer.Duty {
        case 0:
            table = []byte{0, 0, 0, 0, 0, 0, 0, 1}
        case 1:
            table = []byte{0, 0, 0, 0, 0, 0, 1, 1}
        case 2:
            table = []byte{0, 0, 0, 0, 1, 1, 1, 1}
        case 3:
            table = []byte{1, 1, 1, 1, 1, 1, 0, 0}
    }

    return table[sequencer.Position]
}

type Sweep struct {
    Divider Divider
    Enabled bool
    Negate bool // false is add to period, true is subtract from period
    ShiftCount byte
}

func (sweep *Sweep) Tick(pulse1 bool, timer *Timer){
    if sweep.Enabled {
        if sweep.Divider.Clock() {
            shifted := int(timer.Period() >> sweep.ShiftCount)
            if sweep.Negate {
                if pulse1 {
                    shifted = -shifted - 1
                } else {
                    shifted = -shifted
                }
            }
            value := int(timer.Period()) + shifted
            if value < 0 {
                value = 0
            }
            if value > 0x800 {
                value = 0x800
            }
            timer.SetPeriod(uint16(value))
        }
    }
}

type LengthCounter struct {
    Halt bool
    Length byte
}

func (length *LengthCounter) SetLength(index byte){
    table := []byte{
        10, 254, 20,  2, 40,  4, 80,  6, 160,  8, 60, 10, 14, 12, 26, 14,
        12, 16, 24, 18, 48, 20, 96, 22, 192, 24, 72, 26, 16, 28, 32, 30,
    }
    if int(index) >= len(table) {
        log.Printf("APU: invalid length index %v", index)
        return
    }

    length.Length = table[index]
}

func (length *LengthCounter) Clear(){
    length.Length = 0
}

func (length *LengthCounter) Tick() {
    if length.Halt {
        // length.Length = 0
    } else {
        if length.Length > 0 {
            length.Length -= 1
        }
    }
}

type Pulse struct {
    Name string
    Sweep Sweep
    Timer Timer
    Envelope EnvelopeGenerator
    Length LengthCounter
    Frequency float32
    Phase float32
    Duty byte
    Sequencer SquareSequencer
}

func (pulse *Pulse) Copy() Pulse {
    return *pulse
}

func (pulse *Pulse) ParseSweep(value byte){
    enable := (value >> 7) & 0x1
    period := (value >> 4) & 0x7
    negate := (value >> 3) & 0x1
    shift := value & 0x7
    pulse.Sweep.Enabled = enable == 0x1
    pulse.Sweep.Divider.ClockPeriod = uint16(period + 1)
    pulse.Sweep.Divider.Reset()
    pulse.Sweep.Negate = negate == 0x1
    pulse.Sweep.ShiftCount = shift
    if ApuDebug > 0 {
        log.Printf("APU: write %v sweep value=%v enable=%v period=%v negate=%v shift=%v", pulse.Name, value, enable, period, negate, shift)
    }
}

func (pulse *Pulse) SetDuty(duty byte){
    pulse.Duty = duty
    pulse.Sequencer.SetDuty(duty)
}

func (pulse *Pulse) Run(cycles float64){
    clocks := pulse.Timer.Run(cycles)
    pulse.Sequencer.Run(clocks)
}

func (pulse *Pulse) GenerateSample() byte {
    if pulse.Length.Length == 0 {
        return 0
    }

    if pulse.Timer.Divider.ClockPeriod > 0x7ff || pulse.Timer.Divider.ClockPeriod < 8 {
        return 0
    }

    return pulse.Sequencer.Value() * pulse.Envelope.Volume()
}

type Noise struct {
    Length LengthCounter
    Envelope EnvelopeGenerator
    Mode byte
    Timer Timer
    ShiftRegister uint16
}

func (noise *Noise) Copy() Noise {
    return *noise
}

func (noise *Noise) GenerateSample() byte {
    if noise.Length.Length == 0 {
        return 0
    }

    if noise.ShiftRegister & 0x1 == 0 {
        return 0
    }

    return noise.Envelope.Volume()
}

func (noise *Noise) Run(cycles float64){
    /* I can't find where this is documented but the noise channel
     * seems to run at the CPU clock rather than APU
     */
    clocks := noise.Timer.Run(cycles * 2)
    for clocks > 0 {
        bit0 := byte(noise.ShiftRegister & 0x1)
        var feedbackBit byte
        if noise.Mode == 1 {
            /* get bit 6 */
            feedbackBit = byte((noise.ShiftRegister >> 6) & 0x1)
        } else {
            /* get bit 1 */
            feedbackBit = byte((noise.ShiftRegister >> 1) & 0x1)
        }

        feedback := uint16(bit0 ^ feedbackBit)
        noise.ShiftRegister = (feedback << 14) | (noise.ShiftRegister >> 1)

        clocks -= 1
    }
}

type Triangle struct {
    Timer Timer
    Phase int
    Length LengthCounter
    ControlFlag bool
    LinearCounterReloadFlag bool
    LinearCounterReload int
    LinearCounter int
}

func (triangle *Triangle) Copy() Triangle {
    return *triangle
}

var TriangleWaveForm []byte = []byte{
    15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
    0,  1,  2,  3,  4,  5,  6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

func (triangle *Triangle) Run(cycles float64) {
    /* run at CPU speed, which is 2 cycles per APU cycle */
    clocks := triangle.Timer.Run(cycles * 2)
    triangle.Phase = (triangle.Phase + clocks) % len(TriangleWaveForm)
}

func (triangle *Triangle) TickLengthCounter() {
    triangle.Length.Tick()
}

func (triangle *Triangle) TickLinearCounter() {
    if triangle.LinearCounterReloadFlag {
        triangle.LinearCounter = triangle.LinearCounterReload
    } else {
        if triangle.LinearCounter > 0 {
            triangle.LinearCounter -= 1
        }
    }

    if !triangle.ControlFlag {
        triangle.LinearCounterReloadFlag = false
    }
}

func (triangle *Triangle) GenerateSample() byte {
    if triangle.Timer.Divider.ClockPeriod < 5 {
        return 0
    }

    if triangle.Length.Length > 0 && triangle.LinearCounter > 0 {
        return TriangleWaveForm[triangle.Phase]
    }

    return 0
}

type APUState struct {
    /* APU cycles, 1 apu cycle for every 2 cpu cycles */
    Cycles float64
    /* frame sequencer clock, ticks at 240hz */
    Clock uint64
    /* if true then apu is in 4-step mode that generates interrupts
     * otherwise if false then apu is in 5-step mode with no interrupts
     */
    FrameMode bool
    UpdatedFrameCounter float64
    InterruptInhibit bool
    FrameIRQAsserted bool

    SampleCycles float64
    SampleBuffer []float32
    SamplePosition int

    Pulse1 Pulse
    Pulse2 Pulse
    Triangle Triangle
    Noise Noise
    DMC DMC

    EnableNoise bool
    EnableTriangle bool
    EnablePulse2 bool
    EnablePulse1 bool
}

func (apu *APUState) Copy() APUState {
    return APUState{
        Cycles: apu.Cycles,
        Clock: apu.Clock,
        FrameMode: apu.FrameMode,
        UpdatedFrameCounter: apu.UpdatedFrameCounter,
        InterruptInhibit: apu.InterruptInhibit,
        FrameIRQAsserted: apu.FrameIRQAsserted,
        SampleCycles: apu.SampleCycles,
        SampleBuffer: apu.SampleBuffer,
        SamplePosition: apu.SamplePosition,
        Pulse1: apu.Pulse1.Copy(),
        Pulse2: apu.Pulse2.Copy(),
        Triangle: apu.Triangle.Copy(),
        Noise: apu.Noise.Copy(),
        DMC: apu.DMC.Copy(),
        EnableNoise: apu.EnableNoise,
        EnableTriangle: apu.EnableTriangle,
        EnablePulse2: apu.EnablePulse2,
        EnablePulse1: apu.EnablePulse1,
    }
}

func MakeAPU() APUState {
    return APUState{
        Cycles: 0,
        Clock: 0,
        FrameMode: false,
        SampleBuffer: make([]float32, 1024),
        Pulse1: Pulse{
            Name: "pulse1",
        },
        Pulse2: Pulse{
            Name: "pulse2",
        },
        /* On power-up, the shift register is loaded with the value 1. */
        Noise: Noise{
            ShiftRegister: 1,
        },
        DMC: DMC{
            Silence: true,
            Frequency: 5000, // arbitrary value, just has to be non-zero
        },
    }
}

/* Quarter frame actions: envelope and triangle's linear counter */
func (apu *APUState) QuarterFrame() {
    apu.Pulse1.Envelope.Tick()
    apu.Pulse2.Envelope.Tick()
    apu.Noise.Envelope.Tick()
    apu.Triangle.TickLinearCounter()
}

/* Half frame actions: length counters and sweep units */
func (apu *APUState) HalfFrame() {
    /* TODO: reset clock length counters and sweep units */
    apu.Pulse1.Length.Tick()
    if ApuDebug > 0 {
        log.Printf("Pulse1 length now %v", apu.Pulse1.Length.Length)
    }
    apu.Pulse2.Length.Tick()
    apu.Pulse1.Sweep.Tick(true, &apu.Pulse1.Timer)
    apu.Pulse2.Sweep.Tick(false, &apu.Pulse2.Timer)
    apu.Noise.Length.Tick()
    apu.Triangle.TickLengthCounter()
}

func (apu *APUState) Run(apuCycles float64, cyclesPerSample float64, cpu *CPUState) []float32 {
    apu.Pulse1.Run(apuCycles)
    apu.Pulse2.Run(apuCycles)
    apu.Triangle.Run(apuCycles)
    apu.Noise.Run(apuCycles)
    apu.DMC.Run(apuCycles, cpu)

    apu.Cycles += apuCycles

    if apu.UpdatedFrameCounter > 0 {
        apu.UpdatedFrameCounter -= apuCycles * 2
        if apu.UpdatedFrameCounter <= 0 {
            apu.Cycles = 0
            apu.Clock = 0
            apu.UpdatedFrameCounter = 0

            if ApuDebug > 0 {
                log.Printf("APU: reset frame counter")
            }

            if !apu.FrameMode {
                apu.QuarterFrame()
                apu.HalfFrame()
            }
        }
    }

    /* about 240hz on ntsc
     * cpu hz = 1.789773e6
     * apu hz = cpu hz / 2
     * 1.789773e6 / 2 / 3728.5 = 240.01247
     */
    apuCounter := 3728.5
    for apu.Cycles >= apuCounter {
        apu.Clock += 1
        apu.Cycles -= apuCounter

        if ApuDebug > 1 {
            log.Printf("APU frame counter tick %v", apu.Clock)
        }

        /* mode 0 - 4 step */
        if apu.FrameMode {
            /* every 29830 cycles */
            if apu.Clock % 4 == 0 {
                if !apu.InterruptInhibit {
                    apu.FrameIRQAsserted = true
                }
            }
            if apu.Clock % 2 == 0 {
                apu.HalfFrame()
            }

            /* TOOO: reset clock envelope and triangle linear counter */
            apu.QuarterFrame()
        } else {
            /* mode 1 - 5 step */
            switch apu.Clock % 5 {
                case 0, 1, 2, 4:
                    apu.QuarterFrame()
            }
            switch apu.Clock % 5 {
                case 1, 4:
                    apu.HalfFrame()
            }
        }
    }

    apu.SampleCycles += apuCycles
    var out []float32
    if apu.SampleCycles > cyclesPerSample {
        sample := apu.GenerateSample()
        for apu.SampleCycles >= cyclesPerSample {
            apu.SampleCycles -= cyclesPerSample
            apu.SampleBuffer[apu.SamplePosition] = sample
            apu.SamplePosition += 1
            if apu.SamplePosition >= len(apu.SampleBuffer) {
                apu.SamplePosition = 0
                if out == nil {
                    out = make([]float32, len(apu.SampleBuffer))
                }
                copy(out, apu.SampleBuffer)
            }
        }
    }

    return out
}

type DMC struct {
    Irq bool
    Loop bool
    Frequency float64
    Counter float64
    StartingAddress uint16
    Address uint16
    Length uint16
    BytesRemaining uint16
    OutputLevel byte

    IRQAsserted bool

    // Divider Divider

    Silence bool

    ShiftRegister byte
    BitsRemaining byte

    SampleBuffer byte
}

func (dmc *DMC) Copy() DMC {
    return *dmc
}

func (dmc *DMC) GenerateSample() byte {
    /*
    if dmc.Silence {
        return 0
    }
    */

    return dmc.OutputLevel
}

/* http://wiki.nesdev.com/w/index.php/APU_DMC */
func dmcNTSCRate(value byte) uint16 {
    switch value & 0xf {
        case 0x0: return 428 /* low frequency, 4181.71hz */
        case 0x1: return 380
        case 0x2: return 340
        case 0x3: return 320
        case 0x4: return 286
        case 0x5: return 254
        case 0x6: return 226
        case 0x7: return 214
        case 0x8: return 190
        case 0x9: return 160
        case 0xa: return 142
        case 0xb: return 128
        case 0xc: return 106
        case 0xd: return 84
        case 0xe: return 72
        case 0xf: return 54  /* high frequency, 33143.94hz */
    }

    /* can't get here */
    return 428
}

func (apu *APUState) WriteDMCEnable(value byte) {
    irq_enable := (value >> 7) & 0x1
    loop := (value >> 6) & 0x1
    frequency := value & 0b1111

    if ApuDebug > 0 {
        log.Printf("APU: write dmc enable irq %v loop %v frequency 0x%x", irq_enable, loop, frequency)
    }

    apu.DMC.Irq = irq_enable == 1
    apu.DMC.Loop = loop == 1
    /* these periods are all even numbers because there are 2 CPU cycles in an APU cycle.
     * A rate of 428 means the output level changes every 214 APU cycles.
     */
    apu.DMC.Frequency = float64(dmcNTSCRate(frequency)) / 2.0
    apu.DMC.Counter = 0
}

func (dmc *DMC) Reset() {
    dmc.Silence = false
    dmc.BytesRemaining = dmc.Length
    dmc.Address = dmc.StartingAddress
}

func (dmc *DMC) LoadSample(cpu *CPUState) {
    if dmc.BytesRemaining > 0 {
        if ApuDebug > 0 {
            log.Printf("APU: read dmc sample from 0x%x bytes remaining 0x%x", dmc.Address, dmc.BytesRemaining)
        }
        dmc.SampleBuffer = cpu.LoadMemory(dmc.Address)
        dmc.ShiftRegister = dmc.SampleBuffer
        dmc.BitsRemaining = 8
        /* Stall the cpu for X cycles. FIXME: proper accuracy is
         * 4 cycles if it falls on a CPU read cycle.
         * 3 cycles if it falls on a single CPU write cycle (or the second write of a double CPU write).
         * 4 cycles if it falls on the first write of a double CPU write cycle.[4]
         * 2 cycles if it occurs during an OAM DMA, or on the $4014 write cycle that triggers the OAM DMA.
         * 1 cycle if it occurs on the second-last OAM DMA cycle.
         * 3 cycles if it occurs on the last OAM DMA cycle.
         */
        cpu.Stall(3)
        /* The address is incremented; if it exceeds $FFFF, it is wrapped around to $8000. */
        if dmc.Address < 0xffff {
            dmc.Address += 1
        } else {
            dmc.Address = 0x8000
        }

        if dmc.BytesRemaining > 0 {
            dmc.BytesRemaining -= 1
        }

        if dmc.BytesRemaining == 0 {
            if dmc.Loop {
                dmc.Reset()
            } else {
                if dmc.Irq {
                    dmc.IRQAsserted = true
                }
            }
        }
    } else {
        /* Did not read a sample from memory */
        dmc.Silence = true
    }
}

func (dmc *DMC) Run(cycles float64, cpu *CPUState) {
    dmc.Counter += cycles
    for dmc.Counter >= float64(dmc.Frequency) {
        dmc.Counter -= float64(dmc.Frequency)
        if !dmc.Silence {
            value := dmc.ShiftRegister & 0x1
            /* If the silence flag is clear, the output level changes based on bit 0 of the shift register.
            * If the bit is 1, add 2; otherwise, subtract 2. But if adding or subtracting 2 would cause the
            * output level to leave the 0-127 range, leave the output level unchanged. This means subtract 2
            * only if the current level is at least 2, or add 2 only if the current level is at most 125.
            */
            if value == 1 {
                if dmc.OutputLevel <= 125 {
                    dmc.OutputLevel += 2
                } else {
                    dmc.OutputLevel = 127
                }
            } else {
                if dmc.OutputLevel >= 2 {
                    dmc.OutputLevel -= 2
                } else {
                    dmc.OutputLevel = 0
                }
            }
        }

        dmc.ShiftRegister = dmc.ShiftRegister >> 1
        if dmc.BitsRemaining > 0 {
            dmc.BitsRemaining -= 1
        }

        if dmc.BitsRemaining == 0 {
            dmc.LoadSample(cpu)
        }
    }
}

func (apu *APUState) WriteDMCAddress(value byte) {
    /* Sample address = %11AAAAAA.AA000000 = $C000 + (A * 64) */
    apu.DMC.StartingAddress = 0xc000 + (uint16(value) * 64)
    apu.DMC.Address = apu.DMC.StartingAddress

    if ApuDebug > 0 {
        log.Printf("APU: write dmc address 0x%x", apu.DMC.StartingAddress)
    }
}

func (apu *APUState) WriteDMCLength(value byte) {
    if ApuDebug > 0 {
        log.Printf("APU: write dmc length 0x%x", value)
    }
    /* Sample length = %LLLL.LLLL0001 = (L * 16) + 1 bytes */
    apu.DMC.Length = (uint16(value) * 16) + 1
    apu.DMC.BytesRemaining = apu.DMC.Length
}

func (apu *APUState) WriteDMCLoad(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: write dmc output level %v", value)
    }

    /* The DMC output level is set to D, an unsigned value. */
    apu.DMC.OutputLevel = value & 0b111_1111
}

func (apu *APUState) GenerateSample() float32 {
    var pulseValue float32
    var pulse byte

    if apu.EnablePulse1 {
        pulse += apu.Pulse1.GenerateSample()
    }
    if apu.EnablePulse2 {
        pulse += apu.Pulse2.GenerateSample()
    }

    if pulse == 0 {
        pulseValue = 0
    } else {
        pulseValue = 95.88 / (8128.0 / float32(pulse) + 100)
    }

    var restValue float32

    var triangle float32
    var noise float32
    var dmc float32

    if apu.EnableTriangle {
        triangle = float32(apu.Triangle.GenerateSample()) / 8227.0
    }

    if apu.EnableNoise {
        noise = float32(apu.Noise.GenerateSample()) / 12241.0
    }

    dmc = float32(apu.DMC.GenerateSample()) / 22638.0

    all := triangle + noise + dmc
    if math.Abs(float64(all)) < 0.00001 {
        restValue = 0
    } else {
        restValue = 159.79 / (1.0 / (triangle + noise + dmc) + 100)
    }

    _ = pulseValue
    _ = restValue
    // value := pulseValue
    value := pulseValue + restValue
    // value := restValue
    return value
}

func (apu *APUState) WritePulse1Duty(value byte){
    duty := value >> 6
    loop_envelope := (value >> 5) & 0x1
    length_counter_halt := (value >> 4) & 0x1
    volume := (value & 0xf)

    if ApuDebug > 0 {
        log.Printf("APU: write pulse1 duty value=%v duty=%v loop=%v length=%v volume=%v", value, duty, loop_envelope, length_counter_halt, volume)
    }

    apu.Pulse1.SetDuty(duty)
    apu.Pulse1.Length.Halt = length_counter_halt == 0x1
    apu.Pulse1.Envelope.Set(loop_envelope == 0x1, length_counter_halt == 0x1, volume)
}

func (apu *APUState) WritePulse1Sweep(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: Write pulse1 sweep value=%v", value)
    }
    apu.Pulse1.ParseSweep(value)
}

func (apu *APUState) WritePulse1Timer(value byte){
    apu.Pulse1.Timer.Low = uint16(value)
    apu.Pulse1.Timer.Reset()

    if ApuDebug > 0 {
        log.Printf("APU: write pulse1 timer low %v. Timer is now %v", value, apu.Pulse1.Timer.Period())
    }
}

func (apu *APUState) WritePulse1Length(value byte){
    apu.Pulse1.Timer.High = uint16(value & 7)
    lengthIndex := value >> 3
    apu.Pulse1.Length.SetLength(lengthIndex)
    apu.Pulse1.Sequencer.Position = 0

    if ApuDebug > 0 {
        log.Printf("APU: write pulse1 timer high %v length %v", apu.Pulse1.Timer.High, apu.Pulse1.Length.Length)
    }

    apu.Pulse1.Timer.Reset()
    // frequency := 1.789773e6 / (16.0 * float32(apu.Pulse1.Timer.Divider.ClockPeriod))
    // log.Printf("APU: write pulse1 length value=%v counter=%v period=%v frequency=%v", value, apu.Pulse1.Length.Length, apu.Pulse1.Timer.Divider.ClockPeriod, frequency)
}

func (apu *APUState) WritePulse2Duty(value byte){
    duty := value >> 6
    loop_envelope := (value >> 5) & 0x1
    length_counter_halt := (value >> 4) & 0x1
    volume := (value & 0xf)

    if ApuDebug > 0 {
        log.Printf("APU: write pulse2 duty value=%v duty=%v loop=%v length=%v volume=%v", value, duty, loop_envelope, length_counter_halt, volume)
    }

    apu.Pulse2.SetDuty(duty)
    apu.Pulse2.Length.Halt = length_counter_halt == 0x1
    apu.Pulse2.Envelope.Set(loop_envelope == 0x1, length_counter_halt == 0x1, volume)
}

func (apu *APUState) WritePulse2Sweep(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: write pulse2 sweep %v", value)
    }
    apu.Pulse2.ParseSweep(value)
}

func (apu *APUState) WritePulse2Timer(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: write pulse2 timer %v", value)
    }
    apu.Pulse2.Timer.Low = uint16(value)
    apu.Pulse2.Timer.Reset()
}

func (apu *APUState) WritePulse2Length(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: write pulse2 length %v", value)
    }

    apu.Pulse2.Timer.High = uint16(value & 7)
    lengthIndex := value >> 3
    apu.Pulse2.Length.SetLength(lengthIndex)

    apu.Pulse2.Timer.Reset()

    // frequency := 1.789773e6 / (16.0 * float32(apu.Pulse1.Timer.Divider.ClockPeriod))
    // log.Printf("APU: write pulse2 length value=%v counter=%v period=%v frequency=%v", value, apu.Pulse1.Length.Length, apu.Pulse1.Timer.Divider.ClockPeriod, frequency)
}

func (apu *APUState) WriteTriangleCounter(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: write triangle counter %v", value)
    }
    control := (value >> 7) & 0x1
    apu.Triangle.ControlFlag = control == 1
    apu.Triangle.LinearCounterReload = int(value & 127)
}

func (apu *APUState) WriteTriangleTimerLow(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: write triangle timer low %v", value)
    }
    apu.Triangle.Timer.Low = uint16(value)
    apu.Triangle.Timer.Reset()
}

func (apu *APUState) WriteTriangleTimerHigh(value byte){
    if ApuDebug > 0 {
        log.Printf("APU: write triangle timer high %v", value)
    }
    apu.Triangle.Timer.High = uint16(value & 7)
    apu.Triangle.Timer.Reset()
    lengthIndex := value >> 3
    apu.Triangle.Length.SetLength(lengthIndex)
    apu.Triangle.LinearCounterReloadFlag = true
}

/* http://wiki.nesdev.com/w/index.php/APU_Noise */
func noisePeriod(period byte) uint16 {
    /* NTSC */
    switch period & 0xf {
        case 0: return 4
        case 1: return 8
        case 2: return 16
        case 3: return 32
        case 4: return 64
        case 5: return 96
        case 6: return 128
        case 7: return 160
        case 8: return 202
        case 9: return 254
        case 0xa: return 380
        case 0xb: return 508
        case 0xc: return 762
        case 0xd: return 1016
        case 0xe: return 2034
        case 0xf: return 4068
    }

    /* it should not be possible to get here */
    return 4
}

func (apu *APUState) WriteNoiseMode(value byte){
    mode := (value >> 7) & 0x1
    period := value & 0xf
    if ApuDebug > 0 {
        log.Printf("APU: write noise mode value=%v loop=%v period=%v", value, mode, period)
    }

    apu.Noise.Mode = mode
    apu.Noise.Timer.SetPeriod(noisePeriod(period))
}

func (apu *APUState) WriteNoiseEnvelope(value byte){
    loop := (value >> 5) & 0x1 == 0x1
    constant := (value >> 4) & 0x1 == 0x1
    period := value & 0xf
    // log.Printf("APU: write noise envelope value=%v loop=%v enable=%v period=%v", value, loop, enable, period)
    apu.Noise.Envelope.Set(loop, constant, period)
}

func (apu *APUState) WriteNoiseLength(value byte){
    // log.Printf("APU: write noise length value=%v", value)

    length := value >> 3
    apu.Noise.Length.SetLength(length)
}

func (apu *APUState) IsIRQAsserted() bool {
    return apu.FrameIRQAsserted || apu.DMC.IRQAsserted
}

func (apu *APUState) WriteChannelEnable(value byte, cpu *CPUState){
    dmc := (value >> 4) & 0x1
    noise := (value >> 3) & 0x1
    triangle := (value >> 2) & 0x1
    pulse2 := (value >> 1) & 0x1
    pulse1 := (value >> 0) & 0x1

    apu.DMC.IRQAsserted = false

    if dmc == 1 {
        /* If BytesRemaining > 0 then some sample is still playing, so let it finish */
        if apu.DMC.BytesRemaining == 0 {
            apu.DMC.Reset()
        }
    } else {
        apu.DMC.BytesRemaining = 0
    }

    apu.EnableNoise = noise == 0x1
    apu.EnableTriangle = triangle == 0x1
    apu.EnablePulse2 = pulse2 == 0x1
    apu.EnablePulse1 = pulse1 == 0x1

    if !apu.EnablePulse1 {
        apu.Pulse1.Length.Clear()
    }

    if !apu.EnablePulse2 {
        apu.Pulse2.Length.Clear()
    }

    if !apu.EnableTriangle {
        apu.Triangle.Length.Clear()
    }

    if ApuDebug > 0 {
        log.Printf("APU: write channel enable value=%v dmc=%v noise=%v triangle=%v pulse2=%v pulse1=%v", value, dmc, noise, triangle, pulse2, pulse1)
    }
}

func (apu *APUState) WriteFrameCounter(value byte){
    mode := value >> 7
    if ApuDebug > 0 {
        log.Printf("APU: write frame counter value=%v mode=%v", value, mode)
    }
    interrupt := (value >> 6) & 0x1
    apu.InterruptInhibit = interrupt == 1

    /* Interrupt inhibit flag. If set, the frame interrupt flag is cleared, otherwise it is unaffected. */
    if interrupt == 1 {
        apu.FrameIRQAsserted = false
    }

    apu.FrameMode = mode == 0
    /* FIXME: 3 if during an apu cycle, 4 if not. */
    apu.UpdatedFrameCounter = 4
}

func bool_to_byte(x bool) byte {
    if x {
        return 1
    }

    return 0
}

func (apu *APUState) ReadStatus() byte {
    var dmcInterrupt byte = bool_to_byte(apu.DMC.IRQAsserted)
    var frameInterrupt byte = bool_to_byte(apu.FrameIRQAsserted)
    var dmc byte = bool_to_byte(apu.DMC.BytesRemaining > 0)
    var noise byte = bool_to_byte(apu.Noise.Length.Length > 0)
    var triangle byte = bool_to_byte(apu.Triangle.Length.Length > 0)
    var pulse2 byte = bool_to_byte(apu.Pulse2.Length.Length > 0)
    var pulse1 byte = bool_to_byte(apu.Pulse1.Length.Length > 0)

    apu.FrameIRQAsserted = false

    status := (dmcInterrupt << 7) | (frameInterrupt << 6) | (dmc << 4) |
              (noise << 3) | (triangle << 2) | (pulse2 << 1) | (pulse1 << 0)

    if ApuDebug > 0 {
        log.Printf("Read status %08b I=%v F=%v D=%v N=%v T=%v 2=%v 1=%v", status, dmcInterrupt, frameInterrupt, dmc, noise, triangle, pulse2, pulse1)
    }

    return status

}

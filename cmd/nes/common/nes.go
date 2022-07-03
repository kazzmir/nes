package common

import (
    "errors"
    "context"
    "log"
    "time"
    "sync"
    nes "github.com/kazzmir/nes/lib"
)

type AudioResponse int
const (
    AudioResponseEnabled AudioResponse = iota
    AudioResponseDisabled
)

type ScreenListeners struct {
    VideoListeners []chan nes.VirtualScreen
    AudioListeners []chan []float32
    Lock sync.Mutex
}

func (listeners *ScreenListeners) ObserveVideo(screen nes.VirtualScreen){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    if len(listeners.VideoListeners) == 0 {
        return
    }

    buffer := screen.Copy()

    for _, listener := range listeners.VideoListeners {
        select {
            case listener<- buffer:
            default:
        }
    }
}

func (listeners *ScreenListeners) AddVideoListener(listener chan nes.VirtualScreen){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    listeners.VideoListeners = append(listeners.VideoListeners, listener)
}

func (listeners *ScreenListeners) RemoveVideoListener(remove chan nes.VirtualScreen){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    var out []chan nes.VirtualScreen
    for _, listener := range listeners.VideoListeners {
        if listener != remove {
            out = append(out, listener)
        }
    }

    listeners.VideoListeners = out
}

func (listeners *ScreenListeners) ObserveAudio(pcm []float32){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    if len(listeners.AudioListeners) == 0 {
        return
    }

    for _, listener := range listeners.AudioListeners {
        select {
            case listener<- pcm:
            default:
                log.Printf("Cannot observe audio")
        }
    }
}

func (listeners *ScreenListeners) AddAudioListener(listener chan []float32){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    listeners.AudioListeners = append(listeners.AudioListeners, listener)
}

func (listeners *ScreenListeners) RemoveAudioListener(remove chan []float32){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    var out []chan []float32
    for _, listener := range listeners.AudioListeners {
        if listener != remove {
            out = append(out, listener)
        }
    }

    listeners.AudioListeners = out
}


type EmulatorAction int
const (
    EmulatorNothing = iota // just a default value that has no behavior
    EmulatorNormal
    EmulatorTurbo
    EmulatorInfinite
    EmulatorSlowDown
    EmulatorSpeedUp
    EmulatorTogglePause
    EmulatorTogglePPUDebug
    EmulatorStepFrame
    EmulatorSetPause
    EmulatorUnpause
)

func SetupCPU(nesFile nes.NESFile, debug bool) (nes.CPUState, error) {
    cpu := nes.StartupState()

    if nesFile.HorizontalMirror {
        cpu.PPU.SetHorizontalMirror()
    }
    if nesFile.VerticalMirror {
        cpu.PPU.SetVerticalMirror()
    }

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return cpu, err
    }
    cpu.SetMapper(mapper)

    maxCharacterRomLength := len(nesFile.CharacterRom)
    if maxCharacterRomLength > 0x2000 {
        maxCharacterRomLength = 0x2000
    }
    cpu.PPU.CopyCharacterRom(0x0000, nesFile.CharacterRom[:maxCharacterRomLength])

    if debug {
        cpu.Debug = 1
        cpu.PPU.Debug = 1
    }

    cpu.Reset()

    return cpu, nil
}

var MaxCyclesReached error = errors.New("maximum cycles reached")
func RunNES(cpu *nes.CPUState, maxCycles uint64, quit context.Context, toDraw chan<- nes.VirtualScreen,
            bufferReady <-chan nes.VirtualScreen, audio chan<-[]float32,
            emulatorActions <-chan EmulatorAction, screenListeners *ScreenListeners,
            sampleRate float32, verbose int) error {
    instructionTable := nes.MakeInstructionDescriptiontable()

    screen := nes.MakeVirtualScreen(256, 240)

    var cycleCounter float64

    /* run the host timer at this frequency (in ms) so that the counter
     * doesn't tick too fast
     *
     * anything higher than 1 seems ok, with 10 probably being an upper limit
     */
    hostTickSpeed := 5
    cycleDiff := nes.CPUSpeed / (1000.0 / float64(hostTickSpeed))

    /* about 20.292 */
    baseCyclesPerSample := nes.CPUSpeed / 2 / float64(sampleRate)

    cycleTimer := time.NewTicker(time.Duration(hostTickSpeed) * time.Millisecond)
    defer cycleTimer.Stop()

    turboMultiplier := float64(1)

    lastCpuCycle := cpu.Cycle

    paused := false

    stepFrame := false

    /* run without delays when this is true */
    infiniteSpeed := false

    showTimings := false

    start := time.Now()
    cycleCheck := time.NewTicker(time.Second * 2)
    defer cycleCheck.Stop()
    cycleStart := cpu.Cycle

    for quit.Err() == nil {
        if maxCycles > 0 && cpu.Cycle >= maxCycles {
            if verbose > 0 {
                log.Printf("Maximum cycles %v reached", maxCycles)
            }
            return MaxCyclesReached
        }

        if showTimings {
            select {
                case <-cycleCheck.C:
                    diff := time.Now().Sub(start)
                    cycleDiff := cpu.Cycle - cycleStart
                    cyclesPerSecond := float64(cycleDiff) / (float64(diff)/float64(time.Second))
                    xdiff := cyclesPerSecond - nes.CPUSpeed
                    cycleXDiff := float64(cycleDiff) - float64(nes.CPUSpeed) * float64(diff) / float64(time.Second)
                    /* cycle diffs should be as close to 0 as possible */
                    log.Printf("Time=%v Cycles=%v. Expected=%v. Diff=%v Cycles/s=%v. Expected=%v. Diff=%v", diff, cycleDiff, int64(nes.CPUSpeed * float64(diff) / float64(time.Second)), cycleXDiff, cyclesPerSecond, nes.CPUSpeed, xdiff)

                    start = time.Now()
                    cycleStart = cpu.Cycle
                default:
            }
        }

        /* always run the system */
        if infiniteSpeed {
            cycleCounter = 1

            /* ignore anything on the emulatorActions channel, but dont let it fill up */
            select {
                case <- emulatorActions:
                default:
            }
        }

        for cycleCounter <= 0 {
            select {
                case <-quit.Done():
                    return nil
                case action := <-emulatorActions:
                    switch action {
                        case EmulatorNothing:
                            /* nothing */
                        case EmulatorTurbo:
                            turboMultiplier = 3
                        case EmulatorInfinite:
                            infiniteSpeed = true
                        case EmulatorNormal:
                            turboMultiplier = 1
                            if verbose > 0 {
                                log.Printf("Emulator speed set to %v", turboMultiplier)
                            }
                        case EmulatorSlowDown:
                            turboMultiplier -= 0.1
                            if turboMultiplier < 0.1 {
                                turboMultiplier = 0.1
                            }
                            if verbose > 0 {
                                log.Printf("Emulator speed set to %v", turboMultiplier)
                            }
                        case EmulatorStepFrame:
                            stepFrame = !stepFrame
                            if verbose > 0 {
                                log.Printf("Emulator step frame is %v", stepFrame)
                            }
                        case EmulatorSpeedUp:
                            turboMultiplier += 0.1
                            if verbose > 0 {
                                log.Printf("Emulator speed set to %v", turboMultiplier)
                            }
                        case EmulatorTogglePause:
                            paused = !paused
                        case EmulatorSetPause:
                            paused = true
                        case EmulatorUnpause:
                            paused = false
                        case EmulatorTogglePPUDebug:
                            cpu.PPU.ToggleDebug()
                    }
                case <-cycleTimer.C:
                    cycleCounter += cycleDiff * turboMultiplier
            }

            if paused {
                cycleCounter = 0
            }
        }

        // log.Printf("Cycle counter %v\n", cycleCounter)

        err := cpu.Run(instructionTable)
        if err != nil {
            return err
        }
        usedCycles := cpu.Cycle

        cycleCounter -= float64(usedCycles - lastCpuCycle)

        audioData := cpu.APU.Run((float64(usedCycles) - float64(lastCpuCycle)) / 2.0, turboMultiplier * baseCyclesPerSample, cpu)

        if audioData != nil {
            screenListeners.ObserveAudio(audioData)

            // log.Printf("Send audio data via channel")
            select {
                case audio<- audioData:
                default:
                    if verbose > 0 {
                        log.Printf("Warning: audio falling behind")
                    }
            }
        }

        /* ppu runs 3 times faster than cpu */
        nmi, drawn := cpu.PPU.Run((usedCycles - lastCpuCycle) * 3, screen, cpu.Mapper)

        if drawn {
            screenListeners.ObserveVideo(screen)

            select {
                case buffer := <-bufferReady:
                    buffer.CopyFrom(&screen)
                    toDraw <- buffer
                    if stepFrame {
                        paused = true
                    }
                default:
            }
        }

        lastCpuCycle = usedCycles

        if nmi {
            if cpu.Debug > 0 && verbose > 0 {
                log.Printf("Cycle %v Do NMI\n", cpu.Cycle)
            }
            cpu.NMI()
        }
    }

    // log.Printf("CPU cycles %v waited %v nanoseconds out of %v", cpu.Cycle, totalWait, time.Now().Sub(realStart).Nanoseconds())

    return nil
}

func RenderPixelsRGBA(screen nes.VirtualScreen, raw_pixels []byte, overscanPixels int){
    width := int32(screen.Width)
    // height := int32(240 - overscanPixels * 2)

    startPixel := overscanPixels * int(width)
    endPixel := (screen.Height - overscanPixels) * int(width)

    /* FIXME: this can be done with a writer and binary.Writer(BigEndian, pixels) */

    for i, pixel := range screen.Buffer[startPixel:endPixel] {
        /* red */
        raw_pixels[i*4+0] = byte(pixel >> 24)
        /* green */
        raw_pixels[i*4+1] = byte(pixel >> 16)
        /* blue */
        raw_pixels[i*4+2] = byte(pixel >> 8)
        /* alpha */
        raw_pixels[i*4+3] = byte(pixel >> 0)
    }
}

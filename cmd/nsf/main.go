package main

import (
    "log"
    "fmt"
    "io"
    "os"
    "bytes"
    "context"
    "errors"
    "time"
    "encoding/binary"

    nes "github.com/kazzmir/nes/lib"
    "github.com/veandco/go-sdl2/sdl"

    "github.com/jroimartin/gocui"
)

type NSFFile struct {
    LoadAddress uint16
    InitAddress uint16
    PlayAddress uint16
    TotalSongs byte
    StartingSong byte
    NTSCSpeed uint16
    Data []byte
}

func isNSF(header []byte) bool {
    nsfBytes := []byte{'N', 'E', 'S', 'M', 0x1a}
    if len(header) < len(nsfBytes) {
        return false
    }

    return bytes.Equal(header[0:len(nsfBytes)], nsfBytes)
}

func loadNSF(path string) (NSFFile, error) {
    file, err := os.Open(path)
    if err != nil {
        return NSFFile{}, err
    }
    defer file.Close()

    header := make([]byte, 0x80)

    _, err = io.ReadFull(file, header)
    if err != nil {
        return NSFFile{}, fmt.Errorf("Could not read NSF header, is '%v' an NSF file? %v", path, err)
    }

    if !isNSF(header){
        return NSFFile{}, fmt.Errorf("Not an NSF file")
    }

    version := header[0x5]
    totalSongs := header[0x6]
    startingSong := header[0x7]

    loadAddress := (uint16(header[0x9]) << 8) | uint16(header[0x8])
    initAddress := (uint16(header[0xb]) << 8) | uint16(header[0xa])
    playAddress := (uint16(header[0xd]) << 8) | uint16(header[0xc])
    songName := header[0xe:0xe+32]
    artist := header[0x2e:0x2e+32]
    copyright := header[0x4e:0x4e+32]
    ntscSpeed := (uint16(header[0x6f]) << 8) | uint16(header[0x6f])
    bankValues := header[0x70:0x78]
    palSpeed := (uint16(header[0x79]) << 8) | uint16(header[0x78])
    palOrNtsc := header[0x7a]

    extraSoundChip := header[0x7b]
    nsf2Reserved := header[0x7c]
    nsf2MetaData := header[0x7d:0x7d+3]

    _ = nsf2Reserved
    _ = nsf2MetaData

    log.Printf("Version %v", version)
    log.Printf("Total songs %v", totalSongs)
    log.Printf("Starting song %v", startingSong)
    log.Printf("Load address 0x%x", loadAddress)
    log.Printf("Init address 0x%x", initAddress)
    log.Printf("Play address 0x%x", playAddress)
    log.Printf("Song '%v'", string(songName))
    log.Printf("Artist '%v'", string(artist))
    log.Printf("Copyright '%v'", string(copyright))
    log.Printf("NTSC speed %v", ntscSpeed)
    log.Printf("Bank values %v", bankValues)
    log.Printf("PAL speed %v", palSpeed)
    log.Printf("PAL/NTSC %v", palOrNtsc)
    log.Printf("Extra sound chip %v", extraSoundChip)

    _ = version
    _ = totalSongs
    _ = startingSong

    programData := make([]byte, 0x10000 - uint32(loadAddress))
    read, err := io.ReadFull(file, programData)
    if err != nil {
        log.Printf("Could only read 0x%x bytes", read)
    } else {
        log.Printf("Read 0x%x program bytes", read)
    }

    return NSFFile{
        LoadAddress: loadAddress,
        InitAddress: initAddress,
        PlayAddress: playAddress,
        TotalSongs: totalSongs,
        StartingSong: startingSong,
        NTSCSpeed: ntscSpeed,
        Data: programData,
    }, nil
}

type NSFMapper struct {
    Data []byte
    LoadAddress uint16
}

func (mapper *NSFMapper) Write(cpu *nes.CPUState, address uint16, value byte) error {
    return fmt.Errorf("nsf mapper write unimplemented")
}

func (mapper *NSFMapper) Read(address uint16) byte {
    use := int(address) - int(mapper.LoadAddress)
    if use >= len(mapper.Data) {
        return 0
    }
    if use < 0 {
        return 0
    }
    return mapper.Data[use]
}

func MakeNSFMapper(data []byte, loadAddress uint16) nes.Mapper {
    return &NSFMapper{
        Data: data,
        LoadAddress: loadAddress,
    }
}

type NoInput struct {
}

func (buttons *NoInput) Get() nes.ButtonMapping {
    return make(nes.ButtonMapping)
}

var MaxCyclesReached error = errors.New("maximum cycles reached")

func setupAudio(sampleRate float32) (sdl.AudioDeviceID, error) {
    var audioSpec sdl.AudioSpec
    var obtainedSpec sdl.AudioSpec

    audioSpec.Freq = int32(sampleRate)
    audioSpec.Format = sdl.AUDIO_F32LSB
    audioSpec.Channels = 1
    audioSpec.Samples = 1024
    // audioSpec.Callback = sdl.AudioCallback(C.generate_audio_c)
    audioSpec.Callback = nil
    audioSpec.UserData = nil

    device, err := sdl.OpenAudioDevice("", false, &audioSpec, &obtainedSpec, sdl.AUDIO_ALLOW_FORMAT_CHANGE)
    return device, err
}

func playNSF(nsf NSFFile, track byte, audioDevice sdl.AudioDeviceID, sampleRate float32, mainQuit context.Context) error {
    cpu := nes.StartupState()
    cpu.SetMapper(MakeNSFMapper(nsf.Data, nsf.LoadAddress))
    cpu.Input = nes.MakeInput(&NoInput{})

    cpu.A = track
    cpu.X = 0 // ntsc or pal

    /* jsr INIT
     * jsr PLAY
     * jmp $here
     */

    /* FIXME: supposedly NSF files can write to memory 0-0x1ef, but are unlikely
     * to use the interrupt vectors from 0xfffa-0xffff, so this code could be
     * moved to the interrupt vectors
     */
    initJSR := uint16(0)

    cpu.StoreMemory(initJSR, nes.Instruction_JSR)
    cpu.StoreMemory(initJSR + 1, byte(nsf.InitAddress & 0xff))
    cpu.StoreMemory(initJSR + 2, byte(nsf.InitAddress >> 8))

    /* the address of the jsr instruction that jumps to the $play address */
    var playJSR uint16 = initJSR + 3
    cpu.StoreMemory(playJSR, nes.Instruction_JSR)
    cpu.StoreMemory(playJSR + 1, byte(nsf.PlayAddress & 0xff))
    cpu.StoreMemory(playJSR + 2, byte(nsf.PlayAddress >> 8))

    /* jmp in place until the jsr $play instruction is run again */
    jmpSelf := playJSR + 3
    cpu.StoreMemory(jmpSelf, nes.Instruction_JMP_absolute)
    cpu.StoreMemory(jmpSelf + 1, 0x6)
    cpu.StoreMemory(jmpSelf + 2, 0x0)

    // cpu.StoreMemory(0x6, nes.Instruction_KIL_1)
    /* Jump back to the JSR $play instruction */
    /*
    cpu.StoreMemory(0x6, nes.Instruction_JMP_absolute)
    cpu.StoreMemory(0x7, 0x3)
    cpu.StoreMemory(0x8, 0x0)
    */

    /* enable all channels */
    cpu.StoreMemory(nes.APUChannelEnable, 0xf)

    /* set frame mode */
    cpu.StoreMemory(nes.APUFrameCounter, 0x0)

    cpu.PC = 0
    cpu.Debug = 0

    instructionTable := nes.MakeInstructionDescriptiontable()

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

    // nes.ApuDebug = 1

    turboMultiplier := 1.0

    cycleTimer := time.NewTicker(time.Duration(hostTickSpeed) * time.Millisecond)
    defer cycleTimer.Stop()

    playRate := 1000000.0 / float32(nsf.NTSCSpeed)

    playTimer := time.NewTicker(time.Duration(1.0/playRate * 1000 * 1000) * time.Microsecond)
    defer playTimer.Stop()

    lastCpuCycle := cpu.Cycle
    var maxCycles uint64 = 0

    quit, cancel := context.WithCancel(mainQuit)
    paused := false
    _ = cancel

    atPlay := false
    var audioBuffer bytes.Buffer
    for quit.Err() == nil {

        /* the cpu will be executing init for a while, so dont force a jump to $play
         * until the cpu has executed the jsr $play instruction at least once
         */
        if cpu.PC == playJSR {
            // log.Printf("Play routine")
            atPlay = true
        }

        if atPlay {
            select {
                /* every $period hz jump back to the play routine
                 */
                case <-playTimer.C:
                    cpu.PC = playJSR
                default:
            }
        }

        if maxCycles > 0 && cpu.Cycle >= maxCycles {
            log.Printf("Maximum cycles %v reached", maxCycles)
            return MaxCyclesReached
        }

        for cycleCounter <= 0 {
            select {
                case <-quit.Done():
                    return nil
                case <-cycleTimer.C:
                    cycleCounter += cycleDiff * turboMultiplier
            }

            if paused {
                cycleCounter = 0
            }
        }

        err := cpu.Run(instructionTable)
        if err != nil {
            return err
        }
        usedCycles := cpu.Cycle

        cycleCounter -= float64(usedCycles - lastCpuCycle)

        audioData := cpu.APU.Run((float64(usedCycles) - float64(lastCpuCycle)) / 2.0, turboMultiplier * baseCyclesPerSample, &cpu)

        if audioData != nil {
            audioBuffer.Reset()
            /* convert []float32 into []byte */
            for _, sample := range audioData {
                binary.Write(&audioBuffer, binary.LittleEndian, sample)
            }
            // log.Printf("Enqueue audio")
            err := sdl.QueueAudio(audioDevice, audioBuffer.Bytes())
            if err != nil {
                return fmt.Errorf("Error: could not queue audio data: %v", err)
            }
        }

        lastCpuCycle = usedCycles
    }

    return nil
}

type PlayerAction int
const (
    PlayerNextTrack = iota
    PlayerPreviousTrack
    PlayerQuit
)

func run(path string) error {
    nsf, err := loadNSF(path)
    if err != nil {
        return err
    }

    _ = nsf

    err = sdl.Init(sdl.INIT_AUDIO)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    sampleRate := float32(44100)

    audioDevice, err := setupAudio(sampleRate)
    if err != nil {
        log.Printf("Warning: could not set up audio: %v", err)
        audioDevice = 0
    } else {
        defer sdl.CloseAudioDevice(audioDevice)
        log.Printf("Opened SDL audio device %v", audioDevice)
        sdl.PauseAudioDevice(audioDevice, false)
    }

    quit, cancel := context.WithCancel(context.Background())

    gui, err := gocui.NewGui(gocui.OutputNormal)
    if err != nil {
        return err
    }

    defer gui.Close()

    gui.InputEsc = true

    var mainView *gocui.View

    gui.Update(func (gui *gocui.Gui) error {
        var err error
        mainView, err = gui.SetView("main", 0, 0, 30, 30)
        if err != nil && err != gocui.ErrUnknownView {
            return err
        }
        fmt.Fprintf(mainView, "this is a view %v", os.Getpid())

        return nil
    })

    playerActions := make(chan PlayerAction)

    err = gui.SetKeybinding("", gocui.KeyEsc, gocui.ModNone, func(gui *gocui.Gui, view *gocui.View) error {
        return gocui.ErrQuit
    })

    if err != nil {
        log.Printf("Failed to bind esc in the gui: %v", err)
        return err
    }

    err = gui.SetKeybinding("", gocui.KeyArrowLeft, gocui.ModNone, func(gui *gocui.Gui, view *gocui.View) error {
        playerActions <- PlayerPreviousTrack
        return nil
    })

    if err != nil {
        return err
    }

    err = gui.SetKeybinding("", gocui.KeyArrowRight, gocui.ModNone, func(gui *gocui.Gui, view *gocui.View) error {
        playerActions <- PlayerNextTrack
        return nil
    })

    if err != nil {
        return err
    }

    go func(){
        err := gui.MainLoop()
        if err != nil && err != gocui.ErrQuit {
            log.Printf("Error from gocui: %v", err)
        }

        cancel()
    }()

    track := byte(0)

    runPlayer := func(track byte) (context.Context, context.CancelFunc) {
        playQuit, playCancel := context.WithCancel(quit)
        go func(){
            err := playNSF(nsf, track, audioDevice, sampleRate, playQuit)
            if err != nil {
                log.Printf("Unable to play: %v", err)
            }
        }()

        return playQuit, playCancel
    }

    playQuit, playCancel := runPlayer(track)
    for quit.Err() == nil {
        select {
            case action := <-playerActions:
                switch action {
                    case PlayerPreviousTrack:
                        if track > 0 {
                            track -= 1
                            playCancel()
                            playQuit, playCancel = runPlayer(track)
                        }
                    case PlayerNextTrack:
                        if track < nsf.TotalSongs - 1 {
                            track += 1
                            playCancel()
                            playQuit, playCancel = runPlayer(track)
                        }
                }
            case <-quit.Done():
        }
    }

    <-playQuit.Done()

    return nil
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

    var nesPath string

    if len(os.Args) == 1 {
        fmt.Printf("Give a .nsf file to play\n")
        return
    }

    nesPath = os.Args[1]
    err := run(nesPath)
    if err != nil {
        log.Printf("Error: %v", err)
    }
}

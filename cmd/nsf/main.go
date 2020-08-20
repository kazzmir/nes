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
)

type NSFFile struct {
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

    programData := make([]byte, uint32(0x10000) - uint32(loadAddress))
    read, err := io.ReadFull(file, programData)
    if err != nil {
        log.Printf("Could only read 0x%x bytes", read)
    } else {
        log.Printf("Read 0x%x program bytes", read)
    }

    return NSFFile{
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
}

func (mapper *NSFMapper) Write(cpu *nes.CPUState, address uint16, value byte) error {
    return fmt.Errorf("nsf mapper write unimplemented")
}

func (mapper *NSFMapper) Read(address uint16) byte {
    return mapper.Data[address - 0x8000]
}

func MakeNSFMapper(data []byte) nes.Mapper {
    return &NSFMapper{
        Data: data,
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

func run(path string) error {
    nsf, err := loadNSF(path)
    if err != nil {
        return err
    }

    _ = nsf

    cpu := nes.StartupState()
    cpu.SetMapper(MakeNSFMapper(nsf.Data))
    cpu.Input = nes.MakeInput(&NoInput{})

    cpu.A = nsf.StartingSong - 1
    cpu.X = 0

    /* jsr INIT
     * jsr PLAY
     * jmp $here
     */

    cpu.StoreMemory(0x0, nes.Instruction_JSR)
    cpu.StoreMemory(0x1, byte(nsf.InitAddress & 0xff))
    cpu.StoreMemory(0x2, byte(nsf.InitAddress >> 8))

    /* the address of the jsr instruction that jumps to the $play address */
    var playJSR uint16 = 0x3
    cpu.StoreMemory(0x3, nes.Instruction_JSR)
    cpu.StoreMemory(0x4, byte(nsf.PlayAddress & 0xff))
    cpu.StoreMemory(0x5, byte(nsf.PlayAddress >> 8))

    /* jmp in place until the jsr $play instruction is run again */
    cpu.StoreMemory(0x6, nes.Instruction_JMP_absolute)
    cpu.StoreMemory(0x7, 0x6)
    cpu.StoreMemory(0x8, 0x0)

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

    sampleRate := float32(44100)
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

    playRate := 1000000.0 / float32(nsf.NTSCSpeed)

    playTimer := time.NewTicker(time.Duration(1.0/playRate * 1000 * 1000) * time.Microsecond)

    lastCpuCycle := cpu.Cycle
    var maxCycles uint64 = 0

    quit, cancel := context.WithCancel(context.Background())
    paused := false

    _ = cancel

    err = sdl.Init(sdl.INIT_AUDIO)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    audioDevice, err := setupAudio(sampleRate)
    if err != nil {
        log.Printf("Warning: could not set up audio: %v", err)
        audioDevice = 0
    } else {
        defer sdl.CloseAudioDevice(audioDevice)
        log.Printf("Opened SDL audio device %v", audioDevice)
        sdl.PauseAudioDevice(audioDevice, false)
    }

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

package lib

import (
    "bytes"
    "fmt"
    "io"
    "os"
    "log"
    "math"
    "context"
    "time"
    "errors"
)

type NSFFile struct {
    LoadAddress uint16
    InitAddress uint16
    PlayAddress uint16
    TotalSongs byte
    StartingSong byte
    NTSCSpeed uint16
    SongName string
    Artist string
    Copyright string
    Data []byte
    InitialBanks []byte
    ExtraSoundChip byte
}

func isNSF(header []byte) bool {
    nsfBytes := []byte{'N', 'E', 'S', 'M', 0x1a}
    if len(header) < len(nsfBytes) {
        return false
    }

    return bytes.Equal(header[0:len(nsfBytes)], nsfBytes)
}

func IsNSFFile(path string) bool {
    file, err := os.Open(path)
    if err != nil {
        return false
    }
    defer file.Close()

    header := make([]byte, 0x80)

    _, err = io.ReadFull(file, header)
    if err != nil {
        return false
    }

    return isNSF(header)
}

func LoadNSF(path string) (NSFFile, error) {
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

    _ = palSpeed
    _ = palOrNtsc

    /*
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
    */

    _ = version
    _ = totalSongs
    _ = startingSong

    programData, err := io.ReadAll(file)

    if err != nil {
        log.Printf("Unable to read NSF data: %v", err)
    } else {
        log.Printf("Read 0x%x bytes of music data", len(programData))
    }

    return NSFFile{
        LoadAddress: loadAddress,
        InitAddress: initAddress,
        PlayAddress: playAddress,
        TotalSongs: totalSongs,
        StartingSong: startingSong,
        NTSCSpeed: ntscSpeed,
        Data: programData,
        InitialBanks: bankValues,
        ExtraSoundChip: extraSoundChip,

        SongName: string(songName),
        Artist: string(artist),
        Copyright: string(copyright),
    }, nil
}

func (nsf *NSFFile) UseBankSwitch() bool {
    for _, value := range nsf.InitialBanks {
        if value > 0 {
            return true
        }
    }

    return false
}

type NSFMapper struct {
    Data []byte
    // bank switching. The value in bank[0] relates to the addresses read from the first bank, bank[1] second bank, etc
    Banks []byte
    UseBankSwitch bool
    LoadAddress uint16

    VRC6 *VRC6Audio
}

func (mapper *NSFMapper) IsNSF() bool {
    return true
}

func (mapper *NSFMapper) Write(cpu *CPUState, address uint16, value byte) error {
    // 5ff6 and 5ff7 are also bank switched

    // bank switching addresses
    if address >= 0x5ff8 && address <= 0x5fff {
        // normalize to 0
        bank := address - 0x5ff8

        // log.Printf("Set bank %v to %v", bank, value)

        if int(bank) < len(mapper.Banks) {
            // should probably binary-& the value with the maximum bank number
            mapper.Banks[bank] = value
            mapper.UseBankSwitch = true
        }
        return nil
    }

    if mapper.VRC6 != nil && mapper.VRC6.HandleWrite(address, value) {
        return nil
    }

    return fmt.Errorf("nsf mapper write unimplemented for 0x%x=0x%x", address, value)
}

func (mapper *NSFMapper) Read(address uint16) byte {
    use := int(address) - int(mapper.LoadAddress)
    if use >= len(mapper.Data) {
        return 0
    }
    if use < 0 {
        return 0
    }

    if mapper.UseBankSwitch {
        bankIndex := use / 0x1000
        offset := use % 0x1000
        bankAddress := mapper.Banks[bankIndex]
        convertedAddress := int(bankAddress) * 0x1000 + int(offset)
        // log.Printf("Read address 0x%x -> 0x%x", address, int(bankAddress) * 0x1000 + int(offset))

        return mapper.Data[convertedAddress]
    } else {
        return mapper.Data[use]
    }
}

func (mapper *NSFMapper) IsIRQAsserted() bool {
    return false
}

func (mapper *NSFMapper) Compare(other Mapper) error {
    return fmt.Errorf("nsf mapper compare is unimplemented")
}

func (mapper *NSFMapper) Copy() Mapper {
    /* FIXME: implement if useful */
    return nil
}

func (mapper *NSFMapper) Kind() int {
    return -1
}

func MakeNSFMapper(data []byte, loadAddress uint16, banks []byte, extraSoundChip byte) *NSFMapper {
    var vrc6 *VRC6Audio

    if extraSoundChip & 0x1 != 0 {
        vrc6 = MakeVRC6Audio()
    }

    return &NSFMapper{
        Data: data,
        LoadAddress: loadAddress,
        Banks: banks,
        VRC6: vrc6,
    }
}

type NoInput struct {
}

func (buttons *NoInput) Get() ButtonMapping {
    return make(ButtonMapping)
}

type NSFActions int
const (
    NSFActionTogglePause = iota
)

var MaxCyclesReached error = errors.New("maximum cycles reached")

/* 1. set up bank switching registers (if necessary)
 * 2. invoke INIT routine
 * 3. repeatedly invoke PLAY routine, followed by a nop loop until the play timer fires
 */
func PlayNSF(nsf NSFFile, track byte, audioOut chan []float32, sampleRate float32, actions chan NSFActions, mainQuit context.Context) error {
    cpu := StartupState()
    nsfMapper := MakeNSFMapper(nsf.Data, nsf.LoadAddress, make([]byte, int(math.Ceil(float64(len(nsf.Data)) / 0x1000))), nsf.ExtraSoundChip)
    cpu.SetMapper(nsfMapper)
    cpu.Input = MakeInput(&NoInput{})

    // cpu.A = track
    cpu.X = 0 // ntsc or pal
    cpu.Y = 0 // just init to something

    if nsf.UseBankSwitch() {
        // set up bank switching values
        for bank := range (0x6000 - 0x5ff8) {
            address := 0x5ff8 + bank
            cpu.StoreMemory(uint16(address), nsf.InitialBanks[bank])
        }
    }

    cpu.A = track

    /* enable all channels */
    cpu.StoreMemory(APUChannelEnable, 0xf)

    /* set frame mode */
    cpu.StoreMemory(APUFrameCounter, 0x0)

    cpu.PC = 0
    cpu.Debug = 0

    instructionTable := MakeInstructionDescriptiontable()

    var cycleCounter float64

    /* run the host timer at this frequency (in ms) so that the counter
     * doesn't tick too fast
     *
     * anything higher than 1 seems ok, with 10 probably being an upper limit
     */
    hostTickSpeed := 5
    cycleDiff := CPUSpeed / (1000.0 / float64(hostTickSpeed))

    /* about 20.292 */
    baseCyclesPerSample := CPUSpeed / 2 / float64(sampleRate)

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

    doAudio := func (cpuCycles float64) {
        audioData := cpu.APU.Run(cpuCycles / 2.0, turboMultiplier * baseCyclesPerSample, &cpu)
        // audioData = nil
        if nsfMapper.VRC6 != nil {
            vrc6Audio := nsfMapper.VRC6.Run(cpuCycles, baseCyclesPerSample * 2)
            if vrc6Audio != nil {

                if audioData == nil {
                    audioData = vrc6Audio
                } else {
                    for i := 0; i < len(audioData); i++ {
                        audioData[i] += vrc6Audio[i]
                    }
                }

                audioData = vrc6Audio

                // log.Printf("VRC6 audio: %v", vrc6Audio)
                /*
                select {
                    case audioOut <- vrc6Audio:
                    default:
                }
                */
            }
        }

        if audioData != nil {
            // log.Printf("Audio data: %v", audioData)
            select {
                case audioOut <- audioData:
                default:
            }
        }
    }

    runFunction := func (address uint16) error {
        // rts from function will jump back to 0xffff, so quit then
        cpu.PushStack(0xff)
        cpu.PushStack(0xfe)

        cpu.PC = address

        for quit.Err() == nil && cpu.PC != 0xffff {

            if maxCycles > 0 && cpu.Cycle >= maxCycles {
                log.Printf("Maximum cycles %v reached", maxCycles)
                return MaxCyclesReached
            }

            for cycleCounter <= 0 {
                select {
                    case <-quit.Done():
                        return nil
                    case action := <-actions:
                        switch action {
                            case NSFActionTogglePause:
                                paused = !paused
                        }
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

            doAudio(float64(usedCycles - lastCpuCycle))

            lastCpuCycle = usedCycles
        }

        return nil
    }

    doNopCycle := func() error {
        cycleCounter += cycleDiff * turboMultiplier

        if paused {
            cycleCounter = 0
            return nil
        }

        for quit.Err() == nil && cycleCounter > 0 {
            if maxCycles > 0 && cpu.Cycle >= maxCycles {
                log.Printf("Maximum cycles %v reached", maxCycles)
                return MaxCyclesReached
            }

            select {
                // as soon as the play timer fires, run the play function
                case <-playTimer.C:
                    err := runFunction(nsf.PlayAddress)
                    if err != nil {
                        return err
                    }
                default:
            }

            // don't do anything for 2 cycles
            cpu.Cycle += 2
            usedCycles := cpu.Cycle

            cycleCounter -= float64(usedCycles - lastCpuCycle)

            doAudio(float64(usedCycles - lastCpuCycle))

            lastCpuCycle = usedCycles
        }

        return nil
    }

    err := runFunction(nsf.InitAddress)
    if err != nil {
        return err
    }

    for {
        select {
            case <-quit.Done():
                return nil
                /*
            case <-playTimer.C:
                err := runFunction(nsf.PlayAddress)
                if err != nil {
                    return err
                }
                */
            case action := <-actions:
                switch action {
                    case NSFActionTogglePause:
                        paused = !paused
                }
            case <-cycleTimer.C:
                // cycleCounter += cycleDiff * turboMultiplier
                // cycleCounter += 1000
                err := doNopCycle()
                if err != nil {
                    return err
                }
        }
    }
}

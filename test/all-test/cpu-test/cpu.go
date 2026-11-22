package cpu

import (
    "log"
    "fmt"

    nes "github.com/kazzmir/nes/lib"
    commonNes "github.com/kazzmir/nes/cmd/nes/common"

    test_utils "github.com/kazzmir/nes/test/all-test/utils"
)

type DummyInput struct {}
func (d *DummyInput) Get() nes.ButtonMapping {
    mapping := make(nes.ButtonMapping)

    mapping[nes.ButtonIndexA] = false
    mapping[nes.ButtonIndexB] = false
    mapping[nes.ButtonIndexSelect] = false
    mapping[nes.ButtonIndexStart] = false
    mapping[nes.ButtonIndexUp] = false
    mapping[nes.ButtonIndexDown] = false
    mapping[nes.ButtonIndexLeft] = false
    mapping[nes.ButtonIndexRight] = false

    return mapping
}

func doDummyReadTest() (bool, error) {
    rom := "test-roms/cpu_dummy_reads.nes"

    nesFile, err := nes.ParseNesFile(rom, false)
    if err != nil {
        return false, err
    }

    cpu := nes.StartupState()
    cpu.Input = nes.MakeInput(&DummyInput{})

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return false, err
    }
    cpu.SetMapper(mapper)

    cpu.Reset()

    err = commonNes.SimpleRun(&cpu, uint64(nes.CPUSpeed * 2))

    if err != commonNes.MaxCyclesReached {
        return false, err
    }

    result := cpu.A

    return result == 0, nil
}

func doDummyWritePPUTest() (bool, error) {
    rom := "test-roms/cpu_dummy_writes_ppumem.nes"

    nesFile, err := nes.ParseNesFile(rom, false)
    if err != nil {
        return false, err
    }

    cpu := nes.StartupState()
    cpu.Input = nes.MakeInput(&DummyInput{})

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return false, err
    }
    cpu.SetMapper(mapper)

    cpu.Reset()

    err = commonNes.SimpleRun(&cpu, uint64(nes.CPUSpeed * 4))

    if err != commonNes.MaxCyclesReached {
        return false, err
    }

    testStatus := cpu.LoadMemory(0x6000)

    v1 := cpu.LoadMemory(0x6001)
    v2 := cpu.LoadMemory(0x6002)
    v3 := cpu.LoadMemory(0x6003)

    if v1 == 0xde && v2 == 0xb0 && v3 == 0x61 {
        status := cpu.LoadMemory(0x6000)
        if status == 0 {
            return true, nil
        }
        return false, fmt.Errorf("Error code %02x", status)
    }

    return false, fmt.Errorf("invalid test status %02x: %02x %02x %02x", testStatus, v1, v2, v3)
}

func doDummyOAMWriteTest() (bool, error) {
    rom := "test-roms/cpu_dummy_writes_oam.nes"

    nesFile, err := nes.ParseNesFile(rom, false)
    if err != nil {
        return false, err
    }

    cpu := nes.StartupState()
    cpu.Input = nes.MakeInput(&DummyInput{})

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return false, err
    }
    cpu.SetMapper(mapper)

    cpu.Reset()

    err = commonNes.SimpleRun(&cpu, uint64(nes.CPUSpeed * 6))

    if err != commonNes.MaxCyclesReached {
        return false, err
    }

    result := cpu.A

    return result == 0, nil
}

const klaus6502ROM = "test-roms/klaus-6502/6502_functional_test.nes"

func doKlaus6502Test(path string) error {
    rom, err := nes.ParseNesFile(path, false)
    if err != nil {
        return err
    }
    mapper, err := rom.MakeMapper()
    if err != nil {
        return err
    }
    cpu := nes.StartupState()
    cpu.SetMapper(mapper)
    cpu.Reset()

    // The ROM tests CPU semantics, including BRK/RTI, without a running PPU
    // or asynchronous APU interrupts.
    const pass = 0xfff0
    const cycleLimit = 100_000_000
    table := nes.MakeInstructionDescriptiontable()
    for cpu.Cycle < cycleLimit {
        pc := cpu.PC
        if pc == pass {
            return nil
        }
        if err := cpu.Run(table); err != nil {
            return fmt.Errorf("PC=$%04X test case=$%02X: %w", pc, cpu.Ram[0x200], err)
        }
        if cpu.PC == pc {
            return fmt.Errorf("failure loop at $%04X, test case=$%02X (A=%02X X=%02X Y=%02X P=%02X)", pc, cpu.Ram[0x200], cpu.A, cpu.X, cpu.Y, cpu.Status)
        }
    }
    return fmt.Errorf("cycle limit exceeded at $%04X, test case=$%02X", cpu.PC, cpu.Ram[0x200])
}

func Run(debug bool) (bool, error) {
    klausErr := doKlaus6502Test(klaus6502ROM)
    if klausErr != nil {
        log.Printf("%s: %v", test_utils.Failure("Klaus 6502 functional test"), klausErr)
    } else {
        log.Print(test_utils.Success("Klaus 6502 functional test"))
    }

    dummyReadTest, err := doDummyReadTest()
    if err != nil {
        return false, err
    }

    if dummyReadTest {
        log.Print(test_utils.Success("Dummy reads"))
    } else {
        log.Print(test_utils.Failure("Dummy reads"))
    }

    dummyPPUWriteTest, err := doDummyWritePPUTest()
    if err != nil {
        return false, err
    }

    if dummyPPUWriteTest {
        log.Print(test_utils.Success("Dummy writes"))
    } else {
        log.Print(test_utils.Failure("Dummy writes"))
    }

    dummyOAMWriteTest, err := doDummyOAMWriteTest()
    if err != nil {
        return false, err
    }

    if dummyOAMWriteTest {
        log.Print(test_utils.Success("Dummy OAM writes"))
    } else {
        log.Print(test_utils.Failure("Dummy OAM writes"))
    }

    return klausErr == nil && dummyReadTest && dummyPPUWriteTest && dummyOAMWriteTest, nil
}

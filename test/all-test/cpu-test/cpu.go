package cpu

import (
    "log"

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

func Run(debug bool) (bool, error) {
    dummyReadTest, err := doDummyReadTest()
    if err != nil {
        return false, err
    }

    if dummyReadTest {
        log.Print(test_utils.Success("Dummy reads"))
    } else {
        log.Print(test_utils.Failure("Dummy reads"))
    }

    return dummyReadTest, nil
}

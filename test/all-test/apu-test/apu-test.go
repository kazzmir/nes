package aputest

/* Test files are linked from
 *   http://wiki.nesdev.org/w/index.php/Emulator_tests
 *
 * Direct link to file
 *   https://forums.nesdev.org/download/file.php?id=1494
 *
 * Unzip into 'test-roms/apu'
 */

import (
    nes "github.com/kazzmir/nes/lib"
    test_utils "github.com/kazzmir/nes/test/all-test/utils"
    "log"
    "fmt"
)

func doTest(rom string, passAddress uint16, failAddress uint16) (bool, error) {
    nesFile, err := nes.ParseNesFile(rom, false)
    if err != nil {
        return false, err
    }

    cpu := nes.StartupState()

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return false, err
    }
    cpu.SetMapper(mapper)

    cpu.Reset()

    screen := nes.MakeVirtualScreen(256, 240)
    instructionTable := nes.MakeInstructionDescriptiontable()
    baseCyclesPerSample := 100.0

    for cpu.PC != passAddress && cpu.PC != failAddress {
        cycles := cpu.Cycle
        err := cpu.Run(instructionTable)
        if err != nil {
            return false, err
        }
        usedCycles := cpu.Cycle

        audioData := cpu.APU.Run((float64(usedCycles) - float64(cycles)) / 2.0, baseCyclesPerSample, &cpu)
        _ = audioData

        nmi, _ := cpu.PPU.Run((usedCycles - cycles) * 3, screen, mapper)

        if nmi {
            if cpu.Debug > 0 {
                log.Printf("Cycle %v Do NMI\n", cpu.Cycle)
            }
            cpu.NMI()
        }
    }

    switch cpu.PC {
        case passAddress:
            return true, nil
        case failAddress:
            return false, nil
    }

    return false, fmt.Errorf("Unexpected address 0x%x", cpu.PC)
}

type APUTest struct {
    Rom string
    PassAddress uint16
    FailAddress uint16
}

func Run(debug bool) (bool, error) {
    tests := []APUTest{
        APUTest{
            Rom: "test-roms/apu/test_1.nes",
            PassAddress: 0x815a,
            FailAddress: 0x8165,
        },
        APUTest{
            Rom: "test-roms/apu/test_2.nes",
            PassAddress: 0x815a,
            FailAddress: 0x8165,
        },
        APUTest{
            Rom: "test-roms/apu/test_3.nes",
            PassAddress: 0x815b,
            FailAddress: 0x8166,
        },
        APUTest{
            Rom: "test-roms/apu/test_4.nes",
            PassAddress: 0x815b,
            FailAddress: 0x8166,
        },
        APUTest{
            Rom: "test-roms/apu/test_5.nes",
            PassAddress: 0x815c,
            FailAddress: 0x8167,
        },
        APUTest{
            Rom: "test-roms/apu/test_6.nes",
            PassAddress: 0x815c,
            FailAddress: 0x8167,
        },
        APUTest{
            Rom: "test-roms/apu/test_7.nes",
            PassAddress: 0x815d,
            FailAddress: 0x8168,
        },
        APUTest{
            Rom: "test-roms/apu/test_8.nes",
            PassAddress: 0x815d,
            FailAddress: 0x8168,
        },
        APUTest{
            Rom: "test-roms/apu/test_9.nes",
            PassAddress: 0x815d,
            FailAddress: 0x8168,
        },
        APUTest{
            Rom: "test-roms/apu/test_10.nes",
            PassAddress: 0x815d,
            FailAddress: 0x8168,
        },
    }

    nes.ApuDebug = 0

    for i, test := range tests {
        testNum := i + 1
        passed, err := doTest(test.Rom, test.PassAddress, test.FailAddress)

        if err != nil {
            return false, err
        }

        if passed {
            log.Print(test_utils.Success(fmt.Sprintf("APU test %v", testNum)))
        } else {
            log.Print(test_utils.Failure(fmt.Sprintf("APU test %v", testNum)))
        }
    }

    return true, nil
}

package branch

import (
    nes "github.com/kazzmir/nes/lib"
    "log"
    test_utils "github.com/kazzmir/nes/test/all-test/utils"
)

/* Run blargg's branch timing tests. Unzip them into 'test-roms' such that 'test-roms/branch_timing_tests' exists.
 * This test will run
 *   1.Branch_Basics.nes
 *   2.Backward_Branch.nes
 *   3.Forward_Branch.nes
 * And expects a passing value (1) to be written to address 0xf8
 */

const ResultAddress = 0xf8

/* For each test, run the rom for 150k cycles and check whats written to 0xf8 */
func doTest(rom string) (bool, error) {
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

    var lastCycle uint64 = 0
    for totalCycles := uint32(0); totalCycles < 150000; totalCycles++ {
        err := cpu.Run(instructionTable)
        if err != nil {
            return false, err
        }
        usedCycles := cpu.Cycle

        cycleDiff := usedCycles - lastCycle

        audioData := cpu.APU.Run(float64(cycleDiff) / 2.0, baseCyclesPerSample, &cpu)
        _ = audioData

        nmi, _ := cpu.PPU.Run(cycleDiff * 3, screen, mapper)

        if nmi {
            if cpu.Debug > 0 {
                log.Printf("Cycle %v Do NMI\n", cpu.Cycle)
            }
            cpu.NMI()
        }

        lastCycle = usedCycles
    }

    result := cpu.LoadMemory(ResultAddress)

    return result == 1, nil
}

func Run(debug bool) (bool, error) {
    test1, err := doTest("test-roms/branch_timing_tests/1.Branch_Basics.nes")
    if err != nil {
        return false, err
    }

    if test1 {
        log.Print(test_utils.Success("Branch test 1"))
    } else {
        log.Print(test_utils.Failure("Branch test 1"))
    }

    test2, err := doTest("test-roms/branch_timing_tests/2.Backward_Branch.nes")
    if err != nil {
        return false, err
    }

    if test2 {
        log.Print(test_utils.Success("Branch test 2"))
    } else {
        log.Print(test_utils.Failure("Branch test 2"))
    }

    test3, err := doTest("test-roms/branch_timing_tests/3.Forward_Branch.nes")
    if err != nil {
        return false, err
    }

    if test3 {
        log.Print(test_utils.Success("Branch test 3"))
    } else {
        log.Print(test_utils.Failure("Branch test 3"))
    }

    return test1 && test2 && test3, nil
}

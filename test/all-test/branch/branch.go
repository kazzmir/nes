package branch

import (
    "log"

    nes "github.com/kazzmir/nes/lib"
    commonNes "github.com/kazzmir/nes/cmd/nes/common"

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

type DummyOverlay struct {}
func (d *DummyOverlay) Add(s string) {}

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

    err = commonNes.SimpleRun(&cpu, uint64(nes.CPUSpeed * 1))

    if err != commonNes.MaxCyclesReached {
        return false, err
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

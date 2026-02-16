package main

import (
    "log"
    "flag"

    "github.com/kazzmir/nes/test/all-test/nestest"
    aputest "github.com/kazzmir/nes/test/all-test/apu-test"
    branch "github.com/kazzmir/nes/test/all-test/branch"
    screenshot "github.com/kazzmir/nes/test/all-test/screenshot"
    test_utils "github.com/kazzmir/nes/test/all-test/utils"
)

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds)

    doNesTest := flag.Bool("nes", false, "Run nestest")
    doApuTest := flag.Bool("apu", false, "Run aputest")
    doBranchTest := flag.Bool("branch", false, "Run branch test")
    doScreenshotTest := flag.Bool("screenshot", false, "Run screenshot test")
    all := flag.Bool("all", false, "Run all tests")

    flag.Parse()

    testsToRun := 0
    if *doNesTest {
        testsToRun += 1
    }
    if *doApuTest {
        testsToRun += 1
    }
    if *doBranchTest {
        testsToRun += 1
    }
    if *doScreenshotTest {
        testsToRun += 1
    }

    if *all || testsToRun == 0 {
        *doNesTest = true
        *doApuTest = true
        *doBranchTest = true
        *doScreenshotTest = true
    }

    if *doNesTest {
        ok, err := nestest.Run(false)
        if err != nil {
            log.Printf("Error: nestest failed with an error: %v", err)
        } else {
            if ok {
                log.Print(test_utils.Success("nestest"))
            } else {
                log.Print(test_utils.Failure("nestest"))
            }
        }
    }

    if *doApuTest {
        ok, err := aputest.Run(false)
        if err != nil {
            log.Printf("aputest failed with an error: %v", err)
        }
        _ = ok
    }

    if *doBranchTest {
        ok, err := branch.Run(false)
        if err != nil {
            log.Printf("branch failed with an error: %v", err)
        }
        if !ok {
            log.Printf("branch tests failed")
        }
    }

    if *doScreenshotTest {
        ok, err := screenshot.Run(false)
        if err != nil {
            log.Printf("screenshot failed with an error: %v", err)
        }
        if !ok {
            log.Printf("screenshot tests failed")
        }
    }
}

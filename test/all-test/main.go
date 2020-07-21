package main

import (
    "github.com/kazzmir/nes/test/all-test/nestest"
    aputest "github.com/kazzmir/nes/test/all-test/apu-test"
    branch "github.com/kazzmir/nes/test/all-test/branch"
    "log"
)

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds)

    ok, err := nestest.Run(false)
    if err != nil {
        log.Printf("Error: nestest failed with an error: %v", err)
    } else {
        if ok {
            log.Printf("nestest passed")
        } else {
            log.Printf("nestest failed")
        }
    }

    ok, err = aputest.Run(false)
    if err != nil {
        log.Printf("aputest failed with an error: %v", err)
    }
    _ = ok

    ok, err = branch.Run(false)
    if err != nil {
        log.Printf("branch failed with an error: %v", err)
    }
    if !ok {
        log.Printf("branch tests failed")
    }
}

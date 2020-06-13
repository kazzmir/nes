package main

import (
    nes "github.com/kazzmir/nes/lib"
    "log"
)

type Expected struct {
    Instruction nes.Instruction
    CPU nes.CPUState
}

func parseLog(path string) ([]Expected, error) {
    return nil, nil
}

func main(){
    rom := "test-roms/nestest.nes"
    logFile := "test-roms/nestest.log"

    nesFile, err := nes.ParseNesFile(rom)
    if err != nil {
        log.Fatalf("Unable to parse %v: %v\n", rom, err)
    }

    cpu := nes.StartupState()
    cpu.MapCode(0xc000, nesFile.ProgramRom)
    cpu.Status = 0x24

    memory := nes.NewMemory(0x3000)
    stack := nes.NewMemory(0x100)

    cpu.MapStack(&stack)

    golden, err := parseLog(logFile)
    if err != nil {
        log.Fatalf("Error: %v\n", err)
    }

    _ = golden

    for i := 0; i < 1000; i++ {
        err := cpu.Run(&memory)
        if err != nil {
            log.Fatalf("Error: %v\n", err)
            return
        }
    }
}

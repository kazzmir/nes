package main

import (
    "fmt"
    "log"
    "os"

    nes "github.com/kazzmir/nes/lib"
)

func Run(path string) error {
    nesFile, err := nes.ParseNesFile(path)
    if err != nil {
        return err
    }

    // dump_instructions(prgRom)

    cpu := nes.StartupState()
    /* map code to 0xc000 for NROM-128.
     * also map to 0x8000, but most games don't seem to care..?
     * http://wiki.nesdev.com/w/index.php/Programming_NROM
     */
    cpu.MapMemory(0xc000, nesFile.ProgramRom)
    /* for some reason the nestest code starts with status=0x24
     * http://www.qmtpro.com/~nes/misc/nestest.log
     */
    // cpu.Status = 0x34

    for i := 0; i < 1000; i++ {
        err = cpu.Run()
        if err != nil {
            return err
        }
    }

    return nil
}

func main(){
    if len(os.Args) > 1 {
        err := Run(os.Args[1])
        if err != nil {
            log.Printf("Error: %v\n", err)
        }
    } else {
        fmt.Printf("Give a .nes argument\n")
    }
}

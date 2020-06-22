package main

import (
    "fmt"
    "log"
    "strconv"
    "os"

    nes "github.com/kazzmir/nes/lib"

    "github.com/veandco/go-sdl2/sdl"
)

func Run(path string, debug bool, maxCycles uint64) error {
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
    err = cpu.MapMemory(0x8000, nesFile.ProgramRom)
    if err != nil {
        return err
    }

    /* for a 32k rom, dont map the programrom at 0xc000 */
    if len(nesFile.ProgramRom) == 16*1024 {
        err = cpu.MapMemory(0xc000, nesFile.ProgramRom)
        if err != nil {
            return err
        }
    }

    instructionTable := nes.MakeInstructionDescriptiontable()

    if debug {
        cpu.Debug = 1
    }

    cpu.Reset()

    err = sdl.Init(sdl.INIT_EVERYTHING)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    window, err := sdl.CreateWindow("nes", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, 640, 480, sdl.WINDOW_SHOWN)
    if err != nil {
        return err
    }
    defer window.Destroy()

    surface, err := window.GetSurface()
    if err != nil {
        return err
    }

    surface.FillRect(nil, 0)
    window.UpdateSurface()

    for {
        if maxCycles > 0 && cpu.Cycle >= maxCycles {
            break
        }

        cycles := cpu.Cycle
        err = cpu.Run(instructionTable)
        if err != nil {
            return err
        }
        usedCycles := cpu.Cycle

        /* ppu runs 3 times faster than cpu */
        nmi := cpu.PPU.Run((usedCycles - cycles) * 3)

        if nmi {
            if cpu.Debug > 0 {
                log.Printf("Cycle %v Do NMI\n", cpu.Cycle)
            }
            cpu.NMI()
        }
    }

    return nil
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

    var nesPath string
    var debug bool
    var maxCycles uint64

    argIndex := 1
    for argIndex < len(os.Args) {
        arg := os.Args[argIndex]
        switch arg {
            case "-debug", "--debug":
                debug = true
            case "-cycles", "--cycles":
                var err error
                argIndex += 1
                if argIndex >= len(os.Args) {
                    log.Fatalf("Expected a number of cycles\n")
                }
                maxCycles, err = strconv.ParseUint(os.Args[argIndex], 10, 64)
                if err != nil {
                    log.Fatalf("Error parsing cycles: %v\n", err)
                }
            default:
                nesPath = arg
        }

        argIndex += 1
    }

    if nesPath != "" {
        err := Run(nesPath, debug, maxCycles)
        if err != nil {
            log.Printf("Error: %v\n", err)
        }
    } else {
        fmt.Printf("Give a .nes argument\n")
    }
}

package screenshot

import (
    nes "github.com/kazzmir/nes/lib"
    "image"
    "image/color"
)

type FakeButtons struct {
}

func (buttons *FakeButtons) Get() nes.ButtonMapping {
    return make(nes.ButtonMapping)
}

func ScreenToImage(screen nes.VirtualScreen) image.Image {
    out := image.NewRGBA(image.Rect(0, 0, screen.Width, screen.Height))

    for x := 0; x < screen.Width; x++ {
        for y := 0; y < screen.Height; y++ {
            r, g, b, a := screen.GetRGBA(x, y)
            out.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
        }
    }

    return out
}

/* Run a rom for maxCycles and return the last rendered screen */
func Run(rom string, maxCycles int64) (nes.VirtualScreen, error) {
    nesFile, err := nes.ParseNesFile(rom, false)
    if err != nil {
        return nes.VirtualScreen{}, err
    }

    cpu := nes.StartupState()

    cpu.PPU.SetHorizontalMirror(nesFile.HorizontalMirror)
    cpu.PPU.SetVerticalMirror(nesFile.VerticalMirror)

    maxCharacterRomLength := len(nesFile.CharacterRom)
    if maxCharacterRomLength > 0x2000 {
        maxCharacterRomLength = 0x2000
    }
    cpu.PPU.CopyCharacterRom(0x0000, nesFile.CharacterRom[:maxCharacterRomLength])

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return nes.VirtualScreen{}, err
    }
    cpu.SetMapper(mapper)

    cpu.Reset()
    cpu.Input = nes.MakeInput(&FakeButtons{})

    screen := nes.MakeVirtualScreen(256, 240)
    instructionTable := nes.MakeInstructionDescriptiontable()
    baseCyclesPerSample := 100.0

    buffer := nes.MakeVirtualScreen(256, 240)

    var lastCycle uint64 = 0
    totalCycles := int64(0)
    for totalCycles < maxCycles {
        err := cpu.Run(instructionTable)
        if err != nil {
            return nes.VirtualScreen{}, err
        }
        usedCycles := cpu.Cycle

        cycleDiff := usedCycles - lastCycle

        totalCycles += int64(cycleDiff)

        audioData := cpu.APU.Run(float64(cycleDiff) / 2.0, baseCyclesPerSample, &cpu)
        _ = audioData

        nmi, drawn := cpu.PPU.Run(cycleDiff * 3, screen, mapper)

        if drawn {
            buffer.CopyFrom(&screen)
        }

        if nmi {
            cpu.NMI()
        }

        lastCycle = usedCycles
    }

    return buffer, nil
}

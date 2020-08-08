package main

import (
    "os"
    "log"
    "fmt"
    "strconv"

    "path/filepath"
    nes "github.com/kazzmir/nes/lib"
    "github.com/kazzmir/nes/test/screenshot"

    "image/png"
)

func removeExtension(path string) string {
    extension := filepath.Ext(path)
    return path[0:len(path)-len(extension)]
}

func saveScreen(rom string, maxCycles int64, screen nes.VirtualScreen) error {
    romName := removeExtension(filepath.Base(rom))
    imagePath := fmt.Sprintf("images/%v-%v.png", romName, maxCycles)

    out, err := os.Create(imagePath)
    if err != nil {
        return err
    }
    defer out.Close()

    err = png.Encode(out, screenshot.ScreenToImage(screen))
    if err != nil {
        return err
    }

    log.Printf("Saved screenshot to %v", imagePath)

    return nil
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds)

    if len(os.Args) <= 2 {
        log.Printf("Give a .nes file and a number of cycles to run for")
        return
    }

    path := os.Args[1]
    cycles := os.Args[2]
    parsed, err := strconv.ParseInt(cycles, 10, 64)
    if err != nil {
        log.Printf("Could not parse cycles as an integer '%v': %v", cycles, err)
        return
    }

    if parsed <= 0 {
        log.Printf("Give a positive number of cycles: %v", parsed)
        return
    }

    buffer, err := screenshot.Run(path, parsed)
    if err != nil {
        log.Printf("Error: %v", err)
    }

    err = saveScreen(path, parsed, buffer)
    if err != nil {
        log.Printf("Could not save screenshot: %v", err)
    }
}

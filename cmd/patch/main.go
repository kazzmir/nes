package main

import (
    "os"
    "fmt"

    "github.com/kazzmir/nes/lib/patch"
)

func main() {
    if len(os.Args) < 3 {
        fmt.Printf("Usage: %s <rom_file> <patch1> [<patch2> ...]\n", os.Args[0])
        return
    }

    romFile := os.Args[1]

    romBytes, err := os.ReadFile(romFile)
    if err != nil {
        fmt.Printf("Error reading ROM file: %v\n", err)
        return
    }

    for _, patchFile := range os.Args[2:] {
        patchBytes, err := os.ReadFile(patchFile)
        if err != nil {
            fmt.Printf("Error reading patch file: %v\n", err)
            return
        }

        fmt.Printf("Applying patch: %s to ROM: %s\n", patchFile, romFile)
        fmt.Printf("Original ROM size: %d bytes\n", len(romBytes))
        fmt.Printf("Patch size: %d bytes\n", len(patchBytes))

        patchedRom, err := patch.ApplyPatch(romBytes, patchBytes)
        if err != nil {
            fmt.Printf("Error applying patch: %v\n", err)
            return
        }

        fmt.Printf("Patched ROM size: %d bytes\n", len(patchedRom))

        romBytes = patchedRom
    }

    outputFile := romFile + ".patched"
    err = os.WriteFile(outputFile, romBytes, 0644)
    if err != nil {
        fmt.Printf("Error writing patched ROM file: %v\n", err)
        return
    }

    fmt.Printf("Patched ROM written to: %s\n", outputFile)
}

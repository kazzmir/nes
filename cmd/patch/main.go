package main

import (
    "os"
    "fmt"

    "github.com/kazzmir/nes/lib/patch"
)

func main() {
    if len(os.Args) < 3 {
        fmt.Printf("Usage: %s <rom_file> <ips_patch>\n", os.Args[0])
        return
    }

    romFile := os.Args[1]
    patchFile := os.Args[2]

    romBytes, err := os.ReadFile(romFile)
    if err != nil {
        fmt.Printf("Error reading ROM file: %v\n", err)
        return
    }

    patchBytes, err := os.ReadFile(patchFile)
    if err != nil {
        fmt.Printf("Error reading IPS patch file: %v\n", err)
        return
    }

    fmt.Printf("Applying IPS patch: %s to ROM: %s\n", patchFile, romFile)
    fmt.Printf("Original ROM size: %d bytes\n", len(romBytes))
    fmt.Printf("Patch size: %d bytes\n", len(patchBytes))

    patchedRom, err := patch.ApplyIPSPatch(romBytes, patchBytes)
    if err != nil {
        fmt.Printf("Error applying IPS patch: %v\n", err)
        return
    }

    fmt.Printf("Patched ROM size: %d bytes\n", len(patchedRom))

    outputFile := romFile + ".patched"
    err = os.WriteFile(outputFile, patchedRom, 0644)
    if err != nil {
        fmt.Printf("Error writing patched ROM file: %v\n", err)
        return
    }

    fmt.Printf("Patched ROM written to: %s\n", outputFile)
}

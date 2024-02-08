package main

/* CLI utility that helps find/manipulate NES files for specific mappers */

import (
    "fmt"
    "flag"
    "os"
    "strings"
    "path/filepath"

    nes "github.com/kazzmir/nes/lib"
)

func getRoms(mapper uint32) []string {
    /* walk filesystem looking for .nes files and return those that use the given mapper */

    var out []string

    filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if strings.ToLower(filepath.Ext(path)) == ".nes" {
            nesFile, err := nes.ParseNesFile(path, false)
            if err == nil {
                if nesFile.Mapper == mapper {
                    out = append(out, path)
                }
            }
        }

        return nil
    })

    return out
}

func displayRoms(mapper uint32) {
    roms := getRoms(mapper)
    fmt.Printf("Found %d ROMs for mapper %d\n", len(roms), mapper)
    for _, rom := range roms {
        fmt.Printf("%s\n", rom)
    }
}

func main(){

    findMapper := flag.Int("find", -1, "Find all ROMs with a specific mapper")

    flag.Parse()

    if *findMapper != -1 {
        displayRoms(uint32(*findMapper))
    }
}

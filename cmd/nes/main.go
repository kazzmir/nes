package main

import (
    "fmt"
    "log"
    "io"
    "bytes"
    "os"
)

func isINes(check []byte) bool {
    if len(check) != 4 {
        return false
    }

    return bytes.Equal(check, []byte{'N', 'E', 'S', 0x1a})
}

func isNes2(nesHeader []byte) bool {
    if len(nesHeader) < 8 {
        return false
    }

    /* 0xc == 1100
     * 0x8 == 1000
     */

    /* this operation looks at bits 2 and 3, makes sure that bit 3 is 1
     * and bit 2 is 0
     */

    return nesHeader[7] & 0xc == 0x8
}

func parse(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }

    defer file.Close()

    header := make([]byte, 16)
    _, err = io.ReadFull(file, header)
    if err != nil {
        return err
    }

    ines := isINes(header[0:4])
    if !ines {
        return fmt.Errorf("not an nes file")
    }

    nes2 := isNes2(header)

    log.Printf("Nes 2.0 %v\n", nes2)

    return nil
}

func main(){
    if len(os.Args) > 1 {
        err := parse(os.Args[1])
        if err != nil {
            log.Printf("Error: %v\n", err)
        }
    } else {
        fmt.Printf("Give a .nes argument\n")
    }
}

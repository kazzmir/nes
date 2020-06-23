package lib

import (
    "bytes"
    "math"
    "io"
    "fmt"
    "os"
    "log"
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

func readPRG(header []byte) uint64 {
    lsb := header[4]
    /* only use the lowest 4 bits of byte 9 in the header */
    msb := header[9] & 15

    if msb == 15 {
        low2 := lsb & 3
        exponent := lsb >> 2
        return uint64(math.Pow(2.0, float64(exponent))) * uint64(low2*2 + 1)
    } else {
        return (uint64(msb << 8) + uint64(lsb)) << 14
    }
}

func readCHR(header []byte) uint64 {
    lsb := header[5]
    msb := (header[9] >> 4) & 15
    if msb == 15 {
        low2 := lsb & 3
        exponent := lsb >> 2
        return uint64(math.Pow(2.0, float64(exponent))) * uint64(low2*2 + 1)
    } else {
        return (uint64(msb << 8) + uint64(lsb)) << 13
    }
}

func readMapper(header []byte) byte {
    data := header[6]
    return data >> 4
}

type NESFile struct {
    ProgramRom []byte
    CharacterRom []byte
}

func ParseNesFile(path string) (NESFile, error) {
    file, err := os.Open(path)
    if err != nil {
        return NESFile{}, err
    }

    defer file.Close()

    header := make([]byte, 16)
    _, err = io.ReadFull(file, header)
    if err != nil {
        return NESFile{}, err
    }

    ines := isINes(header[0:4])
    if !ines {
        return NESFile{}, fmt.Errorf("not an nes file")
    }

    nes2 := isNes2(header)

    log.Printf("Nes 2.0 %v\n", nes2)

    prgRomSize := readPRG(header)
    chrRomSize := readCHR(header)

    mapper := readMapper(header)

    log.Printf("PRG-ROM %v\n", prgRomSize)
    log.Printf("CHR-ROM %v\n", chrRomSize)
    log.Printf("mapper %v\n", mapper)

    hasTrainer := (header[6] & 4) == 4
    log.Printf("Has trainer area %v\n", hasTrainer)

    log.Printf("Last 5 bytes: %v\n", header[11:])

    if hasTrainer {
        trainer := make([]byte, 512)
        _, err = io.ReadFull(file, trainer)
        if err != nil {
            return NESFile{}, err
        }
        log.Printf("Read trainer area\n")
    }

    programRom := make([]byte, prgRomSize)

    _, err = io.ReadFull(file, programRom)
    if err != nil {
        return NESFile{}, err
    }

    characterRom := make([]byte, chrRomSize)
    _, err = io.ReadFull(file, characterRom)
    if err != nil {
        return NESFile{}, err
    }

    return NESFile{
        ProgramRom: programRom,
        CharacterRom: characterRom,
    }, nil
}


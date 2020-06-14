package main

import (
    nes "github.com/kazzmir/nes/lib"
    "log"
    "strconv"
    "os"
    "strings"
    "bufio"
)

type Expected struct {
    Instruction nes.Instruction
    CPU nes.CPUState
}

func (expected *Expected) Compare(instruction nes.Instruction, cpu nes.CPUState) bool {
    /*
    if !expected.Instruction.Equals(instruction) {
        return false
    }
    */

    if !expected.CPU.Equals(cpu) {
        return false
    }

    return true
}

func parseValue(input string) byte {
    parts := strings.Split(input, ":")
    out, err  := strconv.ParseInt(parts[1], 16, 64)
    if err != nil {
        log.Fatalf("Could not parse hex value from %v: %v\n", input, err)
    }

    return byte(out)
}

func parseLine(line string) Expected {
    pc, err := strconv.ParseInt(line[0:4], 16, 64)
    if err != nil {
        log.Fatalf("Could not parse pc from '%v'\n", line)
    }

    /* TODO: parse instruction */

    values := line[48:]
    // log.Printf("Values '%v'\n", values)

    parts := strings.Split(values, " ")
    // log.Printf("Split: %v %v\n", len(parts), parts)

    A := parseValue(parts[0])
    X := parseValue(parts[1])
    Y := parseValue(parts[2])
    Status := parseValue(parts[3])
    SP := parseValue(parts[4])

    return Expected{
        nes.Instruction{
            Name: "blah",
            Kind: nes.Instruction_NOP,
        },
        nes.CPUState{
            PC: uint16(pc),
            A: A,
            X: X,
            Y: Y,
            Status: Status,
            SP: SP,
        },
    }
}

func parseLog(path string) ([]Expected, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)

    var out []Expected
    for scanner.Scan() {
        line := scanner.Text()
        out = append(out, parseLine(line))
    }

    return out, nil
}

func main(){
    rom := "test-roms/nestest.nes"
    logFile := "test-roms/nestest.log"

    nesFile, err := nes.ParseNesFile(rom)
    if err != nil {
        log.Fatalf("Unable to parse %v: %v\n", rom, err)
    }

    cpu := nes.StartupState()
    cpu.MapMemory(0xc000, nesFile.ProgramRom)
    cpu.Status = 0x24

    golden, err := parseLog(logFile)
    if err != nil {
        log.Fatalf("Error: %v\n", err)
    }

    for _, expected := range golden {
        instruction, err := cpu.Fetch()
        if err != nil {
            log.Fatalf("Error at PC 0x%X: %v\n", cpu.PC, err)
        }

        log.Printf("%X %v %v\n", cpu.PC, instruction.String(), cpu.String())

        if !expected.Compare(instruction, cpu) {
            log.Fatalf("Error: PC 0x%X Expected %v but had %v\n", cpu.PC, expected.CPU.String(), cpu.String())
        }

        err = cpu.Execute(instruction)
        if err != nil {
            log.Fatalf("Error: %v\n", err)
            return
        }
    }
}

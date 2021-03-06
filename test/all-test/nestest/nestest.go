package nestest

import (
    nes "github.com/kazzmir/nes/lib"
    "log"
    "strconv"
    "os"
    "strings"
    "bufio"
    "fmt"
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

func parseCycle(input string) uint64 {
    parts := strings.Split(input, ":")
    out, err  := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        log.Fatalf("Could not parse hex value from %v: %v\n", input, err)
    }

    return uint64(out)
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

    var cycle uint64 = 0
    for _, part := range parts {
        if strings.HasPrefix(part, "CYC") {
            cycle = parseCycle(part)
        }
    }

    return Expected{
        Instruction: nes.Instruction{
            Name: "blah",
            Kind: nes.Instruction_NOP_1,
        },
        CPU: nes.CPUState{
            PC: uint16(pc),
            A: A,
            X: X,
            Y: Y,
            Status: Status,
            SP: SP,
            Cycle: cycle,
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

func Run(debug bool) (bool, error){
    rom := "test-roms/nestest.nes"
    logFile := "test-roms/nestest.log"

    nesFile, err := nes.ParseNesFile(rom, debug)
    if err != nil {
        return false, err
    }

    cpu := nes.StartupState()

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return false, err
    }
    cpu.SetMapper(mapper)

    cpu.Status = 0x24
    cpu.PC = 0xc000
    /* FIXME: initiate the RESET process, which takes 6 clock cycles.
     * https://en.wikipedia.org/wiki/Interrupts_in_65xx_processors
     */
    cpu.Cycle = 7

    golden, err := parseLog(logFile)
    if err != nil {
        return false, err
    }

    table := nes.MakeInstructionDescriptiontable()

    for _, expected := range golden {
        instruction, err := cpu.Fetch(table)
        if err != nil {
            return false, fmt.Errorf("Error at PC 0x%X: %v\n", cpu.PC, err)
        }

        if debug {
            log.Printf("%X %v %v\n", cpu.PC, instruction.String(), cpu.String())
        }

        if !expected.Compare(instruction, cpu) {
            return false, fmt.Errorf("Error: PC 0x%X Expected %v but had %v", cpu.PC, expected.CPU.String(), cpu.String())
        }

        err = cpu.Execute(instruction)
        if err != nil {
            return false, err
        }
    }

    return true, nil
}

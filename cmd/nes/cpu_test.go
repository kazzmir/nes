package main

import (
    "testing"
    "io"
)

func readAllInstructions(reader *InstructionReader) ([]Instruction, error) {
    var out []Instruction

    for {
        instruction, err := reader.ReadInstruction()
        if err != nil {
            return out, err
        }

        out = append(out, instruction)
    }

    return out, nil
}

func checkInstructions(test *testing.T, instructions []Instruction, kinds []InstructionType) {
    if len(kinds) != len(instructions) {
        test.Fatalf("unequal number of instructions %v vs expected %v", len(instructions), len(kinds))
    }

    for i := 0; i < len(instructions); i++ {
        if instructions[i].Kind != kinds[i] {
            test.Fatalf("invalid instruction %v: %v vs %v\n", i, instructions[i].String(), kinds[i])
        }
    }
}

func TestCPUDecode(test *testing.T){
    bytes := []byte{0xa9, 0x01, 0x8d, 0x00, 0x02, 0xa9, 0x05, 0x8d, 0x01, 0x02, 0xa9, 0x08, 0x8d, 0x02, 0x02}

    reader := NewInstructionReader(bytes)
    instructions, err := readAllInstructions(reader)

    if err != nil {
        if err != io.EOF {
            test.Fatalf("could not read instructions: %v", err)
        }
    }

    checkInstructions(test, instructions, []InstructionType{
        Instruction_LDA_immediate,
        Instruction_STA_absolute,
        Instruction_LDA_immediate,
        Instruction_STA_absolute,
        Instruction_LDA_immediate,
        Instruction_STA_absolute,
    })
}

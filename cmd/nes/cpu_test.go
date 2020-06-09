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

func TestCPUSimple(test *testing.T){
    bytes := []byte{
        0xa9, 0x01,       // lda #$01
        0x8d, 0x00, 0x02, // sta #$200
        0xa9, 0x05,       // lda #$05
        0x8d, 0x01, 0x02, // sta #$201
        0xa9, 0x08,       // lda #$08
        0x8d, 0x02, 0x02, // sta #$202
    }

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

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0,
        PC: 0x100,
        Status: 0,
    }

    memory := NewMemory(0x3000)

    for _, instruction := range instructions {
        err = cpu.Execute(instruction, &memory)
        if err != nil {
            test.Fatalf("could not execute instruction %v\n", instruction.String())
        }
    }

    if cpu.A != 0x8 {
        test.Fatalf("A register expected to be 0x8 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0x0 {
        test.Fatalf("X register expected to be 0x0 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x0 {
        test.Fatalf("Y register expected to be 0x0 but was 0x%x\n", cpu.Y)
    }

    if cpu.PC != 0x10f {
        test.Fatalf("PC register expected to be 0x10f but was 0x%x\n", cpu.PC)
    }

    if memory.Load(0x200) != 0x1 {
        test.Fatalf("expected memory location 0x200 to contain 0x1 but was 0x%x\n", memory.Load(0x200))
    }

    if memory.Load(0x201) != 0x5 {
        test.Fatalf("expected memory location 0x201 to contain 0x5 but was 0x%x\n", memory.Load(0x201))
    }

    if memory.Load(0x202) != 0x8 {
        test.Fatalf("expected memory location 0x202 to contain 0x8 but was 0x%x\n", memory.Load(0x202))
    }
}

func TestCPUSimple2(test *testing.T){
    bytes := []byte{
        0xa9, 0xc0, // LDA #$c0
        0xaa,       // tax
        0xe8,       // inx
        0x69, 0xc4, // adc #$c4
        0x00,       // brk
    }

    reader := NewInstructionReader(bytes)
    instructions, err := readAllInstructions(reader)

    if err != nil {
        if err != io.EOF {
            test.Fatalf("could not read instructions: %v", err)
        }
    }

    checkInstructions(test, instructions, []InstructionType{
        Instruction_LDA_immediate,
        Instruction_TAX,
        Instruction_INX,
        Instruction_ADC_immediate,
        Instruction_BRK,
    })

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0,
        PC: 0x100,
        Status: 0,
    }

    memory := NewMemory(0x3000)

    for _, instruction := range instructions {
        err = cpu.Execute(instruction, &memory)
        if err != nil {
            test.Fatalf("could not execute instruction %v\n", instruction.String())
        }
    }

    if cpu.A != 0x84 {
        test.Fatalf("A register expected to be 0x84 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0xc1 {
        test.Fatalf("X register expected to be 0xc1 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x0 {
        test.Fatalf("Y register expected to be 0x0 but was 0x%x\n", cpu.Y)
    }

    if cpu.PC != 0x107 {
        test.Fatalf("PC register expected to be 0x107 but was 0x%x\n", cpu.PC)
    }
}

package lib

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
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))

    for _, instruction := range instructions {
        err = cpu.Execute(instruction)
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

    if cpu.LoadMemory(0x200) != 0x1 {
        test.Fatalf("expected memory location 0x200 to contain 0x1 but was 0x%x\n", cpu.LoadMemory(0x200))
    }

    if cpu.LoadMemory(0x201) != 0x5 {
        test.Fatalf("expected memory location 0x201 to contain 0x5 but was 0x%x\n", cpu.LoadMemory(0x201))
    }

    if cpu.LoadMemory(0x202) != 0x8 {
        test.Fatalf("expected memory location 0x202 to contain 0x8 but was 0x%x\n", cpu.LoadMemory(0x202))
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
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))

    for _, instruction := range instructions {
        err = cpu.Execute(instruction)
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

func TestCPUSimpleBranch(test *testing.T){
    bytes := []byte{
        0xa2, 0x08, // ldx #$08
        0xca,       // dex
        0x8e, 0x00, 0x02, // stx #$200
        0xe0, 0x03, // cpx #$03
        0xd0, 0xf8, // bne 0xf8
        0x8e, 0x01, 0x02, // stx #$201
        0x00, // brk
    }

    reader := NewInstructionReader(bytes)
    instructions, err := readAllInstructions(reader)

    if err != nil {
        if err != io.EOF {
            test.Fatalf("could not read instructions: %v", err)
        }
    }

    checkInstructions(test, instructions, []InstructionType{
        Instruction_LDX_immediate,
        Instruction_DEX,
        Instruction_STX_absolute,
        Instruction_CPX_immediate,
        Instruction_BNE,
        Instruction_STX_absolute,
        Instruction_BRK,
    })

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)

    for i := 0; i < 50; i++ {
        err = cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            test.Fatalf("Could not run CPU: %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.A != 0x0 {
        test.Fatalf("A register expected to be 0x0 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0x03 {
        test.Fatalf("X register expected to be 0x03 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x0 {
        test.Fatalf("Y register expected to be 0x0 but was 0x%x\n", cpu.Y)
    }

    if cpu.LoadMemory(0x200) != 0x3 {
        test.Fatalf("Expected memory location 0x200 to be 0x3 but was 0x%x\n", cpu.LoadMemory(0x200))
    }

    if cpu.LoadMemory(0x201) != 0x3 {
        test.Fatalf("Expected memory location 0x201 to be 0x3 but was 0x%x\n", cpu.LoadMemory(0x201))
    }
}

func TestInstructions1(testing *testing.T){
    bytes := []byte{
        0xa9, 0x0c, // LDA #$c0
        0xa8,       // tay
        0x8c, 0x03, 0x02, // sty #$203
        0x00, // brk
    }

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    for i := 0; i < 50; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.LoadMemory(0x203) != 0x0c {
        testing.Fatalf("Expected memory location 0x203 to be 0x0c but was 0x%x\n", cpu.LoadMemory(0x203))
    }
}

func TestInstructionsZeroPage(testing *testing.T){
    bytes := []byte{
        0xa2, 0x1, // ldx #$01
        0xa9, 0xaa, // lda #$aa
        0x95, 0xa0, // sta #$a0,x
        0xe8,       // inx
        0x95, 0xa0, // sta #$a0, x
        0x00, // brk
    }

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    for i := 0; i < 50; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.LoadMemory(0xa1) != 0xaa {
        testing.Fatalf("Expected memory location 0xa1 to be 0xaa but was 0x%x\n", cpu.LoadMemory(0xa1))
    }

    if cpu.LoadMemory(0xa2) != 0xaa {
        testing.Fatalf("Expected memory location 0xa2 to be 0xaa but was 0x%x\n", cpu.LoadMemory(0xa2))
    }
}

func TestInstructionsIndirectLoad(testing *testing.T){
    bytes := []byte{
        0xa2, 0x01, // ldx #$01
        0xa9, 0x05, // lda #$05
        0x85, 0x01, // sta #$01
        0xa9, 0x07, // lda #$07
        0x85, 0x02, // sta #$02
        0xa0, 0x0a, // ldy #$0a
        0x8c, 0x05, 0x07, // sty #$705
        0xa1, 0x00, // lda ($00,x)
        0x00, // brk
    }

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    for i := 0; i < 50; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.A != 0x0a {
        testing.Fatalf("Expected A register to be 0x0a but was 0x%x\n", cpu.A)
    }
}

func TestStack(testing *testing.T){
    /* store values 0x0 - 0xf into memory locations
     * 0x200 - 0x20f, then 0xf - 0x0 into 0x210 - 0x21f
     * first push the values onto the stack then pop them
     * off again.
     */
    bytes := []byte{
        0xa2, 0x00, // ldx #$00
        0xa0, 0x00, // ldy #$00
        0x8a,       // txa
        0x99, 0x00, 0x02, // sta #$0200,y
        0x48, // pha
        0xe8, // inx
        0xc8, // iny
        0xc0, 0x10, // cpy #$10
        0xd0, 0xf5, // bne
        0x68, // pla
        0x99, 0x00, 0x02, // sta #$0200,y
        0xc8, // iny
        0xc0, 0x20, // cpy #$20
        0xd0, 0xf7, // bne
        0x00, // brk
    }

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xff,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    cpu.SetStack(0x1000)
    for i := 0; i < 200; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.A != 0x0 {
        testing.Fatalf("Expected A register to be 0x0 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0x10 {
        testing.Fatalf("Expected X register to be 0x10 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x20 {
        testing.Fatalf("Expected Y register to be 0x20 but was 0x%x\n", cpu.Y)
    }

    for i := 0; i <= 0xf; i++ {
        address := uint16(0x200 + i)
        if cpu.LoadMemory(address) != byte(i) {
            testing.Fatalf("Expected memory location 0x%x to be 0x%x but was 0x%x\n", address, i, cpu.LoadMemory(address))
        }
    }

    for i := 0xf; i >= 0; i-- {
        address := uint16(0x21f - i)
        if cpu.LoadMemory(address) != byte(i) {
            testing.Fatalf("Expected memory location 0x%x to be 0x%x but was 0x%x\n", address, i, cpu.LoadMemory(address))
        }
    }
}

func TestSubroutine(testing *testing.T){
    bytes := []byte{
        0x20, 0x08, 0x50, // jsr
        0xa0, 0x10, // ldy #$10
        0x4c, 0x0c, 0x50, // jmp
        0xa2, 0x03, // ldx #$03
        0xe8, // inx
        0x60, // rts
        0x00, // brk
    }

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xff,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    cpu.SetStack(0x100)
    for i := 0; i < 200; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.A != 0x0 {
        testing.Fatalf("Expected A register to be 0x0 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0x4 {
        testing.Fatalf("Expected X register to be 0x4 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x10 {
        testing.Fatalf("Expected Y register to be 0x10 but was 0x%x\n", cpu.Y)
    }
}

func TestBit(testing *testing.T){
    /* 3&1 = 1, so dont set the zero flag */
    bytes := []byte{
        0xa9, 0x03, // lda #$03
        0x85, 0x10, // sta $10
        0xa9, 0x01, // lda #$01
        0x24, 0x10, // bit $10
        0x00, // brk
    }

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xff,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    for i := 0; i < 200; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.A != 0x1 {
        testing.Fatalf("Expected A register to be 0x1 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0x0 {
        testing.Fatalf("Expected X register to be 0x0 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x0 {
        testing.Fatalf("Expected Y register to be 0x0 but was 0x%x\n", cpu.Y)
    }

    if cpu.GetZeroFlag() {
        testing.Fatalf("Expected zero flag to be false but was %v\n", cpu.GetZeroFlag())
    }

    /* make sure zero flag gets set becuase 4&3=0 */
    bytes = []byte{
        0xa9, 0x03, // lda #$03
        0x85, 0x10, // sta $10
        0xa9, 0x04, // lda #$04
        0x24, 0x10, // bit $10
        0x00, // brk
    }

    cpu = CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xff,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    for i := 0; i < 200; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.A != 0x4 {
        testing.Fatalf("Expected A register to be 0x1 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0x0 {
        testing.Fatalf("Expected X register to be 0x0 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x0 {
        testing.Fatalf("Expected Y register to be 0x0 but was 0x%x\n", cpu.Y)
    }

    if !cpu.GetZeroFlag() {
        testing.Fatalf("Expected zero flag to be true but was %v\n", cpu.GetZeroFlag())
    }

    if cpu.GetNegativeFlag() {
        testing.Fatalf("Expected negative flag to be false but was %v\n", cpu.GetNegativeFlag())
    }

    if cpu.GetOverflowFlag() {
        testing.Fatalf("Expected overflow flag to be false but was %v\n", cpu.GetOverflowFlag())
    }


    /* N is set to 1 because highest bit of 0x80 is 1 */
    bytes = []byte{
        0xa9, 0x80, // lda #$80
        0x85, 0x10, // sta $10
        0xa9, 0x04, // lda #$04
        0x24, 0x10, // bit $10
        0x00, // brk
    }

    cpu = CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xff,
        PC: 0x5000,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x5000, bytes)
    for i := 0; i < 200; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            testing.Fatalf("Could not execute cpu %v\n", err)
        }

        if cpu.GetInterruptFlag() {
            break
        }
    }

    if cpu.A != 0x4 {
        testing.Fatalf("Expected A register to be 0x1 but was 0x%x\n", cpu.A)
    }

    if cpu.X != 0x0 {
        testing.Fatalf("Expected X register to be 0x0 but was 0x%x\n", cpu.X)
    }

    if cpu.Y != 0x0 {
        testing.Fatalf("Expected Y register to be 0x0 but was 0x%x\n", cpu.Y)
    }

    if !cpu.GetZeroFlag() {
        testing.Fatalf("Expected zero flag to be true but was %v\n", cpu.GetZeroFlag())
    }

    if !cpu.GetNegativeFlag() {
        testing.Fatalf("Expected negative flag to be true but was %v\n", cpu.GetNegativeFlag())
    }

    if cpu.GetOverflowFlag() {
        testing.Fatalf("Expected overflow flag to be false but was %v\n", cpu.GetOverflowFlag())
    }
}

func BenchmarkSimple(benchmark *testing.B){
    bytes := []byte{
        0xa2, 0x02, // ldx #$02
        0x8a, // txa
        0x85, 0x10, // sta $10
        0xe8, // inx
        0x4c, 0x00, 0x06, // jmp #$600
    }

    cpu := CPUState{
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xff,
        PC: 0x600,
        Status: 0,
        Maps: make(map[uint16][]byte),
    }

    cpu.MapMemory(0x0, NewMemory(0x3000))
    cpu.MapMemory(0x600, bytes)

    benchmark.ResetTimer()
    for i := 0; i < benchmark.N; i++ {
        err := cpu.Run(MakeInstructionDescriptiontable())
        if err != nil {
            benchmark.Fatalf("Could not execute cpu %v\n", err)
        }
    }
}

package main

import (
    "bytes"
    "fmt"
    "log"
    "io"
)

type InstructionReader struct {
    data io.Reader
    table map[InstructionType]InstructionDescription
}

type InstructionDescription struct {
    Name string
    Operands byte
}

type Instruction struct {
    Name string
    Kind InstructionType
    Operands []byte
}

func (instruction *Instruction) Length() uint16 {
    return 1 + uint16(len(instruction.Operands))
}

func (instruction *Instruction) OperandByte() (byte, error) {
    if len(instruction.Operands) != 1 {
        return 0, fmt.Errorf("dont have one operand for %v, only have %v", instruction.Name, len(instruction.Operands))
    }
    return instruction.Operands[0], nil
}

func (instruction *Instruction) OperandWord() (uint16, error) {
    if len(instruction.Operands) != 2 {
        return 0, fmt.Errorf("dont have two operands for %v, only have %v", instruction.Name, len(instruction.Operands))
    }
    high := instruction.Operands[1]
    low := instruction.Operands[0]
    return (uint16(high) << 8) | uint16(low), nil
}

func (instruction *Instruction) String() string {
    var out bytes.Buffer
    out.WriteString(instruction.Name)
    for _, operand := range instruction.Operands {
        out.WriteRune(' ')
        out.WriteString(fmt.Sprintf("0x%x", operand))
    }
    return out.String()
}

type InstructionType int

const (
    Instruction_BRK InstructionType = 0x00
    Instruction_ORA_d_x = 0x01
    Instruction_STP_02 = 0x02
    Instruction_STP_03 = 0x03
    Instruction_STP_04 = 0x04
    Instruction_ORA_zpg = 0x05
    Instruction_ASL_zero = 0x06
    Instruction_SLO_07 = 0x07
    Instruction_PHP = 0x08
    Instruction_ORA_immediate = 0x09
    Instruction_ASL_accumulator = 0x0a
    Instruction_ANC_0b = 0x0b
    Instruction_NOP_0c = 0x0c
    Instruction_ORA_abs = 0x0d
    Instruction_ASL_abs = 0x0e
    Instruction_SLO_abs = 0x0f
    Instruction_BPL_rel = 0x10
    Instruction_CLC = 0x18
    Instruction_JSR_absolute = 0x20
    Instruction_BIT_zero = 0x24
    Instruction_PLP = 0x28
    Instruction_AND_immediate = 0x29
    Instruction_ROL_accumulator = 0x2a
    Instruction_BIT_absolute = 0x2c
    Instruction_BMI_rel = 0x30
    Instruction_SEC = 0x38
    Instruction_RTI = 0x40
    Instruction_EOR_zero = 0x45
    Instruction_LSR_zero = 0x46
    Instruction_PHA = 0x48
    Instruction_EOR_immediate = 0x49
    Instruction_LSR_accumulator = 0x4a
    Instruction_JMP_absolute = 0x4c
    Instruction_BVC_rel = 0x50
    Instruction_SRE_y = 0x53
    Instruction_RTS = 0x60
    Instruction_ROR_zero = 0x66
    Instruction_PLA = 0x68
    Instruction_ADC_immediate = 0x69
    Instruction_ROR_accumulator = 0x6a
    Instruction_BVS_rel = 0x70
    Instruction_SEI_implied = 0x78
    Instruction_ADC_absolute_y = 0x79
    Instruction_STA_indirect_x = 0x81
    Instruction_STA_zero = 0x85
    Instruction_STX_zero = 0x86
    Instruction_DEY = 0x88
    Instruction_TXA = 0x8a
    Instruction_STY_absolute = 0x8c
    Instruction_STA_absolute = 0x8d
    Instruction_STX_absolute = 0x8e
    Instruction_BCC_rel = 0x90
    Instruction_STA_indirect_y = 0x91
    Instruction_STA_zeropage_x = 0x95
    Instruction_TYA = 0x98
    Instruction_TXS = 0x9a
    Instruction_STA_absolute_x = 0x9d
    Instruction_LDY_immediate = 0xa0
    Instruction_LDA_indirect_x = 0xa1
    Instruction_LDX_immediate = 0xa2
    Instruction_LDA_zero = 0xa5
    Instruction_LDX_zero = 0xa6
    Instruction_TAY = 0xa8
    Instruction_LDA_immediate = 0xa9
    Instruction_TAX = 0xaa
    Instruction_LDA_absolute = 0xad
    Instruction_BCS_rel = 0xb0
    Instruction_LDA_indirect_y = 0xb1
    Instruction_LDA_zero_x = 0xb5
    Instruction_CLV = 0xb8
    Instruction_TSX = 0xba
    Instruction_LDA_absolute_x = 0xbd
    Instruction_CMP_zero = 0xc5
    Instruction_DEC_zero = 0xc6
    Instruction_CMP_immediate = 0xc9
    Instruction_DEX = 0xca
    Instruction_BNE = 0xd0
    Instruction_CLD = 0xd8
    Instruction_CPX_immediate = 0xe0
    Instruction_INX = 0xe8
    Instruction_SBC_immediate = 0xe9
    Instruction_NOP = 0xea
    Instruction_INC_zero = 0xe6
    Instruction_BEQ_rel = 0xf0
    Instruction_Unknown_ff = 0xff
)

func NewInstructionReader(data []byte) *InstructionReader {
    table := make(map[InstructionType]InstructionDescription)
    table[Instruction_BRK] = InstructionDescription{Name: "brk", Operands: 0}
    table[Instruction_Unknown_ff] = InstructionDescription{Name: "unknown", Operands: 0}
    table[Instruction_BNE] = InstructionDescription{Name: "bne", Operands: 1}
    table[Instruction_RTS] = InstructionDescription{Name: "rts", Operands: 0}
    table[Instruction_BEQ_rel] = InstructionDescription{Name: "beq", Operands: 1}
    table[Instruction_BMI_rel] = InstructionDescription{Name: "bmi", Operands: 1}
    table[Instruction_BPL_rel] = InstructionDescription{Name: "bpl", Operands: 1}
    table[Instruction_BCC_rel] = InstructionDescription{Name: "bcc", Operands: 1}
    table[Instruction_BCS_rel] = InstructionDescription{Name: "bcs", Operands: 1}
    table[Instruction_BVC_rel] = InstructionDescription{Name: "bvc", Operands: 1}
    table[Instruction_BVS_rel] = InstructionDescription{Name: "bvs", Operands: 1}
    table[Instruction_LDA_immediate] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_zero] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_SEI_implied] = InstructionDescription{Name: "sei", Operands: 0}
    table[Instruction_STA_absolute] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_JSR_absolute] = InstructionDescription{Name: "jsr", Operands: 2}
    table[Instruction_LDA_absolute] = InstructionDescription{Name: "lda", Operands: 2}
    table[Instruction_LDX_immediate] = InstructionDescription{Name: "ldx", Operands: 1}
    table[Instruction_LDA_absolute_x] = InstructionDescription{Name: "lda", Operands: 2}
    table[Instruction_INX] = InstructionDescription{Name: "inx", Operands: 0}
    table[Instruction_JMP_absolute] = InstructionDescription{Name: "jmp", Operands: 2}
    table[Instruction_LDA_zero] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_LDY_immediate] = InstructionDescription{Name: "ldy", Operands: 1}
    table[Instruction_CMP_immediate] = InstructionDescription{Name: "cmp", Operands: 1}
    table[Instruction_CLC] = InstructionDescription{Name: "clc", Operands: 0}
    table[Instruction_ADC_immediate] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_PHA] = InstructionDescription{Name: "pha", Operands: 0}
    table[Instruction_PLA] = InstructionDescription{Name: "pla", Operands: 0}
    table[Instruction_NOP] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_STA_absolute_x] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_LDA_indirect_y] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_indirect_y] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_LDA_indirect_x] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_indirect_x] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_SBC_immediate] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_LSR_accumulator] = InstructionDescription{Name: "lsr", Operands: 0}
    table[Instruction_PHP] = InstructionDescription{Name: "php", Operands: 0}
    table[Instruction_PLP] = InstructionDescription{Name: "plp", Operands: 0}
    table[Instruction_TXA] = InstructionDescription{Name: "txa", Operands: 0}
    table[Instruction_TYA] = InstructionDescription{Name: "tya", Operands: 0}
    table[Instruction_TSX] = InstructionDescription{Name: "tsx", Operands: 0}
    table[Instruction_TAX] = InstructionDescription{Name: "tax", Operands: 0}
    table[Instruction_AND_immediate] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_TAY] = InstructionDescription{Name: "tay", Operands: 0}
    table[Instruction_INC_zero] = InstructionDescription{Name: "inc", Operands: 1}
    table[Instruction_ORA_immediate] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_DEC_zero] = InstructionDescription{Name: "dec", Operands: 1}
    table[Instruction_BIT_zero] = InstructionDescription{Name: "bit", Operands: 1}
    table[Instruction_STX_zero] = InstructionDescription{Name: "stx", Operands: 1}
    table[Instruction_EOR_zero] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_LSR_zero] = InstructionDescription{Name: "lsr", Operands: 1}
    table[Instruction_ROR_zero] = InstructionDescription{Name: "ror", Operands: 1}
    table[Instruction_ROR_accumulator] = InstructionDescription{Name: "ror", Operands: 0}
    table[Instruction_EOR_immediate] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_DEX] = InstructionDescription{Name: "dex", Operands: 0}
    table[Instruction_LDX_zero] = InstructionDescription{Name: "ldx", Operands: 1}
    table[Instruction_LDA_zero_x] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_SEC] = InstructionDescription{Name: "sec", Operands: 0}
    table[Instruction_ADC_absolute_y] = InstructionDescription{Name: "adc", Operands: 2}
    table[Instruction_DEY] = InstructionDescription{Name: "dey", Operands: 0}
    table[Instruction_ROL_accumulator] = InstructionDescription{Name: "rol", Operands: 0}
    table[Instruction_ASL_accumulator] = InstructionDescription{Name: "asl", Operands: 0}
    table[Instruction_CLV] = InstructionDescription{Name: "clv", Operands: 0}
    table[Instruction_TXS] = InstructionDescription{Name: "txs", Operands: 0}
    table[Instruction_BIT_absolute] = InstructionDescription{Name: "bit", Operands: 2}
    table[Instruction_STX_absolute] = InstructionDescription{Name: "stx", Operands: 2}
    table[Instruction_ASL_zero] = InstructionDescription{Name: "asl", Operands: 1}
    table[Instruction_CLD] = InstructionDescription{Name: "cld", Operands: 0}
    table[Instruction_RTI] = InstructionDescription{Name: "rti", Operands: 0}
    table[Instruction_CMP_zero] = InstructionDescription{Name: "cmp", Operands: 1}
    table[Instruction_CPX_immediate] = InstructionDescription{Name: "cpx", Operands: 1}
    table[Instruction_STY_absolute] = InstructionDescription{Name: "sty", Operands: 2}
    table[Instruction_STA_zeropage_x] = InstructionDescription{Name: "sta", Operands: 1}

    /* make sure I don't do something dumb */
    for key, value := range table {
        if value.Operands > 2 {
            panic(fmt.Sprintf("internal error: operands cannot be more than 2 for instruction %v: %v", key, value.Name))
        }
    }

    return &InstructionReader{
        data: bytes.NewReader(data),
        table: table,
    }
}

/* instructions can vary in their size */
func (reader *InstructionReader) ReadInstruction() (Instruction, error) {
    first := make([]byte, 1)
    _, err := io.ReadFull(reader.data, first)
    if err != nil {
        return Instruction{}, err
    }

    firstI := InstructionType(first[0])

    description, ok := reader.table[firstI]
    if !ok {
        return Instruction{}, fmt.Errorf("unknown instruction: 0x%x\n", first)
    }

    out := Instruction{
        Name: description.Name,
        Kind: firstI,
        Operands: nil,
    }

    operands := make([]byte, description.Operands)
    _, err = io.ReadFull(reader.data, operands)
    if err != nil {
        return Instruction{}, fmt.Errorf("unable to read %v operands for instruction %v", description.Operands, description.Name)
    }

    out.Operands = operands

    return out, nil
}

/* https://www.masswerk.at/6502/6502_instruction_set.html
 * A = accumulator
 * abs = absolute
 * n/# = immediate
 * impl = implied
 * ind = indirect
 * rel = relative
 * zpg = zeropage
 */

func dump_instructions(instructions []byte){
    reader := NewInstructionReader(instructions)

    count := 1
    for {
        instruction, err := reader.ReadInstruction()
        if err != nil {
            log.Printf("Error decoding instruction %v: %v\n", count, err)
            return
        }

        log.Printf("Instruction %v: %v\n", count, instruction.String())
        count += 1
    }
}

type CPUState struct {
    A byte
    X byte
    Y byte
    SP byte
    PC uint16
    Status byte

    CodeStart uint16
    Code []byte
}

func (cpu *CPUState) MapCode(location int, code []byte){
    cpu.CodeStart = uint16(location)
    cpu.Code = code
}

func (cpu *CPUState) Fetch() (Instruction, error) {
    where := cpu.PC - cpu.CodeStart
    if where < 0 {
        return Instruction{}, fmt.Errorf("Invalid PC value: %v", cpu.PC)
    }

    if int(where) >= len(cpu.Code) {
        return Instruction{}, fmt.Errorf("Invalid PC value: %v", cpu.PC)
    }

    use := cpu.Code[where:]
    /* FIXME: dont create a new reader each time */
    reader := NewInstructionReader(use)
    return reader.ReadInstruction()
}

func (cpu *CPUState) Run(memory *Memory) error {
    instruction, err := cpu.Fetch()
    if err != nil {
        return err
    }

    log.Printf("PC: 0x%x Execute instruction %v\n", cpu.PC, instruction.String())
    return cpu.Execute(instruction, memory)
}

func (cpu *CPUState) setBit(bit byte, set bool){
    if set {
        cpu.Status = cpu.Status | bit
    } else {
        cpu.Status = cpu.Status & (^bit)
    }
}

func (cpu *CPUState) getBit(bit byte) bool {
    return (cpu.Status & bit) == bit
}

func (cpu *CPUState) GetInterruptFlag() bool {
    return cpu.getBit(byte(0x4))
}

func (cpu *CPUState) SetInterruptFlag(set bool){
    cpu.setBit(byte(0x4), set)
}

func (cpu *CPUState) GetZeroFlag() bool {
    return cpu.getBit(byte(0x2))
}

func (cpu *CPUState) SetZeroFlag(zero bool){
    cpu.setBit(byte(0x2), zero)
}

type Memory struct {
    Data []byte
}

func NewMemory(size int) Memory {
    data := make([]byte, size)
    /* by default the data initializes to all 0's, but we could
     * put some other arbitrary byte value in each slot
     */
    return Memory{
        Data: data,
    }
}

func (memory *Memory) Store(address uint16, value byte){
    memory.Data[address] = value
}

func (memory *Memory) Load(address uint16) byte {
    return memory.Data[address]
}

func (cpu *CPUState) Execute(instruction Instruction, memory *Memory) error {
    switch instruction.Kind {
        case Instruction_LDA_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.A = value
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            memory.Store(address, cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            memory.Store(uint16(address), cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            zero_address := relative + cpu.X
            /* Load the two bytes at address $(relative+X) to
             * construct a 16-bit address
             */
            low := memory.Load(uint16(zero_address))
            high := memory.Load(uint16(zero_address+1))

            address := (uint16(high) << 8) | uint16(low)
            /* Then load the value at that 16-bit address */
            value := memory.Load(address)

            cpu.A = value
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDY_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.Y = value
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TAY:
            cpu.Y = cpu.A
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TAX:
            cpu.X = cpu.A
            cpu.PC += instruction.Length()
            return nil
        case Instruction_INX:
            /* FIXME: handle overflow */
            cpu.X += 1
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            /* FIXME: handle overflow */
            cpu.A += value
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STY_absolute:
            value, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            // log.Printf("Store Y:0x%x into 0x%x\n", cpu.Y, value)
            memory.Store(value, cpu.Y)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STX_absolute:
            value, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            // log.Printf("Store X:0x%x into 0x%x\n", cpu.X, value)
            memory.Store(value, cpu.X)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_zeropage_x:
            value, err := instruction.OperandByte()
            if err != nil {
                return nil
            }
            address := uint16(value + cpu.X)
            memory.Store(address, cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPX_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return nil
            }
            cpu.SetZeroFlag(cpu.X == value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_BNE:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()
            if !cpu.GetZeroFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }
            return nil
        case Instruction_LDX_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.X = value
            cpu.PC += instruction.Length()
            return nil
        case Instruction_DEX:
            cpu.X -= 1
            cpu.PC += instruction.Length()
            return nil
        case Instruction_BRK:
            cpu.SetInterruptFlag(true)
            cpu.PC += instruction.Length()
            return nil
    }

    return fmt.Errorf("unable to execute instruction 0x%x: %v", instruction.Kind, instruction.String())
}

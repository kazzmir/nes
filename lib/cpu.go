package lib

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

func equalBytes(a []byte, b[]byte) bool {
    for i := 0; i < len(a); i++ {
        if a[i] != b[i] {
            return false
        }
    }

    return true
}

func (instruction *Instruction) Equals(other Instruction) bool {
    return instruction.Name == other.Name &&
           instruction.Kind == other.Kind &&
           len(instruction.Operands) == len(other.Operands) &&
           equalBytes(instruction.Operands, other.Operands)
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
    Instruction_ORA_indirect_x = 0x01
    Instruction_STP_02 = 0x02
    Instruction_STP_03 = 0x03
    Instruction_STP_04 = 0x04
    Instruction_ORA_zero = 0x05
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
    Instruction_BPL = 0x10
    Instruction_CLC = 0x18
    Instruction_JSR = 0x20
    Instruction_AND_indirect_x = 0x21
    Instruction_BIT_zero = 0x24
    Instruction_AND_zero = 0x25
    Instruction_ROL_zero = 0x26
    Instruction_PLP = 0x28
    Instruction_AND_immediate = 0x29
    Instruction_ROL_accumulator = 0x2a
    Instruction_BIT_absolute = 0x2c
    Instruction_AND_absolute = 0x2d
    Instruction_BMI = 0x30
    Instruction_SEC = 0x38
    Instruction_RTI = 0x40
    Instruction_EOR_indirect_x = 0x41
    Instruction_EOR_zero = 0x45
    Instruction_LSR_zero = 0x46
    Instruction_PHA = 0x48
    Instruction_EOR_immediate = 0x49
    Instruction_LSR_accumulator = 0x4a
    Instruction_JMP_absolute = 0x4c
    Instruction_BVC_relative = 0x50
    Instruction_SRE_y = 0x53
    Instruction_RTS = 0x60
    Instruction_ADC_indirect_x = 0x61
    Instruction_ADC_zero = 0x65
    Instruction_ROR_zero = 0x66
    Instruction_PLA = 0x68
    Instruction_ADC_immediate = 0x69
    Instruction_ROR_accumulator = 0x6a
    Instruction_BVS_relative = 0x70
    Instruction_SEI = 0x78
    Instruction_ADC_absolute_y = 0x79
    Instruction_STA_indirect_x = 0x81
    Instruction_STY_zero = 0x84
    Instruction_STA_zero = 0x85
    Instruction_STX_zero = 0x86
    Instruction_DEY = 0x88
    Instruction_TXA = 0x8a
    Instruction_STY_absolute = 0x8c
    Instruction_STA_absolute = 0x8d
    Instruction_STX_absolute = 0x8e
    Instruction_BCC_relative = 0x90
    Instruction_STA_indirect_y = 0x91
    Instruction_STA_zeropage_x = 0x95
    Instruction_TYA = 0x98
    Instruction_STA_absolute_y = 0x99
    Instruction_TXS = 0x9a
    Instruction_STA_absolute_x = 0x9d
    Instruction_LDY_immediate = 0xa0
    Instruction_LDA_indirect_x = 0xa1
    Instruction_LDX_immediate = 0xa2
    Instruction_LDY_zero = 0xa4
    Instruction_LDA_zero = 0xa5
    Instruction_LDX_zero = 0xa6
    Instruction_TAY = 0xa8
    Instruction_LDA_immediate = 0xa9
    Instruction_TAX = 0xaa
    Instruction_LDA_absolute = 0xad
    Instruction_LDX_absolute = 0xae
    Instruction_BCS_relative = 0xb0
    Instruction_LDA_indirect_y = 0xb1
    Instruction_LDA_zero_x = 0xb5
    Instruction_CLV = 0xb8
    Instruction_TSX = 0xba
    Instruction_LDA_absolute_x = 0xbd
    Instruction_CPY_immediate = 0xc0
    Instruction_CMP_indirect_x = 0xc1
    Instruction_CPY_zero = 0xc4
    Instruction_CMP_zero = 0xc5
    Instruction_DEC_zero = 0xc6
    Instruction_INY = 0xc8
    Instruction_CMP_immediate = 0xc9
    Instruction_DEX = 0xca
    Instruction_BNE = 0xd0
    Instruction_CLD = 0xd8
    Instruction_CPX_immediate = 0xe0
    Instruction_SBC_indirect_x = 0xe1
    Instruction_CPX_zero = 0xe4
    Instruction_SBC_zero = 0xe5
    Instruction_INX = 0xe8
    Instruction_SBC_immediate = 0xe9
    Instruction_NOP = 0xea
    Instruction_INC_zero = 0xe6
    Instruction_BEQ_relative = 0xf0
    Instruction_SED = 0xf8
    Instruction_Unknown_ff = 0xff
)

func NewInstructionReader(data []byte) *InstructionReader {
    table := make(map[InstructionType]InstructionDescription)
    table[Instruction_BRK] = InstructionDescription{Name: "brk", Operands: 0}
    table[Instruction_Unknown_ff] = InstructionDescription{Name: "unknown", Operands: 0}
    table[Instruction_BNE] = InstructionDescription{Name: "bne", Operands: 1}
    table[Instruction_RTS] = InstructionDescription{Name: "rts", Operands: 0}
    table[Instruction_BEQ_relative] = InstructionDescription{Name: "beq", Operands: 1}
    table[Instruction_BMI] = InstructionDescription{Name: "bmi", Operands: 1}
    table[Instruction_BPL] = InstructionDescription{Name: "bpl", Operands: 1}
    table[Instruction_BCC_relative] = InstructionDescription{Name: "bcc", Operands: 1}
    table[Instruction_BCS_relative] = InstructionDescription{Name: "bcs", Operands: 1}
    table[Instruction_BVC_relative] = InstructionDescription{Name: "bvc", Operands: 1}
    table[Instruction_BVS_relative] = InstructionDescription{Name: "bvs", Operands: 1}
    table[Instruction_LDA_immediate] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_zero] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_STY_zero] = InstructionDescription{Name: "sty", Operands: 1}
    table[Instruction_SEI] = InstructionDescription{Name: "sei", Operands: 0}
    table[Instruction_STA_absolute] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_JSR] = InstructionDescription{Name: "jsr", Operands: 2}
    table[Instruction_LDA_absolute] = InstructionDescription{Name: "lda", Operands: 2}
    table[Instruction_LDX_immediate] = InstructionDescription{Name: "ldx", Operands: 1}
    table[Instruction_LDA_absolute_x] = InstructionDescription{Name: "lda", Operands: 2}
    table[Instruction_INX] = InstructionDescription{Name: "inx", Operands: 0}
    table[Instruction_JMP_absolute] = InstructionDescription{Name: "jmp", Operands: 2}
    table[Instruction_LDA_zero] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_LDY_immediate] = InstructionDescription{Name: "ldy", Operands: 1}
    table[Instruction_LDY_zero] = InstructionDescription{Name: "ldy", Operands: 1}
    table[Instruction_CMP_immediate] = InstructionDescription{Name: "cmp", Operands: 1}
    table[Instruction_CLC] = InstructionDescription{Name: "clc", Operands: 0}
    table[Instruction_ADC_immediate] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_ADC_zero] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_ADC_indirect_x] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_PHA] = InstructionDescription{Name: "pha", Operands: 0}
    table[Instruction_PLA] = InstructionDescription{Name: "pla", Operands: 0}
    table[Instruction_NOP] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_STA_absolute_x] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_LDA_indirect_y] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_indirect_y] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_LDA_indirect_x] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_indirect_x] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_SBC_immediate] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_SBC_zero] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_SBC_indirect_x] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_LSR_accumulator] = InstructionDescription{Name: "lsr", Operands: 0}
    table[Instruction_PHP] = InstructionDescription{Name: "php", Operands: 0}
    table[Instruction_PLP] = InstructionDescription{Name: "plp", Operands: 0}
    table[Instruction_TXA] = InstructionDescription{Name: "txa", Operands: 0}
    table[Instruction_TYA] = InstructionDescription{Name: "tya", Operands: 0}
    table[Instruction_TSX] = InstructionDescription{Name: "tsx", Operands: 0}
    table[Instruction_TAX] = InstructionDescription{Name: "tax", Operands: 0}
    table[Instruction_AND_immediate] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_AND_absolute] = InstructionDescription{Name: "and", Operands: 2}
    table[Instruction_AND_indirect_x] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_AND_zero] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_TAY] = InstructionDescription{Name: "tay", Operands: 0}
    table[Instruction_INC_zero] = InstructionDescription{Name: "inc", Operands: 1}
    table[Instruction_ORA_immediate] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_ORA_zero] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_ORA_indirect_x] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_DEC_zero] = InstructionDescription{Name: "dec", Operands: 1}
    table[Instruction_BIT_zero] = InstructionDescription{Name: "bit", Operands: 1}
    table[Instruction_STX_zero] = InstructionDescription{Name: "stx", Operands: 1}
    table[Instruction_EOR_zero] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_EOR_indirect_x] = InstructionDescription{Name: "eor", Operands: 1}
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
    table[Instruction_ROL_zero] = InstructionDescription{Name: "rol", Operands: 1}
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
    table[Instruction_CMP_indirect_x] = InstructionDescription{Name: "cmp", Operands: 1}
    table[Instruction_CPX_zero] = InstructionDescription{Name: "cpx", Operands: 1}
    table[Instruction_CPX_immediate] = InstructionDescription{Name: "cpx", Operands: 1}
    table[Instruction_STY_absolute] = InstructionDescription{Name: "sty", Operands: 2}
    table[Instruction_STA_zeropage_x] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_STA_absolute_y] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_INY] = InstructionDescription{Name: "iny", Operands: 0}
    table[Instruction_CPY_immediate] = InstructionDescription{Name: "cpy", Operands: 1}
    table[Instruction_CPY_zero] = InstructionDescription{Name: "cpy", Operands: 1}
    table[Instruction_SED] = InstructionDescription{Name: "sed", Operands: 0}
    table[Instruction_LDX_absolute] = InstructionDescription{Name: "ldx", Operands: 2}

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
    PC := 0
    reader := NewInstructionReader(instructions)

    count := 1
    for {
        instruction, err := reader.ReadInstruction()
        if err != nil {
            log.Printf("Error decoding instruction %v: %v\n", count, err)
            return
        }

        log.Printf("Instruction %v at pc 0x%x: %v\n", count, PC, instruction.String())
        PC += int(instruction.Length())
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

    Maps map[uint16][]byte
    StackBase uint16
}

func (cpu *CPUState) Equals(other CPUState) bool {
    return cpu.A == other.A &&
           cpu.X == other.X &&
           cpu.Y == other.Y &&
           cpu.SP == other.SP &&
           cpu.PC == other.PC &&
           cpu.Status == other.Status;
}

func (cpu *CPUState) String() string {
    return fmt.Sprintf("A:0x%X X:0x%X Y:0x%X SP:0x%X P:0x%X PC:0x%X", cpu.A, cpu.X, cpu.Y, cpu.SP, cpu.Status, cpu.PC)
}

func (cpu *CPUState) MapCode(location int, code []byte){
    cpu.CodeStart = uint16(location)
    cpu.Code = code
}

func (cpu *CPUState) MapMemory(location uint16, memory []byte) error {
    for base, memory := range cpu.Maps {
        if location >= base && location <= base + uint16(len(memory)) {
            return fmt.Errorf("Overlapping memory map with 0x%x - 0x%x", base, base + uint16(len(memory)))
        }
    }

    cpu.Maps[location] = memory
    return nil
}

func (cpu *CPUState) SetStack(location uint16){
    cpu.StackBase = location
}

func (cpu *CPUState) LoadMemory(address uint16) byte {
    large := uint64(address)
    for base, memory := range cpu.Maps {
        if large >= uint64(base) && large < uint64(base) + uint64(len(memory)) {
            return memory[address-base]
        }
    }

    /* FIXME: return an error? */
    log.Printf("Warning: accessing unmapped memory at 0x%x\n", address)
    return 0
}

func (cpu *CPUState) StoreMemory(address uint16, value byte) {
    large := uint64(address)
    for base, memory := range cpu.Maps {
        if large >= uint64(base) && large < uint64(base) + uint64(len(memory)) {
            memory[address-base] = value
            return
        }
    }

    log.Printf("Warning: could not access unmapped memory at 0x%x\n", address)
}

func (cpu *CPUState) LoadStack(where byte) byte {
    return cpu.LoadMemory(cpu.StackBase + uint16(where))
}

func (cpu *CPUState) StoreStack(where byte, value byte) {
    cpu.StoreMemory(cpu.StackBase + uint16(where), value)
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

func (cpu *CPUState) Run() error {
    instruction, err := cpu.Fetch()
    if err != nil {
        return err
    }

    cycle := 0
    log.Printf("PC: 0x%x Execute instruction %v A:%X X:%X Y:%X P:%X SP:%X CYC:%v\n", cpu.PC, instruction.String(), cpu.A, cpu.X, cpu.Y, cpu.Status, cpu.SP, cycle)
    return cpu.Execute(instruction)
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
    return cpu.getBit(byte(1<<2))
}

func (cpu *CPUState) SetInterruptFlag(set bool){
    cpu.setBit(byte(1<<2), set)
}

func (cpu *CPUState) GetZeroFlag() bool {
    return cpu.getBit(byte(0x2))
}

func (cpu *CPUState) SetZeroFlag(zero bool){
    cpu.setBit(byte(0x2), zero)
}

func (cpu *CPUState) SetCarryFlag(set bool){
    cpu.setBit(byte(0x1), set)
}

func (cpu *CPUState) GetCarryFlag() bool {
    return cpu.getBit(byte(0x1))
}

func (cpu *CPUState) GetNegativeFlag() bool {
    return cpu.getBit(byte(1 << 7))
}

func (cpu *CPUState) SetNegativeFlag(set bool) {
    cpu.setBit(byte(1 << 7), set)
}

func (cpu *CPUState) GetOverflowFlag() bool {
    return cpu.getBit(byte(1 << 6))
}

func (cpu *CPUState) SetOverflowFlag(set bool) {
    cpu.setBit(byte(1 << 6), set)
}

func (cpu *CPUState) SetDecimalFlag(set bool) {
    cpu.setBit(byte(1<<3), set)
}

func (cpu *CPUState) GetDecimalFlag() bool {
    return cpu.getBit(byte(1<<3))
}

type Memory struct {
    Data []byte
}

func NewMemory(size int) []byte {
    return make([]byte, size)
}

func NewMemory2(size int) Memory {
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

func (cpu *CPUState) doAnd(value byte){
    cpu.A = cpu.A & value
    cpu.SetNegativeFlag(int8(cpu.A) < 0)
    cpu.SetZeroFlag(cpu.A == 0)
}

func (cpu *CPUState) loadA(value byte){
    cpu.A = value
    cpu.SetNegativeFlag(int8(value) < 0)
    cpu.SetZeroFlag(cpu.A == 0)
}

func (cpu *CPUState) loadY(value byte){
    cpu.Y = value
    cpu.SetNegativeFlag(int8(cpu.Y) < 0)
    cpu.SetZeroFlag(cpu.Y == 0)
}

func (cpu *CPUState) loadX(value byte){
    cpu.X = value
    cpu.SetNegativeFlag(int8(cpu.X) < 0)
    cpu.SetZeroFlag(value == 0)
}

func (cpu *CPUState) ComputeIndirect(relative byte, index byte) uint16 {
    zero_address := relative + cpu.X
    /* Load the two bytes at address $(relative+X) to
    * construct a 16-bit address
    */
    low := cpu.LoadMemory(uint16(zero_address))
    high := cpu.LoadMemory(uint16(zero_address+1))

    return (uint16(high) << 8) | uint16(low)
}

func (cpu *CPUState) doOrA(value byte){
    cpu.A = cpu.A | value
    cpu.SetNegativeFlag(int8(cpu.A) < 0)
    cpu.SetZeroFlag(cpu.A == 0)
}

func (cpu *CPUState) doLsr(value byte) byte {
    carry := value & 1
    out := value >> 1
    cpu.SetNegativeFlag(false)
    cpu.SetZeroFlag(out == 0)
    cpu.SetCarryFlag(carry == 1)
    return out
}

func (cpu *CPUState) doRol(value byte) byte {
    var carryBit byte
    if cpu.GetCarryFlag() {
        carryBit = 1
    }

    newCarry := (value & (1<<7)) == (1<<7)
    out := (value << 1) | carryBit

    cpu.SetCarryFlag(newCarry)
    cpu.SetNegativeFlag(int8(out) < 0)
    cpu.SetZeroFlag(out == 0)
    return out
}

func (cpu *CPUState) doRor(value byte) byte {
    var carryBit byte
    if cpu.GetCarryFlag() {
        carryBit = 1
    }

    newCarry := (value & 1) == 1
    out := (value >> 1) | (carryBit << 7)
    cpu.SetCarryFlag(newCarry)
    cpu.SetNegativeFlag(int8(out) < 0)
    cpu.SetZeroFlag(out == 0)
    return out
}

func (cpu *CPUState) doAsl(value byte) byte {
    carry := value & (1<<7)
    out := value << 1
    cpu.SetNegativeFlag(int8(out) < 0)
    cpu.SetZeroFlag(out == 0)
    cpu.SetCarryFlag(carry == (1<<7))
    return out
}

func (cpu *CPUState) doEorA(value byte){
    cpu.A = cpu.A ^ value
    cpu.SetNegativeFlag(int8(cpu.A) < 0)
    cpu.SetZeroFlag(cpu.A == 0)
}

func (cpu *CPUState) doCpx(value byte){
    cpu.SetNegativeFlag(int8(cpu.X - value) < 0)
    cpu.SetCarryFlag(cpu.X >= value)
    cpu.SetZeroFlag(cpu.X == value)
}

func (cpu *CPUState) doCpy(value byte){
    cpu.SetNegativeFlag(int8(cpu.Y - value) < 0)
    cpu.SetCarryFlag(cpu.Y >= value)
    cpu.SetZeroFlag(cpu.Y == value)
}

func (cpu *CPUState) doAdc(value byte){
    /* 0010 1100
     * 0110 1100
     * NVBB DIZC
     */

    var carryBit byte = 0
    if cpu.GetCarryFlag() {
        carryBit = 1
    }

    /* set overflow when the result would not fit into a twos-complement number */
    full := int16(int8(cpu.A)) + int16(int8(value)) + int16(carryBit)

    /* set the carry flag when the result is larger than 8-bits */
    carry := int16(cpu.A) + int16(value) + int16(carryBit) > 0xff
    cpu.A = cpu.A + value + carryBit
    cpu.SetNegativeFlag(int8(cpu.A) < 0)

    /* set overflow if the value would not fit into a twos-complement number
    * http://www.6502.org/tutorials/vflag.html
    */
    cpu.SetOverflowFlag(full >= 128 || full <= -129)
    cpu.SetCarryFlag(carry)
    cpu.SetZeroFlag(cpu.A == 0)
}

func (cpu *CPUState) doSbc(value byte){
    var carryValue int8
    if !cpu.GetCarryFlag() {
        carryValue = 1
    }

    full := int16(int8(cpu.A)) - int16(int8(value)) - int16(carryValue)
    carry := int16(cpu.A) - int16(value) - int16(carryValue) >= 0

    result := int8(cpu.A) - int8(value) - carryValue
    cpu.A = byte(result)
    cpu.SetCarryFlag(carry)
    cpu.SetOverflowFlag(full >= 128 || full <= -129)
    cpu.SetNegativeFlag(result < 0)
    cpu.SetZeroFlag(result == 0)
}

func (cpu *CPUState) doCmp(value byte){
    result := int8(cpu.A) - int8(value)
    carry := cpu.A >= value
    cpu.SetCarryFlag(carry)
    cpu.SetNegativeFlag(result < 0)
    cpu.SetZeroFlag(result == 0)
}

func (cpu *CPUState) Execute(instruction Instruction) error {
    switch instruction.Kind {
        case Instruction_LDA_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadA(cpu.LoadMemory(uint16(address)))
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadA(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.StoreMemory(address, cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STY_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.StoreMemory(uint16(address), cpu.Y)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.StoreMemory(uint16(address), cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            /*
            zero_address := relative + cpu.X
            / * Load the two bytes at address $(relative+X) to
             * construct a 16-bit address
             * /
            low := cpu.LoadMemory(uint16(zero_address))
            high := cpu.LoadMemory(uint16(zero_address+1))

            address := (uint16(high) << 8) | uint16(low)
            / * Then load the value at that 16-bit address * /
            value := cpu.LoadMemory(address)
            */

            address := cpu.ComputeIndirect(relative, cpu.X)
            value := cpu.LoadMemory(address)

            cpu.A = value
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDX_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.loadX(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDY_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.loadY(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDY_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadY(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TAY:
            cpu.Y = cpu.A
            cpu.SetNegativeFlag(int8(cpu.Y) < 0)
            cpu.SetZeroFlag(cpu.Y == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TAX:
            cpu.X = cpu.A
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_INX:
            /* FIXME: handle overflow */
            cpu.X += 1
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirect(relative, cpu.X)
            value := cpu.LoadMemory(address)

            cpu.doAdc(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doAdc(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doAdc(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doSbc(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirect(relative, cpu.X)
            value := cpu.LoadMemory(address)

            cpu.doSbc(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_immediate:
            /* A := A - M - !C */
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doSbc(value)

            cpu.PC += instruction.Length()
            return nil

        case Instruction_STY_absolute:
            value, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            // log.Printf("Store Y:0x%x into 0x%x\n", cpu.Y, value)
            cpu.StoreMemory(value, cpu.Y)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STX_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.StoreMemory(uint16(address), cpu.X)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STX_absolute:
            value, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            // log.Printf("Store X:0x%x into 0x%x\n", cpu.X, value)
            cpu.StoreMemory(value, cpu.X)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.Y)
            cpu.StoreMemory(full, cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := cpu.ComputeIndirect(relative, cpu.X)
            cpu.StoreMemory(address, cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_zeropage_x:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(value + cpu.X)
            cpu.StoreMemory(address, cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_PLA:
            cpu.SP += 1
            cpu.A = cpu.LoadStack(cpu.SP)
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_PHP:
            /* PHP always sets the B flags to 1
             * http://wiki.nesdev.com/w/index.php/CPU_ALL#The_B_flag
             */
            value := cpu.Status | byte(1<<4) | byte(1<<5)
            cpu.StoreStack(cpu.SP, value)
            cpu.SP -= 1
            cpu.PC += instruction.Length()
            return nil
        case Instruction_PLP:
            cpu.SP += 1
            value := cpu.LoadStack(cpu.SP)
            /* 00110000 */
            b_bits := byte(0x30)
            /* the new status is all the non-b bits of the value pulled
             * from the stack, but include the existing b-bits already
             * set in the status register
             */
            cpu.Status = (value & (^b_bits)) | (cpu.Status & b_bits)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_PHA:
            cpu.StoreStack(cpu.SP, cpu.A)
            cpu.SP -= 1
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPY_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doCpy(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPY_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doCpy(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPX_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doCpx(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPX_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doCpx(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_BCC_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()

            if !cpu.GetCarryFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }

            return nil
        case Instruction_BIT_zero:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            /* pull from the zero page */
            value := cpu.LoadMemory(uint16(relative))

            cpu.SetZeroFlag((cpu.A & value) == 0)
            cpu.SetNegativeFlag((value & (1<<7)) == (1<<7))
            cpu.SetOverflowFlag((value & (1<<6)) == (1<<6))

            cpu.PC += instruction.Length()
            return nil
        case Instruction_BEQ_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()
            if cpu.GetZeroFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }

            return nil
        case Instruction_SEI:
            cpu.SetInterruptFlag(true)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_BCS_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()

            if cpu.GetCarryFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }
            return nil
        case Instruction_BMI:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()
            if cpu.GetNegativeFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }

            return nil
        case Instruction_BPL:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()
            if ! cpu.GetNegativeFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }

            return nil
        case Instruction_BVS_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()
            if cpu.GetOverflowFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }
            return nil
        /* branch on overflow clear */
        case Instruction_BVC_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()
            if !cpu.GetOverflowFlag() {
                cpu.PC = uint16(int(cpu.PC) + int(int8(value)))
            }
            return nil
        /* branch on zero flag clear */
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
        /* load X with an immediate value */
        case Instruction_LDX_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadX(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDX_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            value := cpu.LoadMemory(address)
            cpu.X = value
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            value := cpu.LoadMemory(address)
            cpu.A = value
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            return nil
        /* decrement X */
        case Instruction_DEX:
            cpu.X -= 1
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_DEY:
            cpu.Y -= 1
            cpu.SetNegativeFlag(int8(cpu.Y) < 0)
            cpu.SetZeroFlag(cpu.Y == 0)
            cpu.PC += instruction.Length()
            return nil
        /* A = X */
        case Instruction_TXA:
            cpu.A = cpu.X;
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TYA:
            cpu.A = cpu.Y;
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            return nil

        /* increment Y */
        case Instruction_INY:
            cpu.Y += 1
            cpu.SetNegativeFlag(int8(cpu.Y) < 0)
            cpu.SetZeroFlag(cpu.Y == 0)
            cpu.PC += instruction.Length()
            return nil
        /* push PC+2 on stack, jump to address */
        case Instruction_JSR:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            next := cpu.PC + 2

            low := byte(next & 0xff)
            high := byte(next >> 8)

            cpu.StoreStack(cpu.SP, high)
            cpu.SP -= 1
            cpu.StoreStack(cpu.SP, low)
            cpu.SP -= 1

            cpu.PC = address
            return nil
        case Instruction_AND_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doAnd(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_AND_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doAnd(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_AND_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doAnd(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_AND_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirect(relative, cpu.X)
            value := cpu.LoadMemory(address)
            cpu.doAnd(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RTS:
            cpu.SP += 1
            low := cpu.LoadStack(cpu.SP)
            cpu.SP += 1
            high := cpu.LoadStack(cpu.SP)

            cpu.PC = (uint16(high) << 8) + uint16(low) + 1

            return nil
        case Instruction_RTI:
            cpu.SP += 1
            value := cpu.LoadStack(cpu.SP)
            cpu.SP += 1
            low := cpu.LoadStack(cpu.SP)
            cpu.SP += 1
            high := cpu.LoadStack(cpu.SP)

            /* see PLP */
            b_bits := byte(0x30)
            cpu.Status = (value & (^b_bits)) | (cpu.Status & b_bits)

            cpu.PC = (uint16(high) << 8) | uint16(low)
            return nil
        case Instruction_LSR_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doLsr(value))
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LSR_accumulator:
            cpu.A = cpu.doLsr(cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ASL_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doAsl(value))
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ASL_accumulator:
            cpu.A = cpu.doAsl(cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_EOR_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doEorA(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_EOR_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirect(relative, cpu.X)
            value := cpu.LoadMemory(address)
            cpu.doEorA(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_EOR_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doEorA(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ORA_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doOrA(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ORA_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doOrA(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ORA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := cpu.ComputeIndirect(relative, cpu.X)
            value := cpu.LoadMemory(address)
            cpu.doOrA(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TSX:
            cpu.X = cpu.SP
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TXS:
            cpu.SP = cpu.X
            cpu.PC += instruction.Length()
            return nil
        case Instruction_DEC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address)) - 1
            cpu.StoreMemory(uint16(address), value)
            cpu.SetNegativeFlag(int8(value) < 0)
            cpu.SetZeroFlag(value == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_INC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address)) + 1
            cpu.StoreMemory(uint16(address), value)
            cpu.SetNegativeFlag(int8(value) < 0)
            cpu.SetZeroFlag(value == 0)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ROL_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doRol(value))
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ROL_accumulator:
            cpu.A = cpu.doRol(cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ROR_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doRor(value))
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ROR_accumulator:
            cpu.A = cpu.doRor(cpu.A)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CLV:
            cpu.SetOverflowFlag(false)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_JMP_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            cpu.PC = address
            return nil
        case Instruction_NOP:
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CMP_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doCmp(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CMP_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirect(relative, cpu.X)
            value := cpu.LoadMemory(address)
            cpu.doCmp(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CMP_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doCmp(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CLC:
            cpu.SetCarryFlag(false)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SEC:
            cpu.SetCarryFlag(true)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SED:
            cpu.SetDecimalFlag(true)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CLD:
            cpu.SetDecimalFlag(false)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_BRK:
            cpu.SetInterruptFlag(true)
            cpu.PC += instruction.Length()
            return nil
    }

    return fmt.Errorf("unable to execute instruction 0x%x: %v at PC 0x%x", instruction.Kind, instruction.String(), cpu.PC)
}

/* http://wiki.nesdev.com/w/index.php/CPU_power_up_state */
func StartupState() CPUState {
    cpu := CPUState {
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xfd,
        PC: 0xc000,
        Status: 0x34, // 110100
        Maps: make(map[uint16][]byte),
    }

    /* http://wiki.nesdev.com/w/index.php/CPU_memory_map */
    memory := NewMemory(0x800)
    cpu.MapMemory(0x0, memory)
    cpu.MapMemory(0x800, memory)
    cpu.MapMemory(0x1000, memory)
    cpu.MapMemory(0x1800, memory)
    cpu.SetStack(0x100)

    return cpu
}


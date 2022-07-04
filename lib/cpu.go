package lib

import (
    "bytes"
    "fmt"
    "log"
    "io"
    "encoding/json"
)

/* opcode references
 * http://wiki.nesdev.com/w/index.php/CPU_unofficial_opcodes -- nice table of opcodes
 * http://www.oxyron.de/html/opcodes02.html -- has illegal opcodes and their semantics
 * https://www.masswerk.at/6502/6502_instruction_set.html
 * http://www.6502.org/tutorials/6502opcodes.html
 * http://bbc.nvg.org/doc/6502OpList.txt
 */

const NMIVector uint16 = 0xfffa
const ResetVector uint16 = 0xfffc
const IRQVector uint16 = 0xfffe
const BRKVector uint16 = 0xfff6

/* http://wiki.nesdev.com/w/index.php/Cycle_reference_chart#Clock_rates
 * NTSC 2c0c clock speed is 21.47~ MHz ÷ 12 = 1.789773 MHz
 * Every second we should run this many cycles
 */
const CPUSpeed float64 = 1.789773e6

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
    out.WriteString(fmt.Sprintf("%02X ", instruction.Kind))
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
    Instruction_ORA_indirect_x =      0x01
    Instruction_KIL_1 =               0x02
    Instruction_SLO_indirect_x =      0x03
    Instruction_NOP_zero =            0x04
    Instruction_ORA_zero =            0x05
    Instruction_ASL_zero =            0x06
    Instruction_SLO_zero =            0x07
    Instruction_PHP =                 0x08
    Instruction_ORA_immediate =       0x09
    Instruction_ASL_accumulator =     0x0a
    Instruction_ANC_immediate_1 =     0x0b
    Instruction_NOP_absolute =        0x0c
    Instruction_ORA_absolute =        0x0d
    Instruction_ASL_absolute =        0x0e
    Instruction_SLO_absolute =        0x0f
    Instruction_BPL =                 0x10
    Instruction_ORA_indirect_y =      0x11
    Instruction_KIL_2 =               0x12
    Instruction_SLO_indirect_y =      0x13
    Instruction_NOP_zero_x =          0x14
    Instruction_ORA_zero_x =          0x15
    Instruction_ASL_zero_x =          0x16
    Instruction_SLO_zero_x =          0x17
    Instruction_CLC =                 0x18
    Instruction_ORA_absolute_y =      0x19
    Instruction_NOP_1 =               0x1a
    Instruction_SLO_absolute_y =      0x1b
    Instruction_NOP_absolute_x_1 =    0x1c
    Instruction_ORA_absolute_x =      0x1d
    Instruction_ASL_absolute_x =      0x1e
    Instruction_SLO_absolute_x =      0x1f
    Instruction_JSR =                 0x20
    Instruction_AND_indirect_x =      0x21
    Instruction_KIL_3 =               0x22
    Instruction_RLA_indirect_x =      0x23
    Instruction_BIT_zero =            0x24
    Instruction_AND_zero =            0x25
    Instruction_ROL_zero =            0x26
    Instruction_RLA_zero =            0x27
    Instruction_PLP =                 0x28
    Instruction_AND_immediate =       0x29
    Instruction_ROL_accumulator =     0x2a
    Instruction_ANC_immediate_2 =     0x2b
    Instruction_BIT_absolute =        0x2c
    Instruction_AND_absolute =        0x2d
    Instruction_ROL_absolute =        0x2e
    Instruction_RLA_absolute =        0x2f
    Instruction_BMI =                 0x30
    Instruction_AND_indirect_y =      0x31
    Instruction_KIL_4 =               0x32
    Instruction_RLA_indirect_y =      0x33
    Instruction_NOP_zero_x_1 =        0x34
    Instruction_AND_zero_x =          0x35
    Instruction_ROL_zero_x =          0x36
    Instruction_RLA_zero_x =          0x37
    Instruction_SEC =                 0x38
    Instruction_AND_absolute_y =      0x39
    Instruction_NOP_2 =               0x3a
    Instruction_RLA_absolute_y =      0x3b
    Instruction_NOP_absolute_x_2 =    0x3c
    Instruction_AND_absolute_x =      0x3d
    Instruction_ROL_absolute_x =      0x3e
    Instruction_RLA_absolute_x =      0x3f
    Instruction_RTI =                 0x40
    Instruction_EOR_indirect_x =      0x41
    Instruction_KIL_5 =               0x42
    Instruction_SRE_indirect_x =      0x43
    Instruction_NOP_zero_1 =          0x44
    Instruction_EOR_zero =            0x45
    Instruction_LSR_zero =            0x46
    Instruction_SRE_zero =            0x47
    Instruction_PHA =                 0x48
    Instruction_EOR_immediate =       0x49
    Instruction_LSR_accumulator =     0x4a
    Instruction_ALR =                 0x4b
    Instruction_JMP_absolute =        0x4c
    Instruction_EOR_absolute =        0x4d
    Instruction_LSR_absolute =        0x4e
    Instruction_SRE_absolute =        0x4f
    Instruction_BVC_relative =        0x50
    Instruction_EOR_indirect_y =      0x51
    Instruction_KIL_6 =               0x52
    Instruction_SRE_indirect_y =      0x53
    Instruction_NOP_zero_x_2 =        0x54
    Instruction_EOR_zero_x =          0x55
    Instruction_LSR_zero_x =          0x56
    Instruction_SRE_zero_x =          0x57
    Instruction_CLI =                 0x58
    Instruction_EOR_absolute_y =      0x59
    Instruction_NOP_3 =               0x5a
    Instruction_SRE_absolute_y =      0x5b
    Instruction_NOP_absolute_x_3 =    0x5c
    Instruction_EOR_absolute_x =      0x5d
    Instruction_LSR_absolute_x =      0x5e
    Instruction_SRE_absolute_x =      0x5f
    Instruction_RTS =                 0x60
    Instruction_ADC_indirect_x =      0x61
    Instruction_KIL_7 =               0x62
    Instruction_RRA_indirect_x =      0x63
    Instruction_NOP_zero_2 =          0x64
    Instruction_ADC_zero =            0x65
    Instruction_ROR_zero =            0x66
    Instruction_RRA_zero =            0x67
    Instruction_PLA =                 0x68
    Instruction_ADC_immediate =       0x69
    Instruction_ROR_accumulator =     0x6a
    Instruction_ARR =                 0x6b
    Instruction_JMP_indirect =        0x6c
    Instruction_ADC_absolute =        0x6d
    Instruction_ROR_absolute =        0x6e
    Instruction_RRA_absolute =        0x6f
    Instruction_BVS_relative =        0x70
    Instruction_ADC_indirect_y =      0x71
    Instruction_KIL_8 =               0x72
    Instruction_RRA_indirect_y =      0x73
    Instruction_NOP_zero_x_3 =        0x74
    Instruction_ADC_zero_x =          0x75
    Instruction_ROR_zero_x =          0x76
    Instruction_RRA_zero_x =          0x77
    Instruction_SEI =                 0x78
    Instruction_ADC_absolute_y =      0x79
    Instruction_NOP_4 =               0x7a
    Instruction_RRA_absolute_y =      0x7b
    Instruction_NOP_absolute_x_4 =    0x7c
    Instruction_ADC_absolute_x =      0x7d
    Instruction_ROR_absolute_x =      0x7e
    Instruction_RRA_absolute_x =      0x7f
    Instruction_NOP_immediate =       0x80
    Instruction_STA_indirect_x =      0x81
    Instruction_NOP_immediate_2 =     0x82
    Instruction_SAX_indirect_x =      0x83
    Instruction_STY_zero =            0x84
    Instruction_STA_zero =            0x85
    Instruction_STX_zero =            0x86
    Instruction_SAX_zero =            0x87
    Instruction_DEY =                 0x88
    Instruction_TXA =                 0x8a
    Instruction_STY_absolute =        0x8c
    Instruction_XAA =                 0x8b
    Instruction_STA_absolute =        0x8d
    Instruction_STX_absolute =        0x8e
    Instruction_SAX_absolute =        0x8f
    Instruction_BCC_relative =        0x90
    Instruction_STA_indirect_y =      0x91
    Instruction_KIL_9 =               0x92
    Instruction_AHX_indirect_y =      0x93
    Instruction_STY_zero_x =          0x94
    Instruction_STA_zero_x =          0x95
    Instruction_STX_zero_y =          0x96
    Instruction_SAX_zero_y =          0x97
    Instruction_TYA =                 0x98
    Instruction_STA_absolute_y =      0x99
    Instruction_TXS =                 0x9a
    Instruction_SHY =                 0x9c
    Instruction_STA_absolute_x =      0x9d
    Instruction_SHX =                 0x9e
    Instruction_AHX_absolute_y =      0x9f
    Instruction_LDY_immediate =       0xa0
    Instruction_LDA_indirect_x =      0xa1
    Instruction_LDX_immediate =       0xa2
    Instruction_LAX_indirect_x =      0xa3
    Instruction_LDY_zero =            0xa4
    Instruction_LDA_zero =            0xa5
    Instruction_LDX_zero =            0xa6
    Instruction_LAX_zero =            0xa7
    Instruction_TAY =                 0xa8
    Instruction_LDA_immediate =       0xa9
    Instruction_TAX =                 0xaa
    Instruction_LAX_immediate =       0xab
    Instruction_LDY_absolute =        0xac
    Instruction_LDA_absolute =        0xad
    Instruction_LDX_absolute =        0xae
    Instruction_LAX_absolute =        0xaf
    Instruction_BCS_relative =        0xb0
    Instruction_LDA_indirect_y =      0xb1
    Instruction_KIL_10 =              0xb2
    Instruction_LAX_indirect_y =      0xb3
    Instruction_LDY_zero_x =          0xb4
    Instruction_LDA_zero_x =          0xb5
    Instruction_LDX_zero_y =          0xb6
    Instruction_LAX_zero_y =          0xb7
    Instruction_CLV =                 0xb8
    Instruction_LDA_absolute_y =      0xb9
    Instruction_TSX =                 0xba
    Instruction_LDY_absolute_x =      0xbc
    Instruction_LDA_absolute_x =      0xbd
    Instruction_LDX_absolute_y =      0xbe
    Instruction_LAX_absolute_y =      0xbf
    Instruction_CPY_immediate =       0xc0
    Instruction_CMP_indirect_x =      0xc1
    Instruction_DCP_indirect_x =      0xc3
    Instruction_CPY_zero =            0xc4
    Instruction_CMP_zero =            0xc5
    Instruction_DEC_zero =            0xc6
    Instruction_DCP_zero =            0xc7
    Instruction_INY =                 0xc8
    Instruction_CMP_immediate =       0xc9
    Instruction_DEX =                 0xca
    Instruction_AXS =                 0xcb
    Instruction_CPY_absolute =        0xcc
    Instruction_CMP_absolute =        0xcd
    Instruction_DEC_absolute =        0xce
    Instruction_DCP_absolute =        0xcf
    Instruction_BNE =                 0xd0
    Instruction_CMP_indirect_y =      0xd1
    Instruction_KIL_11 =              0xd2
    Instruction_DCP_indirect_y =      0xd3
    Instruction_NOP_zero_x_4 =        0xd4
    Instruction_CMP_zero_x =          0xd5
    Instruction_DEC_zero_x =          0xd6
    Instruction_DCP_zero_x =          0xd7
    Instruction_CLD =                 0xd8
    Instruction_CMP_absolute_y =      0xd9
    Instruction_NOP_5 =               0xda
    Instruction_DCP_absolute_y =      0xdb
    Instruction_NOP_absolute_x_5 =    0xdc
    Instruction_CMP_absolute_x =      0xdd
    Instruction_DEC_absolute_x =      0xde
    Instruction_DCP_absolute_x =      0xdf
    Instruction_CPX_immediate =       0xe0
    Instruction_SBC_indirect_x =      0xe1
    Instruction_ISC_indirect_x =      0xe3
    Instruction_CPX_zero =            0xe4
    Instruction_SBC_zero =            0xe5
    Instruction_INC_zero =            0xe6
    Instruction_ISC_zero =            0xe7
    Instruction_INX =                 0xe8
    Instruction_SBC_immediate =       0xe9
    Instruction_NOP_6 =               0xea
    Instruction_SBC_immediate_1 =     0xeb
    Instruction_CPX_absolute =        0xec
    Instruction_SBC_absolute =        0xed
    Instruction_INC_absolute =        0xee
    Instruction_ISC_absolute =        0xef
    Instruction_BEQ_relative =        0xf0
    Instruction_SBC_indirect_y =      0xf1
    Instruction_KIL_12 =              0xf2
    Instruction_ISC_indirect_y =      0xf3
    Instruction_NOP_zero_x_5 =        0xf4
    Instruction_SBC_zero_x =          0xf5
    Instruction_INC_zero_x =          0xf6
    Instruction_ISC_zero_x =          0xf7
    Instruction_SED =                 0xf8
    Instruction_SBC_absolute_y =      0xf9
    Instruction_NOP_7 =               0xfa
    Instruction_ISC_absolute_y =      0xfb
    Instruction_NOP_absolute_x_6 =    0xfc
    Instruction_SBC_absolute_x =      0xfd
    Instruction_INC_absolute_x =      0xfe
    Instruction_ISC_absolute_x =      0xff
)

type InstructionTable map[InstructionType]InstructionDescription

func MakeInstructionDescriptiontable() InstructionTable {
    table := make(map[InstructionType]InstructionDescription)
    table[Instruction_BRK] = InstructionDescription{Name: "brk", Operands: 0}
    table[Instruction_BNE] = InstructionDescription{Name: "bne", Operands: 1}
    table[Instruction_RTS] = InstructionDescription{Name: "rts", Operands: 0}
    table[Instruction_BEQ_relative] = InstructionDescription{Name: "beq", Operands: 1}
    table[Instruction_BMI] = InstructionDescription{Name: "bmi", Operands: 1}
    table[Instruction_BPL] = InstructionDescription{Name: "bpl", Operands: 1}

    table[Instruction_ANC_immediate_1] = InstructionDescription{Name: "anc", Operands: 1}
    table[Instruction_ANC_immediate_2] = InstructionDescription{Name: "anc", Operands: 1}

    table[Instruction_AXS] = InstructionDescription{Name: "axs", Operands: 1}
    table[Instruction_ALR] = InstructionDescription{Name: "alr", Operands: 1}
    table[Instruction_ARR] = InstructionDescription{Name: "arr", Operands: 1}

    table[Instruction_SHY] = InstructionDescription{Name: "shy", Operands: 2}
    table[Instruction_SHX] = InstructionDescription{Name: "shy", Operands: 2}

    table[Instruction_AHX_indirect_y] = InstructionDescription{Name: "ahx", Operands: 1}
    table[Instruction_AHX_absolute_y] = InstructionDescription{Name: "ahx", Operands: 2}

    table[Instruction_XAA] = InstructionDescription{Name: "xaa", Operands: 1}

    table[Instruction_CLI] = InstructionDescription{Name: "cli", Operands: 0}

    table[Instruction_KIL_1] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_2] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_3] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_4] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_5] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_6] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_7] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_8] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_9] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_10] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_11] = InstructionDescription{Name: "kil", Operands: 0}
    table[Instruction_KIL_12] = InstructionDescription{Name: "kil", Operands: 0}

    table[Instruction_BCC_relative] = InstructionDescription{Name: "bcc", Operands: 1}
    table[Instruction_BCS_relative] = InstructionDescription{Name: "bcs", Operands: 1}
    table[Instruction_BVC_relative] = InstructionDescription{Name: "bvc", Operands: 1}
    table[Instruction_BVS_relative] = InstructionDescription{Name: "bvs", Operands: 1}
    table[Instruction_LDA_immediate] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_LDA_absolute_y] = InstructionDescription{Name: "lda", Operands: 2}
    table[Instruction_STA_zero] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_STY_zero] = InstructionDescription{Name: "sty", Operands: 1}
    table[Instruction_STY_zero_x] = InstructionDescription{Name: "sty", Operands: 1}

    table[Instruction_SRE_indirect_x] = InstructionDescription{Name: "sre", Operands: 1}
    table[Instruction_SRE_indirect_y] = InstructionDescription{Name: "sre", Operands: 1}
    table[Instruction_SRE_absolute_x] = InstructionDescription{Name: "sre", Operands: 2}
    table[Instruction_SRE_absolute_y] = InstructionDescription{Name: "sre", Operands: 2}
    table[Instruction_SRE_absolute] = InstructionDescription{Name: "sre", Operands: 2}
    table[Instruction_SRE_zero] = InstructionDescription{Name: "sre", Operands: 1}
    table[Instruction_SRE_zero_x] = InstructionDescription{Name: "sre", Operands: 1}

    table[Instruction_RRA_indirect_x] = InstructionDescription{Name: "rra", Operands: 1}
    table[Instruction_RRA_indirect_y] = InstructionDescription{Name: "rra", Operands: 1}
    table[Instruction_RRA_absolute_x] = InstructionDescription{Name: "rra", Operands: 2}
    table[Instruction_RRA_absolute_y] = InstructionDescription{Name: "rra", Operands: 2}
    table[Instruction_RRA_absolute] = InstructionDescription{Name: "rra", Operands: 2}
    table[Instruction_RRA_zero] = InstructionDescription{Name: "rra", Operands: 1}
    table[Instruction_RRA_zero_x] = InstructionDescription{Name: "rra", Operands: 1}

    table[Instruction_RLA_indirect_x] = InstructionDescription{Name: "rla", Operands: 1}
    table[Instruction_RLA_indirect_y] = InstructionDescription{Name: "rla", Operands: 1}
    table[Instruction_RLA_absolute_y] = InstructionDescription{Name: "rla", Operands: 2}
    table[Instruction_RLA_absolute_x] = InstructionDescription{Name: "rla", Operands: 2}
    table[Instruction_RLA_absolute] = InstructionDescription{Name: "rla", Operands: 2}
    table[Instruction_RLA_zero] = InstructionDescription{Name: "rla", Operands: 1}
    table[Instruction_RLA_zero_x] = InstructionDescription{Name: "rla", Operands: 1}
    table[Instruction_SEI] = InstructionDescription{Name: "sei", Operands: 0}
    table[Instruction_SLO_indirect_x] = InstructionDescription{Name: "slo", Operands: 1}
    table[Instruction_SLO_absolute_x] = InstructionDescription{Name: "slo", Operands: 2}
    table[Instruction_SLO_absolute_y] = InstructionDescription{Name: "slo", Operands: 2}
    table[Instruction_SLO_zero_x] = InstructionDescription{Name: "slo", Operands: 1}
    table[Instruction_SLO_indirect_y] = InstructionDescription{Name: "slo", Operands: 1}
    table[Instruction_SLO_zero] = InstructionDescription{Name: "slo", Operands: 1}
    table[Instruction_SLO_absolute] = InstructionDescription{Name: "slo", Operands: 2}
    table[Instruction_STA_absolute] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_SAX_indirect_x] = InstructionDescription{Name: "sax", Operands: 1}
    table[Instruction_SAX_absolute] = InstructionDescription{Name: "sax", Operands: 2}
    table[Instruction_SAX_zero] = InstructionDescription{Name: "sax", Operands: 1}
    table[Instruction_SAX_zero_y] = InstructionDescription{Name: "sax", Operands: 1}
    table[Instruction_JSR] = InstructionDescription{Name: "jsr", Operands: 2}
    table[Instruction_LDY_absolute] = InstructionDescription{Name: "ldy", Operands: 2}
    table[Instruction_LDY_absolute_x] = InstructionDescription{Name: "ldy", Operands: 2}
    table[Instruction_LDY_zero_x] = InstructionDescription{Name: "ldy", Operands: 1}
    table[Instruction_LDA_absolute] = InstructionDescription{Name: "lda", Operands: 2}
    table[Instruction_LDX_immediate] = InstructionDescription{Name: "ldx", Operands: 1}
    table[Instruction_LDX_absolute_y] = InstructionDescription{Name: "ldx", Operands: 2}
    table[Instruction_LDX_zero_y] = InstructionDescription{Name: "ldx", Operands: 1}
    table[Instruction_LDA_absolute_x] = InstructionDescription{Name: "lda", Operands: 2}
    table[Instruction_INX] = InstructionDescription{Name: "inx", Operands: 0}
    table[Instruction_JMP_absolute] = InstructionDescription{Name: "jmp", Operands: 2}
    table[Instruction_JMP_indirect] = InstructionDescription{Name: "jmp", Operands: 2}
    table[Instruction_LDA_zero] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_LDY_immediate] = InstructionDescription{Name: "ldy", Operands: 1}
    table[Instruction_LDY_zero] = InstructionDescription{Name: "ldy", Operands: 1}
    table[Instruction_ISC_zero_x] = InstructionDescription{Name: "isc", Operands: 1}
    table[Instruction_ISC_absolute_x] = InstructionDescription{Name: "isc", Operands: 2}
    table[Instruction_ISC_absolute_y] = InstructionDescription{Name: "isc", Operands: 2}
    table[Instruction_ISC_indirect_x] = InstructionDescription{Name: "isc", Operands: 1}
    table[Instruction_ISC_indirect_y] = InstructionDescription{Name: "isc", Operands: 1}
    table[Instruction_ISC_absolute] = InstructionDescription{Name: "isc", Operands: 2}
    table[Instruction_ISC_zero] = InstructionDescription{Name: "isc", Operands: 1}
    table[Instruction_DCP_indirect_x] = InstructionDescription{Name: "dcp", Operands: 1}
    table[Instruction_DCP_indirect_y] = InstructionDescription{Name: "dcp", Operands: 1}
    table[Instruction_DCP_absolute] = InstructionDescription{Name: "dcp", Operands: 2}
    table[Instruction_DCP_absolute_y] = InstructionDescription{Name: "dcp", Operands: 2}
    table[Instruction_DCP_absolute_x] = InstructionDescription{Name: "dcp", Operands: 2}
    table[Instruction_DCP_zero] = InstructionDescription{Name: "dcp", Operands: 1}
    table[Instruction_DCP_zero_x] = InstructionDescription{Name: "dcp", Operands: 1}
    table[Instruction_CMP_immediate] = InstructionDescription{Name: "cmp", Operands: 1}
    table[Instruction_CMP_absolute_x] = InstructionDescription{Name: "cmp", Operands: 2}
    table[Instruction_CMP_absolute_y] = InstructionDescription{Name: "cmp", Operands: 2}
    table[Instruction_CMP_zero_x] = InstructionDescription{Name: "cmp", Operands: 1}
    table[Instruction_CMP_indirect_y] = InstructionDescription{Name: "cmp", Operands: 1}
    table[Instruction_CMP_absolute] = InstructionDescription{Name: "cmp", Operands: 2}
    table[Instruction_CLC] = InstructionDescription{Name: "clc", Operands: 0}
    table[Instruction_LAX_immediate] = InstructionDescription{Name: "lax", Operands: 1}
    table[Instruction_LAX_indirect_x] = InstructionDescription{Name: "lax", Operands: 1}
    table[Instruction_LAX_zero_y] = InstructionDescription{Name: "lax", Operands: 1}
    table[Instruction_LAX_zero] = InstructionDescription{Name: "lax", Operands: 1}
    table[Instruction_LAX_absolute] = InstructionDescription{Name: "lax", Operands: 2}
    table[Instruction_LAX_absolute_y] = InstructionDescription{Name: "lax", Operands: 2}
    table[Instruction_LAX_indirect_y] = InstructionDescription{Name: "lax", Operands: 1}
    table[Instruction_ADC_immediate] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_ADC_absolute_x] = InstructionDescription{Name: "adc", Operands: 2}
    table[Instruction_ADC_zero_x] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_ADC_indirect_y] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_ADC_absolute] = InstructionDescription{Name: "adc", Operands: 2}
    table[Instruction_ADC_zero] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_ADC_indirect_x] = InstructionDescription{Name: "adc", Operands: 1}
    table[Instruction_PHA] = InstructionDescription{Name: "pha", Operands: 0}
    table[Instruction_PLA] = InstructionDescription{Name: "pla", Operands: 0}
    table[Instruction_NOP_immediate] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_immediate_2] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_absolute_x_1] = InstructionDescription{Name: "nop", Operands: 2}
    table[Instruction_NOP_absolute_x_2] = InstructionDescription{Name: "nop", Operands: 2}
    table[Instruction_NOP_absolute_x_3] = InstructionDescription{Name: "nop", Operands: 2}
    table[Instruction_NOP_absolute_x_4] = InstructionDescription{Name: "nop", Operands: 2}
    table[Instruction_NOP_absolute_x_5] = InstructionDescription{Name: "nop", Operands: 2}
    table[Instruction_NOP_absolute_x_6] = InstructionDescription{Name: "nop", Operands: 2}
    table[Instruction_NOP_1] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_NOP_2] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_NOP_3] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_NOP_4] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_NOP_5] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_NOP_6] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_NOP_7] = InstructionDescription{Name: "nop", Operands: 0}
    table[Instruction_NOP_absolute] = InstructionDescription{Name: "nop", Operands: 2}
    table[Instruction_NOP_zero_x] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero_x_1] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero_x_2] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero_x_3] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero_x_4] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero_x_5] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero_1] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_NOP_zero_2] = InstructionDescription{Name: "nop", Operands: 1}
    table[Instruction_STA_absolute_x] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_LDA_indirect_y] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_indirect_y] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_LDA_indirect_x] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_STA_indirect_x] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_SBC_immediate] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_SBC_immediate_1] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_SBC_absolute_x] = InstructionDescription{Name: "sbc", Operands: 2}
    table[Instruction_SBC_zero_x] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_SBC_absolute_y] = InstructionDescription{Name: "sbc", Operands: 2}
    table[Instruction_SBC_indirect_y] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_SBC_absolute] = InstructionDescription{Name: "sbc", Operands: 2}
    table[Instruction_SBC_zero] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_SBC_indirect_x] = InstructionDescription{Name: "sbc", Operands: 1}
    table[Instruction_LSR_accumulator] = InstructionDescription{Name: "lsr", Operands: 0}
    table[Instruction_LSR_absolute_x] = InstructionDescription{Name: "lsr", Operands: 2}
    table[Instruction_LSR_zero_x] = InstructionDescription{Name: "lsr", Operands: 1}
    table[Instruction_LSR_absolute] = InstructionDescription{Name: "lsr", Operands: 2}
    table[Instruction_PHP] = InstructionDescription{Name: "php", Operands: 0}
    table[Instruction_PLP] = InstructionDescription{Name: "plp", Operands: 0}
    table[Instruction_TXA] = InstructionDescription{Name: "txa", Operands: 0}
    table[Instruction_TYA] = InstructionDescription{Name: "tya", Operands: 0}
    table[Instruction_TSX] = InstructionDescription{Name: "tsx", Operands: 0}
    table[Instruction_TAX] = InstructionDescription{Name: "tax", Operands: 0}
    table[Instruction_AND_immediate] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_AND_absolute_x] = InstructionDescription{Name: "and", Operands: 2}
    table[Instruction_AND_zero_x] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_AND_absolute_y] = InstructionDescription{Name: "and", Operands: 2}
    table[Instruction_AND_indirect_y] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_AND_absolute] = InstructionDescription{Name: "and", Operands: 2}
    table[Instruction_AND_indirect_x] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_AND_zero] = InstructionDescription{Name: "and", Operands: 1}
    table[Instruction_TAY] = InstructionDescription{Name: "tay", Operands: 0}
    table[Instruction_INC_zero] = InstructionDescription{Name: "inc", Operands: 1}
    table[Instruction_INC_absolute_x] = InstructionDescription{Name: "inc", Operands: 2}
    table[Instruction_INC_zero_x] = InstructionDescription{Name: "inc", Operands: 1}
    table[Instruction_INC_absolute] = InstructionDescription{Name: "inc", Operands: 2}
    table[Instruction_ORA_immediate] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_ORA_absolute_x] = InstructionDescription{Name: "ora", Operands: 2}
    table[Instruction_ORA_zero_x] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_ORA_absolute_y] = InstructionDescription{Name: "ora", Operands: 2}
    table[Instruction_ORA_indirect_y] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_ORA_absolute] = InstructionDescription{Name: "ora", Operands: 2}
    table[Instruction_ORA_zero] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_ORA_indirect_x] = InstructionDescription{Name: "ora", Operands: 1}
    table[Instruction_DEC_zero] = InstructionDescription{Name: "dec", Operands: 1}
    table[Instruction_DEC_absolute_x] = InstructionDescription{Name: "dec", Operands: 2}
    table[Instruction_DEC_zero_x] = InstructionDescription{Name: "dec", Operands: 1}
    table[Instruction_DEC_absolute] = InstructionDescription{Name: "dec", Operands: 2}
    table[Instruction_BIT_zero] = InstructionDescription{Name: "bit", Operands: 1}
    table[Instruction_STX_zero] = InstructionDescription{Name: "stx", Operands: 1}
    table[Instruction_STX_zero_y] = InstructionDescription{Name: "stx", Operands: 1}
    table[Instruction_EOR_zero] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_EOR_absolute_x] = InstructionDescription{Name: "eor", Operands: 2}
    table[Instruction_EOR_zero_x] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_EOR_absolute_y] = InstructionDescription{Name: "eor", Operands: 2}
    table[Instruction_EOR_indirect_y] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_EOR_absolute] = InstructionDescription{Name: "eor", Operands: 2}
    table[Instruction_EOR_indirect_x] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_LSR_zero] = InstructionDescription{Name: "lsr", Operands: 1}
    table[Instruction_ROR_zero] = InstructionDescription{Name: "ror", Operands: 1}
    table[Instruction_ROR_absolute_x] = InstructionDescription{Name: "ror", Operands: 2}
    table[Instruction_ROR_zero_x] = InstructionDescription{Name: "ror", Operands: 1}
    table[Instruction_ROR_absolute] = InstructionDescription{Name: "ror", Operands: 2}
    table[Instruction_ROR_accumulator] = InstructionDescription{Name: "ror", Operands: 0}
    table[Instruction_EOR_immediate] = InstructionDescription{Name: "eor", Operands: 1}
    table[Instruction_DEX] = InstructionDescription{Name: "dex", Operands: 0}
    table[Instruction_LDX_zero] = InstructionDescription{Name: "ldx", Operands: 1}
    table[Instruction_LDA_zero_x] = InstructionDescription{Name: "lda", Operands: 1}
    table[Instruction_SEC] = InstructionDescription{Name: "sec", Operands: 0}
    table[Instruction_ADC_absolute_y] = InstructionDescription{Name: "adc", Operands: 2}
    table[Instruction_DEY] = InstructionDescription{Name: "dey", Operands: 0}
    table[Instruction_ROL_zero] = InstructionDescription{Name: "rol", Operands: 1}
    table[Instruction_ROL_absolute_x] = InstructionDescription{Name: "rol", Operands: 2}
    table[Instruction_ROL_zero_x] = InstructionDescription{Name: "rol", Operands: 1}
    table[Instruction_ROL_absolute] = InstructionDescription{Name: "rol", Operands: 2}
    table[Instruction_ROL_accumulator] = InstructionDescription{Name: "rol", Operands: 0}
    table[Instruction_ASL_accumulator] = InstructionDescription{Name: "asl", Operands: 0}
    table[Instruction_ASL_absolute_x] = InstructionDescription{Name: "asl", Operands: 2}
    table[Instruction_ASL_absolute] = InstructionDescription{Name: "asl", Operands: 2}
    table[Instruction_ASL_zero_x] = InstructionDescription{Name: "asl", Operands: 1}
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
    table[Instruction_CPX_absolute] = InstructionDescription{Name: "cpx", Operands: 2}
    table[Instruction_CPX_immediate] = InstructionDescription{Name: "cpx", Operands: 1}
    table[Instruction_STY_absolute] = InstructionDescription{Name: "sty", Operands: 2}
    table[Instruction_STA_zero_x] = InstructionDescription{Name: "sta", Operands: 1}
    table[Instruction_STA_absolute_y] = InstructionDescription{Name: "sta", Operands: 2}
    table[Instruction_INY] = InstructionDescription{Name: "iny", Operands: 0}
    table[Instruction_CPY_immediate] = InstructionDescription{Name: "cpy", Operands: 1}
    table[Instruction_CPY_absolute] = InstructionDescription{Name: "cpy", Operands: 2}
    table[Instruction_CPY_zero] = InstructionDescription{Name: "cpy", Operands: 1}
    table[Instruction_SED] = InstructionDescription{Name: "sed", Operands: 0}
    table[Instruction_LDX_absolute] = InstructionDescription{Name: "ldx", Operands: 2}

    /* make sure I don't do something dumb */
    for key, value := range table {
        if value.Operands > 2 {
            panic(fmt.Sprintf("internal error: operands cannot be more than 2 for instruction %v: %v", key, value.Name))
        }
    }

    return table
}

func NewInstructionReader(data []byte) *InstructionReader {
    return &InstructionReader{
        data: bytes.NewReader(data),
        table: MakeInstructionDescriptiontable(),
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

type Button int
const (
    ButtonIndexA Button = 0
    ButtonIndexB = 1
    ButtonIndexSelect = 2
    ButtonIndexStart = 3
    ButtonIndexUp = 4
    ButtonIndexDown = 5
    ButtonIndexLeft = 6
    ButtonIndexRight = 7
)

func AllButtons() []Button {
    return []Button{ButtonIndexA, ButtonIndexB, ButtonIndexSelect,
                    ButtonIndexStart, ButtonIndexUp, ButtonIndexDown,
                    ButtonIndexLeft, ButtonIndexRight}
}

type ButtonMapping map[Button]bool

type HostInput interface {
    Get() ButtonMapping
}

type Input struct {
    Buttons []bool
    NextRead byte
    Host HostInput
}

func (input *Input) Reset() {
    mapping := input.Host.Get()
    input.Buttons[ButtonIndexA] = mapping[ButtonIndexA]
    input.Buttons[ButtonIndexB] = mapping[ButtonIndexB]
    input.Buttons[ButtonIndexSelect] = mapping[ButtonIndexSelect]
    input.Buttons[ButtonIndexStart] = mapping[ButtonIndexStart]
    input.Buttons[ButtonIndexUp] = mapping[ButtonIndexUp]
    input.Buttons[ButtonIndexDown] = mapping[ButtonIndexDown]
    input.Buttons[ButtonIndexLeft] = mapping[ButtonIndexLeft]
    input.Buttons[ButtonIndexRight] = mapping[ButtonIndexRight]
}

func (input *Input) Read() byte {
    var out byte
    if input.Buttons[input.NextRead] {
        out = 1
    }
    input.NextRead = (input.NextRead + 1) % 8
    return out
}

func MakeInput(host HostInput) *Input {
    return &Input{
        Buttons: make([]bool, 8),
        NextRead: 0,
        Host: host,
    }
}

type CPUState struct {
    A byte `json:"a"`
    X byte `json:"x"`
    Y byte `json:"y"`
    SP byte `json:"sp"`
    PC uint16 `json:"pc"`
    Status byte `json:"status"`

    Cycle uint64 `json:"cycle"`

    /* holds a reference to the 2k ram the cpu can directly access.
     * this memory is mapped into Maps[] as well. Maps[] is redundant,
     * and can be removed at some point.
     */
    Ram []byte `json:"ram,omitempty"`
    Maps [][]byte `json:"-"`
    StackBase uint16 `json:"stackbase"`

    PPU PPUState `json:"ppu"`
    APU APUState `json:"apu"`
    Debug uint `json:"debug,omitempty"`
    StallCycles int `json:"stallcycles,omitempty"`

    /* controller input */
    Input *Input `json:"-"`

    Mapper Mapper `json:"-"`
}

func (cpu *CPUState) Serialize(writer io.Writer) error {
    encoder := json.NewEncoder(writer)
    return encoder.Encode(cpu)
}

func (cpu *CPUState) Load(other *CPUState){
    input := cpu.Input
    *cpu = other.Copy()
    cpu.Input = input
    cpu.Maps = make([][]byte, 256)

    cpu.MapMemory(0x0, cpu.Ram)
    cpu.MapMemory(0x800, cpu.Ram)
    cpu.MapMemory(0x1000, cpu.Ram)
    cpu.MapMemory(0x1800, cpu.Ram)
}

func (cpu *CPUState) Copy() CPUState {
    var mapper Mapper
    if cpu.Mapper != nil {
        mapper = cpu.Mapper.Copy()
    }
    return CPUState{
        A: cpu.A,
        X: cpu.X,
        Y: cpu.Y,
        SP: cpu.SP,
        PC: cpu.PC,
        Status: cpu.Status,
        Cycle: cpu.Cycle,
        Ram: copySlice(cpu.Ram),
        Maps: nil,
        StackBase: cpu.StackBase,
        PPU: cpu.PPU.Copy(),
        APU: cpu.APU.Copy(),
        Debug: cpu.Debug,
        StallCycles: cpu.StallCycles,
        Input: nil,
        Mapper: mapper,
    }
}

func (cpu *CPUState) Equals(other CPUState) bool {
    return cpu.A == other.A &&
           cpu.X == other.X &&
           cpu.Y == other.Y &&
           cpu.SP == other.SP &&
           cpu.PC == other.PC &&
           cpu.Cycle == other.Cycle &&
           cpu.Status == other.Status;
}

func (cpu *CPUState) String() string {
    return fmt.Sprintf("A:0x%X X:0x%X Y:0x%X SP:0x%X P:0x%X PC:0x%X Cycle:%v", cpu.A, cpu.X, cpu.Y, cpu.SP, cpu.Status, cpu.PC, cpu.Cycle)
}

func (cpu *CPUState) MapMemory(location uint16, memory []byte) error {

    if location & 0xff != 0 {
        return fmt.Errorf("Must map on a page boundary: %v", location)
    }

    if len(memory) % 256 != 0 {
        return fmt.Errorf("Mapping a non-page aligned memory slice: %v\n", len(memory))
    }

    // log.Printf("Mapping address 0x%x with 0x%x bytes\n", location, len(memory))
    base := location >> 8
    for page := 0; page < len(memory) / 256; page++ {
        use := base + uint16(page)
        if cpu.Maps[use] != nil {
            return fmt.Errorf("Memory is already mapped at page 0x%x\n", use)
        }

        // log.Printf("Map page 0x%x\n", use)
        cpu.Maps[use] = memory[page * 256:page*256 + 256]
    }

    /*
    for base, memory := range cpu.Maps {
        if location >= base && uint64(location) < uint64(base) + uint64(len(memory)) {
            return fmt.Errorf("Overlapping memory map with 0x%x - 0x%x", base, uint64(base) + uint64(len(memory)))
        }
    }

    cpu.Maps[location] = memory
    */
    return nil
}

func (cpu *CPUState) SetStack(location uint16){
    cpu.StackBase = location
}

func (cpu *CPUState) PushStack(value byte) {
    cpu.StoreStack(cpu.SP, value)
    /* FIXME: what happens when SP reaches 0? */
    cpu.SP -= 1
}

func (cpu *CPUState) PopStack() byte {
    cpu.SP += 1
    return cpu.LoadStack(cpu.SP)
}

func (cpu *CPUState) LoadMemory(address uint16) byte {
    // large := uint64(address)

    page := address >> 8
    if page >= 0x20 && page < 0x40 {
        /* every 8 bytes is mirrored, so only consider the last 3-bits of the address */
        use := address & 0x7
        switch 0x2000 | use {
            case PPUCTRL:
                log.Printf("Warning: reading from PPUCTRL location is not allowed\n")
                return 0
            case PPUMASK:
                log.Printf("Warning: reading from PPUMASK location is not allowed\n")
                return 0
            case PPUDATA:
                return cpu.PPU.ReadVideoMemory()
            case PPUSTATUS:
                return cpu.PPU.ReadStatus()
            case OAMDATA:
                return cpu.PPU.ReadOAM(byte(address))
        }

        log.Printf("Unhandled PPU read to 0x%x\n", address)
        return 0
    }

    switch address {
        case JOYPAD1:
            return cpu.Input.Read()
        case JOYPAD2:
            /* FIXME: handle player 2 input */
            return 0
        case APUStatus:
            return cpu.APU.ReadStatus()
    }

    if page >= 0x60 {
        if cpu.Mapper == nil {
            log.Printf("No mapper set, cannot read from mapper memory: 0x%x", address)
            return 0
        }
        return cpu.Mapper.Read(address)
    }

    if cpu.Maps[page] == nil {
        log.Printf("Warning: loading unmapped memory at 0x%x\n", address)
        return 0
    }

    return cpu.Maps[page][address & 0xff]

    /*
    for base, memory := range cpu.Maps {
        // log.Printf("Accessing memory 0x%x check 0x%x to 0x%x\n", address, uint64(base), uint64(base) + uint64(len(memory)))
        if large >= uint64(base) && large < uint64(base) + uint64(len(memory)) {
            return memory[address-base]
        }
    }
    */

    /* FIXME: return an error? */
    log.Printf("Warning: loading unmapped memory at 0x%x\n", address)
    return 0
}

/* Special PPU memory-mapped locations */
const (
    PPUCTRL uint16 = 0x2000
    PPUMASK = 0x2001
    PPUSTATUS = 0x2002
    OAMADDR = 0x2003
    OAMDATA = 0x2004
    PPUSCROLL = 0x2005
    PPUADDR = 0x2006
    PPUDATA = 0x2007
    OAMDMA = 0x4014
)

/* APU memory-mapped locations */
const (
    APUPulse1DutyCycle = 0x4000
    APUPulse1Sweep = 0x4001
    APUPulse1Timer = 0x4002
    APUPulse1Length = 0x4003
    APUPulse2DutyCycle = 0x4004
    APUPulse2Sweep = 0x4005
    APUPulse2Timer = 0x4006
    APUPulse2Length = 0x4007
    APUTriangleCounter = 0x4008
    APUTriangleIgnore = 0x4009
    APUTriangleTimerLow = 0x400A
    APUTriangleTimerHigh = 0x400B
    APUNoiseEnvelope = 0x400c
    APUNoiseMode = 0x400e
    APUNoiseIgnore = 0x400d
    APUNoiseLength = 0x400f
    APUDMCEnable = 0x4010
    APUDMCLoad = 0x4011
    APUDMCAddress = 0x4012
    APUDMCLength = 0x4013
    APUChannelEnable = 0x4015
    APUFrameCounter = 0x4017
    APUStatus = 0x4015 // for reading
)

/* Input memory-mapped locations */
const (
    INPUT_POLL = 0x4016
    JOYPAD1 = 0x4016
    JOYPAD2 = 0x4017
)

func (cpu *CPUState) SetMapper(mapper Mapper){
    cpu.Mapper = mapper
    // mapper.Initialize(cpu)
}

/* unmap the 32k block starting at 0x8000 */
func (cpu *CPUState) UnmapAllProgramMemory() {
    err := cpu.UnmapMemory(0x8000, 0x10000 - 0x8000)
    if err != nil {
        log.Printf("Warning: internal error could not unmap all memory %v", err)
    }
}

func (cpu *CPUState) UnmapMemory(address uint16, length uint16) error {
    if address & 0xff != 0 {
        return fmt.Errorf("Expected address to be page aligned: %v", address)
    }
    page := address >> 8

    if length & 0xff != 0 {
        return fmt.Errorf("Expected memory length to be page aligned: %v at 0x%x", length, address)
    }

    pages := length >> 8

    if page + pages > 0x100 {
        return fmt.Errorf("Cannot unmap pages past 0x100: 0x%x", page + pages)
    }

    for i := uint16(0); i < pages; i++ {
        cpu.Maps[page + i] = nil
    }

    return nil
}

func (cpu *CPUState) GetMemoryPage(address uint16) []byte {
    page := address >> 8

    return cpu.Maps[page]

    /*
    for base, memory := range cpu.Maps {
        if uint64(address) >= uint64(base) && uint64(address) < uint64(base) + uint64(len(memory)) {
            return memory[address:address+256]
        }
    }
    */

    return nil
}

func (cpu *CPUState) StoreMemory(address uint16, value byte) {
    // large := uint64(address)

    /* writes to certain ppu register are ignored before this cycle
     * http://wiki.nesdev.com/w/index.php/PPU_power_up_state
     */
    const ignore_ppu_write_cycle = 29658

    page := address >> 8
    if page >= 0x20 && page < 0x40 {
        /* every 8 bytes is mirrored, so only consider the last 3-bits of the address */
        use := address & 0x7
        switch 0x2000 | use {
            case PPUCTRL:
                if cpu.Cycle > ignore_ppu_write_cycle {
                    cpu.PPU.SetControllerFlags(value)
                    if cpu.Debug > 0 {
                        log.Printf("Set PPUCTRL to 0x%x: %v", value, cpu.PPU.ControlString())
                    }
                }
                return
            case PPUMASK:
                if cpu.Cycle > ignore_ppu_write_cycle {
                    cpu.PPU.SetMask(value)
                    if cpu.Debug > 0 {
                        log.Printf("Set PPUMASK to 0x%x: %v", value, cpu.PPU.MaskString())
                    }
                }
                return
            case PPUSCROLL:
                if cpu.Cycle > ignore_ppu_write_cycle {
                    if cpu.Debug > 0 {
                        log.Printf("Write 0x%x to PPUSCROLL", value)
                    }
                    cpu.PPU.WriteScroll(value)
                }
                return
            case PPUADDR:
                if cpu.Cycle > ignore_ppu_write_cycle {
                    if cpu.Debug > 0 {
                        log.Printf("Write 0x%x to PPUADDR", value)
                    }
                    cpu.PPU.WriteAddress(value)
                }
                return
            case PPUDATA:
                cpu.PPU.WriteVideoMemory(value)
                return
            case OAMADDR:
                cpu.PPU.SetOAMAddress(value)
                return
            case OAMDATA:
                cpu.PPU.WriteOAM(value)
                return
        }

        log.Printf("Unhandled PPU write to 0x%x\n", address)
        return
    }

    switch address {
        case APUPulse1DutyCycle:
            cpu.APU.WritePulse1Duty(value)
            return
        case APUPulse1Sweep:
            cpu.APU.WritePulse1Sweep(value)
            return
        case APUPulse1Timer:
            cpu.APU.WritePulse1Timer(value)
            return
        case APUPulse1Length:
            cpu.APU.WritePulse1Length(value)
            return
        case APUPulse2DutyCycle:
            cpu.APU.WritePulse2Duty(value)
            return
        case APUPulse2Sweep:
            cpu.APU.WritePulse2Sweep(value)
            return
        case APUPulse2Timer:
            cpu.APU.WritePulse2Timer(value)
            return
        case APUPulse2Length:
            cpu.APU.WritePulse2Length(value)
            return
        case APUTriangleCounter:
            cpu.APU.WriteTriangleCounter(value)
            return
        case APUTriangleIgnore:
            /* FIXME: some games write here 0x4009, its unclear why. just ignore
             * the writes for now
             */
            return
        case APUTriangleTimerLow:
            cpu.APU.WriteTriangleTimerLow(value)
            return
        case APUTriangleTimerHigh:
            cpu.APU.WriteTriangleTimerHigh(value)
            return
        case APUNoiseLength:
            cpu.APU.WriteNoiseLength(value)
            return
        case APUNoiseMode:
            cpu.APU.WriteNoiseMode(value)
            return
        case APUNoiseIgnore:
            /* FIXME: some games write here 0x400d, its unclear why. just ignore
             * the writes for now
             */
            return
        case APUNoiseEnvelope:
            cpu.APU.WriteNoiseEnvelope(value)
            return
        case APUChannelEnable:
            cpu.APU.WriteChannelEnable(value, cpu)
            return
        case APUFrameCounter:
            cpu.APU.WriteFrameCounter(value)
            return
        case APUDMCLoad:
            cpu.APU.WriteDMCLoad(value)
            return
        case APUDMCEnable:
            cpu.APU.WriteDMCEnable(value)
            return
        case APUDMCAddress:
            cpu.APU.WriteDMCAddress(value)
            return
        case APUDMCLength:
            cpu.APU.WriteDMCLength(value)
            return
        case INPUT_POLL:
            cpu.Input.Reset()
            return
        case OAMDMA:
            if cpu.Debug > 0 {
                log.Printf("Setting up OAM dma with 0x%x\n", value)
            }
            cpu.PPU.CopyOAM(cpu.GetMemoryPage(uint16(value) << 8))
            /* FIXME: 514 if on an odd cpu cycle */
            cpu.Stall(513)
            return
    }

    if address >= 0x6000 {
        err := cpu.Mapper.Write(cpu, address, value)
        if err != nil {
            log.Printf("Warning: writing to mapper memory: %v", err)
        }

        return
    }

    if cpu.Maps[page] == nil {
        log.Printf("Warning: could not store into unmapped memory at 0x%x value 0x%x\n", address, value)
    } else {
        cpu.Maps[page][address & 0xff] = value
    }

    /*
    for base, memory := range cpu.Maps {
        if large >= uint64(base) && large < uint64(base) + uint64(len(memory)) {
            memory[address-base] = value
            return
        }
    }
    */
}

func (cpu *CPUState) LoadStack(where byte) byte {
    return cpu.LoadMemory(cpu.StackBase + uint16(where))
}

func (cpu *CPUState) StoreStack(where byte, value byte) {
    cpu.StoreMemory(cpu.StackBase + uint16(where), value)
}

func (cpu *CPUState) Fetch(table InstructionTable) (Instruction, error) {
    first := cpu.LoadMemory(cpu.PC)
    firstI := InstructionType(first)

    description, ok := table[firstI]
    if !ok {
        return Instruction{}, fmt.Errorf("unknown instruction: 0x%x\n", first)
    }

    operands := make([]byte, description.Operands)
    for i := 0; i < int(description.Operands); i++ {
        operands[i] = cpu.LoadMemory(cpu.PC + uint16(i + 1))
    }

    instruction := Instruction{
        Name: description.Name,
        Kind: firstI,
        Operands: operands,
    }

    return instruction, nil
}

func (cpu *CPUState) Stall(cycles int){
    cpu.StallCycles += cycles
}

func (cpu *CPUState) Run(table InstructionTable) error {
    if cpu.StallCycles > 0 {
        cpu.StallCycles -= 1
        cpu.Cycle += 1
        return nil
    }

    if !cpu.GetInterruptDisableFlag() && cpu.IsIRQAsserted() {
        cpu.Interrupt()
    }

    instruction, err := cpu.Fetch(table)
    if err != nil {
        return err
    }

    if cpu.Debug > 0 {
        log.Printf("PC: 0x%x Execute instruction %v A:%X X:%X Y:%X P:%X SP:%X CYC:%v\n", cpu.PC, instruction.String(), cpu.A, cpu.X, cpu.Y, cpu.Status, cpu.SP, cpu.Cycle)
    }
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

func (cpu *CPUState) GetInterruptDisableFlag() bool {
    return cpu.getBit(byte(1<<2))
}

func (cpu *CPUState) SetInterruptDisableFlag(set bool){
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

/* returns a new address and whether a page boundary was crossed */
func (cpu *CPUState) ComputeIndirectY(relative byte) (uint16, bool) {
    /* Load two values from the zero page at (relative, relative+1)
     * Then construct a new address where low=(relative) and high=(relative+1)
     * Then add cpu.Y to the new address, and load the resulting address
     */
    low := uint16(cpu.LoadMemory(uint16(relative)))
    /* keeping 'relative' as a byte ensures wrap around works correctly */
    high := uint16(cpu.LoadMemory(uint16(relative + 1)))
    address := (high<<8) | low

    /* FIXME: not sure if converting Y to uint16 is correct */
    out := address + uint16(cpu.Y)

    /* If we crossed a page (256 bytes) then the upper 8 bits of 'out'
     * will be different than the upper 8 bits of address.
     * Given: address = 0x10f0, Y=0x20, then out=0x1110
     * and 0x11 != 0x10
     */
    page_cross := (out>>8) != (address>>8)

    return out, page_cross
}

/*
func (cpu *CPUState) LoadIndirectY(relative byte) byte {
    return cpu.LoadMemory(cpu.ComputeIndirectY(relative))
}
*/

func (cpu *CPUState) ComputeIndirectX(relative byte) uint16 {
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

func (cpu *CPUState) doBit(value byte){
    cpu.SetZeroFlag((cpu.A & value) == 0)
    cpu.SetNegativeFlag((value & (1<<7)) == (1<<7))
    cpu.SetOverflowFlag((value & (1<<6)) == (1<<6))
}

func (cpu *CPUState) doInc(value byte) byte {
    value = value + 1
    cpu.SetNegativeFlag(int8(value) < 0)
    cpu.SetZeroFlag(value == 0)
    return value
}

func (cpu *CPUState) doDec(value byte) byte {
    value = value - 1
    cpu.SetNegativeFlag(int8(value) < 0)
    cpu.SetZeroFlag(value == 0)
    return value
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

/* illegal opcode that combines inc with sbc */
func (cpu *CPUState) doIsc(address uint16){
    value := cpu.LoadMemory(address) + 1

    cpu.StoreMemory(address, value)
    /* FIXME: not totally sure sbc is the right thing to do here */
    cpu.doSbc(value)
}

/* illegal opcode that combines ROL with 'and' */
func (cpu *CPUState) doRla(address uint16){
    value := cpu.LoadMemory(address)
    /* FIXME: not sure if this is right */
    roled := cpu.doRol(value)
    cpu.StoreMemory(address, roled)
    cpu.doAnd(roled)
}

/* illegal opcode that combines ROR with adc */
func (cpu *CPUState) doRra(address uint16){
    original := cpu.LoadMemory(address)
    value := cpu.doRor(original)
    cpu.StoreMemory(address, value)
    cpu.doAdc(value)
}

/* illegal opcode that combines right-shift with xor */
func (cpu *CPUState) doSre(address uint16){
    original := cpu.LoadMemory(address)
    /* FIXME: not sure if this carry computation is right */
    carry := (original & 1) == 1
    value := original >> 1
    cpu.StoreMemory(address, value)
    cpu.doEorA(value)
    cpu.SetCarryFlag(carry)
}

/* illegal opcode that combines shift left with or */
func (cpu *CPUState) doSlo(address uint16){
    original := cpu.LoadMemory(address)
    carry := (original & (1<<7)) == (1<<7)
    value := original << 1
    cpu.StoreMemory(address, value)
    /* negative and zero flags are set here */
    cpu.doOrA(value)

    /* but carry is set based on the left-shift operation of memory */
    cpu.SetCarryFlag(carry)

    /*
    cpu.A = cpu.A | value

    cpu.SetNegativeFlag(int8(cpu.A) < 0)
    cpu.SetZeroFlag(cpu.A == 0)
    cpu.SetCarryFlag(carry == (1<<7))
    */
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
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadA(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (full>>8) != (address>>8)
            cpu.loadA(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_LDA_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            cpu.loadA(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_LDA_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadA(value)
            cpu.Cycle += 2
            cpu.PC += instruction.Length()
            return nil

        case Instruction_RRA_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            cpu.doRra(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RRA_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            cpu.doRra(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RRA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.doRra(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_RRA_zero:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero)
            cpu.doRra(address)
            cpu.Cycle += 5
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RRA_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            cpu.doRra(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RRA_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            _ = page_cross
            cpu.doRra(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 8
            return nil
        case Instruction_RRA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            cpu.doRra(address)
            cpu.Cycle += 8
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SRE_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            cpu.doSre(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SRE_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            cpu.doSre(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SRE_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.doSre(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SRE_zero:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero)
            cpu.doSre(address)
            cpu.Cycle += 5
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SRE_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            cpu.doSre(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SRE_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            _ = page_cross
            cpu.doSre(address)
            cpu.Cycle += 8
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SRE_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            cpu.doSre(address)
            cpu.Cycle += 8
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RLA_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            cpu.doRla(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RLA_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            cpu.doRla(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RLA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.doRla(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RLA_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            cpu.doRla(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RLA_zero:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero)
            cpu.doRla(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            return nil
        case Instruction_RLA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            cpu.doRla(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 8
            return nil
        case Instruction_RLA_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            _ = page_cross
            cpu.Cycle += 8
            cpu.doRla(address)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SLO_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            cpu.doSlo(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SLO_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            cpu.doSlo(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SLO_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            _ = page_cross
            cpu.doSlo(address)
            cpu.Cycle += 8
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SLO_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.doSlo(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SLO_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            cpu.doSlo(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SLO_zero:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero)
            cpu.doSlo(address)
            cpu.Cycle += 5
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SLO_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            cpu.doSlo(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 8
            return nil
        case Instruction_STA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.StoreMemory(address, cpu.A)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STY_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.StoreMemory(uint16(zero + cpu.X), cpu.Y)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STY_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.StoreMemory(uint16(address), cpu.Y)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STA_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.StoreMemory(uint16(address), cpu.A)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            value := cpu.LoadMemory(address)
            cpu.loadA(value)
            cpu.Cycle += 5
            if page_cross {
                cpu.Cycle += 1
            }
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)

            cpu.loadA(value)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDX_zero_y:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.Y)
            cpu.loadX(cpu.LoadMemory(address))
            cpu.Cycle += 4
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
            cpu.Cycle += 3
            return nil
        case Instruction_LDY_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadY(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDY_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.loadY(value)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDY_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadY(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_TAY:
            cpu.Y = cpu.A
            cpu.SetNegativeFlag(int8(cpu.Y) < 0)
            cpu.SetZeroFlag(cpu.Y == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_TAX:
            cpu.X = cpu.A
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_INX:
            /* FIXME: handle overflow */
            cpu.X += 1
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_ADC_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doAdc(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doAdc(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_ADC_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doAdc(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_ADC_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            value := cpu.LoadMemory(address)
            cpu.doAdc(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_ADC_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doAdc(value)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)

            cpu.doAdc(value)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doAdc(value)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ADC_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doAdc(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_XAA:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.A = cpu.X & value
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)

            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil

        case Instruction_SHY:
            return fmt.Errorf("cpu instruction SHY unimplemented")
        case Instruction_SHX:
            return fmt.Errorf("cpu instruction SHX unimplemented")

        case Instruction_AHX_indirect_y:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address, page_cross := cpu.ComputeIndirectY(value)
            _ = page_cross

            /* FIXME: probably not exactly right
             * http://forums.nesdev.com/viewtopic.php?f=3&t=10698&start=15
             */
            cpu.StoreMemory(address, cpu.X & cpu.A & (byte(address >> 8) + 1))

            cpu.Cycle += 6
            cpu.PC += instruction.Length()

            return nil

        case Instruction_AHX_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            /* FIXME: probably not exactly right */
            cpu.StoreMemory(address, cpu.X & cpu.A & (byte(address >> 8) + 1))

            cpu.Cycle += 6
            cpu.PC += instruction.Length()

            return nil

        case Instruction_ARR:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.A = cpu.doRor(cpu.A & value)
            /* FIXME: supposedly this sets the V overflow flag, but not sure how */

            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_ALR:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.A = cpu.doLsr(cpu.A & value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_AXS:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            /* FIXME: not 100% sure on this */
            result := int8(cpu.A & cpu.X) - int8(value)
            carry := result >= int8(value)
            cpu.SetCarryFlag(carry)
            cpu.SetNegativeFlag(result < 0)
            cpu.SetZeroFlag(result == 0)
            cpu.X = byte(result)

            cpu.Cycle += 2
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ANC_immediate_1,
             Instruction_ANC_immediate_2:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.A = cpu.A & value
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.SetCarryFlag((cpu.A & (1<<7)) == (1<<7))

            cpu.Cycle += 2
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doSbc(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_SBC_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doSbc(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_SBC_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            value := cpu.LoadMemory(address)
            cpu.doSbc(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_SBC_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doSbc(value)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doSbc(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doSbc(value)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)

            cpu.doSbc(value)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_SBC_immediate,
             Instruction_SBC_immediate_1:
            /* A := A - M - !C */
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doSbc(value)
            cpu.Cycle += 2

            cpu.PC += instruction.Length()
            return nil
        case Instruction_SAX_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.StoreMemory(address, cpu.A & cpu.X)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_SAX_zero_y:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.Y)
            cpu.StoreMemory(address, cpu.A & cpu.X)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_SAX_zero:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero)
            cpu.StoreMemory(address, cpu.A & cpu.X)
            cpu.PC += instruction.Length()
            cpu.Cycle += 3
            return nil
        case Instruction_SAX_indirect_x:
            /* illegal opcode. addr := A & X, where addr := (X,X+1) */
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            low := uint16(cpu.LoadMemory(uint16(zero + cpu.X)))
            high := uint16(cpu.LoadMemory(uint16(zero + cpu.X + 1)))
            address := (high<<8) | low
            cpu.StoreMemory(address, cpu.A & cpu.X)
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_STY_absolute:
            value, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            // log.Printf("Store Y:0x%x into 0x%x\n", cpu.Y, value)
            cpu.StoreMemory(value, cpu.Y)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STX_zero_y:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.Y)
            cpu.StoreMemory(address, cpu.X)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_STX_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.StoreMemory(uint16(address), cpu.X)
            cpu.PC += instruction.Length()
            cpu.Cycle += 3
            return nil
        case Instruction_STX_absolute:
            value, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            // log.Printf("Store X:0x%x into 0x%x\n", cpu.X, value)
            cpu.StoreMemory(value, cpu.X)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_STA_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.X)
            cpu.StoreMemory(full, cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            return nil
        case Instruction_STA_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.Y)
            cpu.StoreMemory(full, cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            return nil
        case Instruction_STA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := cpu.ComputeIndirectX(relative)
            cpu.StoreMemory(address, cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_STA_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            _ = page_cross
            cpu.StoreMemory(address, cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_STA_zero_x:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(value + cpu.X)
            cpu.StoreMemory(address, cpu.A)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_PLA:
            cpu.A = cpu.PopStack()
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_PHP:
            /* PHP always sets the B flags to 1
             * http://wiki.nesdev.com/w/index.php/CPU_ALL#The_B_flag
             */
            value := cpu.Status | byte(1<<4) | byte(1<<5)
            cpu.PushStack(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 3
            return nil
        case Instruction_PLP:
            value := cpu.PopStack()
            /* 00110000 */
            b_bits := byte(0x30)
            /* the new status is all the non-b bits of the value pulled
             * from the stack, but include the existing b-bits already
             * set in the status register
             */
            cpu.Status = (value & (^b_bits)) | (cpu.Status & b_bits)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_PHA:
            cpu.PushStack(cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 3
            return nil
        case Instruction_CPY_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doCpy(value)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPY_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doCpy(value)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPY_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doCpy(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_CPX_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doCpx(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_CPX_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doCpx(value)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CPX_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doCpx(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_BCC_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()

            if !cpu.GetCarryFlag() {
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_crossing := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1
                if page_crossing {
                    cpu.Cycle += 1
                }
            }

            cpu.Cycle += 2

            return nil
        case Instruction_BIT_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doBit(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_BIT_zero:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            /* pull from the zero page */
            value := cpu.LoadMemory(uint16(relative))

            cpu.doBit(value)

            cpu.Cycle += 3

            cpu.PC += instruction.Length()
            return nil
        case Instruction_BEQ_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()
            if cpu.GetZeroFlag() {
                /* FIXME: add a cycle for a page crossing only if the branch is taken,
                 * or should the extra cycle get used even if the branch is not taken?
                 */
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_cross := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1
                if page_cross {
                    cpu.Cycle += 1
                }
            }

            cpu.Cycle += 2

            return nil
        case Instruction_SEI:
            cpu.SetInterruptDisableFlag(true)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_BCS_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()

            if cpu.GetCarryFlag() {
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_crossing := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1
                if page_crossing {
                    cpu.Cycle += 1
                }
            }

            cpu.Cycle += 2

            return nil
        case Instruction_BMI:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()
            if cpu.GetNegativeFlag() {
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_cross := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1
                if page_cross {
                    cpu.Cycle += 1
                }
            }

            cpu.Cycle += 2

            return nil
        case Instruction_CLI:
            cpu.SetInterruptDisableFlag(false)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_KIL_1,
             Instruction_KIL_2,
             Instruction_KIL_3,
             Instruction_KIL_4,
             Instruction_KIL_5,
             Instruction_KIL_6,
             Instruction_KIL_7,
             Instruction_KIL_8,
             Instruction_KIL_9,
             Instruction_KIL_10,
             Instruction_KIL_11,
             Instruction_KIL_12:
            return fmt.Errorf("kil opcode")
        case Instruction_BPL:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.PC += instruction.Length()
            if ! cpu.GetNegativeFlag() {
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_crossing := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1
                if page_crossing {
                    cpu.Cycle += 1
                }
            }
            cpu.Cycle += 2

            return nil
        case Instruction_BVS_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()
            if cpu.GetOverflowFlag() {
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_crossing := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1

                if page_crossing {
                    cpu.Cycle += 1
                }
            }
            cpu.Cycle += 2
            return nil
        /* branch on overflow clear */
        case Instruction_BVC_relative:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()
            if !cpu.GetOverflowFlag() {
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_cross := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1
                if page_cross {
                    cpu.Cycle += 1
                }
            }
            cpu.Cycle += 2
            return nil
        /* branch on zero flag clear */
        case Instruction_BNE:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.PC += instruction.Length()
            if !cpu.GetZeroFlag() {
                newPC := uint16(int(cpu.PC) + int(int8(value)))
                page_crossing := (newPC >> 8) != (cpu.PC >> 8)
                cpu.PC = newPC
                cpu.Cycle += 1
                if page_crossing {
                    cpu.Cycle += 1
                }
            }
            cpu.Cycle += 2
            return nil
        /* load X with an immediate value */
        case Instruction_LDX_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.loadX(value)
            cpu.Cycle += 2
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LDX_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            value := cpu.LoadMemory(full)
            cpu.loadX(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_LDX_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            value := cpu.LoadMemory(address)
            cpu.loadX(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_LDY_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (address >> 8) != (full >> 8)
            cpu.loadY(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_LDY_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            value := cpu.LoadMemory(address)
            cpu.loadY(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_LDA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            value := cpu.LoadMemory(address)
            cpu.loadA(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        /* decrement X */
        case Instruction_DEX:
            cpu.X -= 1
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_DEY:
            cpu.Y -= 1
            cpu.SetNegativeFlag(int8(cpu.Y) < 0)
            cpu.SetZeroFlag(cpu.Y == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        /* A = X */
        case Instruction_TXA:
            cpu.A = cpu.X;
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_TYA:
            cpu.A = cpu.Y;
            cpu.SetNegativeFlag(int8(cpu.A) < 0)
            cpu.SetZeroFlag(cpu.A == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil

        /* increment Y */
        case Instruction_INY:
            cpu.Y += 1
            cpu.SetNegativeFlag(int8(cpu.Y) < 0)
            cpu.SetZeroFlag(cpu.Y == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
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

            cpu.PushStack(high)
            cpu.PushStack(low)

            cpu.PC = address
            cpu.Cycle += 6
            return nil
        case Instruction_AND_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doAnd(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_AND_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doAnd(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_AND_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            value := cpu.LoadMemory(address)
            cpu.Cycle += 5
            if page_cross {
                cpu.Cycle += 1
            }
            cpu.doAnd(value)
            cpu.PC += instruction.Length()
            return nil
        case Instruction_AND_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doAnd(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_AND_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doAnd(value)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_AND_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doAnd(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
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
            cpu.Cycle += 3
            return nil
        case Instruction_AND_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)
            cpu.doAnd(value)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_RTS:
            low := cpu.PopStack()
            high := cpu.PopStack()

            cpu.PC = (uint16(high) << 8) + uint16(low) + 1
            cpu.Cycle += 6

            return nil
        case Instruction_RTI:
            value := cpu.PopStack()
            low := cpu.PopStack()
            high := cpu.PopStack()

            /* see PLP */
            b_bits := byte(0x30)
            cpu.Status = (value & (^b_bits)) | (cpu.Status & b_bits)

            cpu.Cycle += 6
            cpu.PC = (uint16(high) << 8) | uint16(low)
            return nil
        case Instruction_LSR_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doLsr(value))
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_LSR_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doLsr(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            return nil
        case Instruction_LSR_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            value := cpu.LoadMemory(full)
            cpu.StoreMemory(full, cpu.doLsr(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 7
            return nil
        case Instruction_LSR_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doLsr(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_LSR_accumulator:
            cpu.A = cpu.doLsr(cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_ASL_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            value := cpu.LoadMemory(full)
            cpu.StoreMemory(full, cpu.doAsl(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 7
            return nil
        case Instruction_ASL_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doAsl(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_ASL_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doAsl(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_ASL_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doAsl(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            return nil
        case Instruction_ASL_accumulator:
            cpu.A = cpu.doAsl(cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_EOR_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doEorA(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_EOR_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doEorA(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_EOR_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            value := cpu.LoadMemory(address)
            cpu.doEorA(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_EOR_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doEorA(value)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_EOR_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doEorA(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
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
            cpu.Cycle += 3
            return nil
        case Instruction_EOR_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)
            cpu.doEorA(value)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_EOR_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doEorA(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_ORA_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doOrA(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ORA_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doOrA(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_ORA_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doOrA(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_ORA_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            value := cpu.LoadMemory(address)
            cpu.doOrA(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_ORA_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doOrA(value)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ORA_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doOrA(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_ORA_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            cpu.doOrA(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_ORA_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)
            cpu.doOrA(value)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_TSX:
            cpu.X = cpu.SP
            cpu.SetNegativeFlag(int8(cpu.X) < 0)
            cpu.SetZeroFlag(cpu.X == 0)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_TXS:
            cpu.SP = cpu.X
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_DEC_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            value := cpu.LoadMemory(full)
            cpu.StoreMemory(full, cpu.doDec(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 7
            return nil
        case Instruction_DEC_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doDec(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_DEC_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doDec(value))
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_DEC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doDec(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            return nil
        case Instruction_INC_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            value := cpu.LoadMemory(full)
            cpu.StoreMemory(full, cpu.doInc(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 7
            return nil
        case Instruction_INC_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doInc(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_INC_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doInc(value))
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_INC_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doInc(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            return nil
        case Instruction_ROL_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            value := cpu.LoadMemory(full)
            cpu.StoreMemory(full, cpu.doRol(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 7
            return nil
        case Instruction_ROL_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doRol(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_ROL_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doRol(value))
            cpu.Cycle += 6
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
            cpu.Cycle += 5
            return nil
        case Instruction_ROL_accumulator:
            cpu.A = cpu.doRol(cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_ROR_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            value := cpu.LoadMemory(full)
            cpu.StoreMemory(full, cpu.doRor(value))
            cpu.PC += instruction.Length()
            cpu.Cycle += 7
            return nil
        case Instruction_ROR_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doRor(value))
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ROR_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return nil
            }
            address := uint16(zero + cpu.X)
            value := cpu.LoadMemory(address)
            cpu.StoreMemory(address, cpu.doRor(value))
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ROR_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.StoreMemory(uint16(address), cpu.doRor(value))
            cpu.Cycle += 5
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ROR_accumulator:
            cpu.A = cpu.doRor(cpu.A)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_CLV:
            cpu.SetOverflowFlag(false)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_JMP_indirect:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            low := uint16(cpu.LoadMemory(address))
            /* adding 1 to the address used to get the high byte does
             * not use carry, so if address=0x30FF then the address
             * used to load the high byte is 0x3000 and not 0x3100
             * http://www.6502.org/tutorials/6502opcodes.html#JMP
             */
            xhigh := (address & 0xff00) | uint16((byte(address) + 1))
            high := uint16(cpu.LoadMemory(xhigh))
            full := (high<<8) | low
            cpu.PC = full
            cpu.Cycle += 5
            return nil
        case Instruction_JMP_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            cpu.PC = address
            cpu.Cycle += 3
            return nil
        case Instruction_NOP_immediate,
             Instruction_NOP_immediate_2:
            /* theres an operand but we dont need to use it */
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_NOP_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            /* FIXME: should we read memory here? */
            cpu.LoadMemory(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            return nil
        case Instruction_NOP_zero_x,
             Instruction_NOP_zero_x_1,
             Instruction_NOP_zero_x_2,
             Instruction_NOP_zero_x_3,
             Instruction_NOP_zero_x_4,
             Instruction_NOP_zero_x_5:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            /* FIXME: should we read memory here? */
            cpu.LoadMemory(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 4

            return nil
        case Instruction_NOP_zero,
             Instruction_NOP_zero_1,
             Instruction_NOP_zero_2:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            /* FIXME: should we read memory here? */
            cpu.LoadMemory(uint16(zero))
            cpu.PC += instruction.Length()
            /* FIXME: this is a guess */
            cpu.Cycle += 3
            return nil
        case Instruction_NOP_1,
             Instruction_NOP_2,
             Instruction_NOP_3,
             Instruction_NOP_4,
             Instruction_NOP_5,
             Instruction_NOP_6,
             Instruction_NOP_7:
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_NOP_absolute_x_1,
             Instruction_NOP_absolute_x_2,
             Instruction_NOP_absolute_x_3,
             Instruction_NOP_absolute_x_4,
             Instruction_NOP_absolute_x_5,
             Instruction_NOP_absolute_x_6:

             address, err := instruction.OperandWord()
             if err != nil {
                 return err
             }

             full := address + uint16(cpu.X)
             page_cross := (address >> 8) != (full >> 8)
             /* FIXME: should we read memory here? */
             cpu.LoadMemory(full)
             cpu.PC += instruction.Length()
             cpu.Cycle += 4
             if page_cross {
                 cpu.Cycle += 1
             }
             return nil
         case Instruction_LAX_immediate:
             value, err := instruction.OperandByte()
             if err != nil {
                 return err
             }

             cpu.A = value
             cpu.X = value

             cpu.SetNegativeFlag(int8(cpu.A) < 0)
             cpu.SetZeroFlag(cpu.A == 0)

             cpu.PC += instruction.Length()
             cpu.Cycle += 2
             return nil
         case Instruction_LAX_zero_y:
             zero, err := instruction.OperandByte()
             if err != nil {
                 return err
             }
             address := uint16(zero + cpu.Y)
             value := cpu.LoadMemory(address)
             cpu.loadA(value)
             cpu.X = value
             cpu.PC += instruction.Length()
             cpu.Cycle += 4
             return nil
         case Instruction_LAX_indirect_y:
             relative, err := instruction.OperandByte()
             if err != nil {
                 return err
             }
             address, page_cross := cpu.ComputeIndirectY(relative)
             value := cpu.LoadMemory(address)
             cpu.loadA(value)
             cpu.X = value
             cpu.PC += instruction.Length()
             cpu.Cycle += 5
             if page_cross {
                 cpu.Cycle += 1
             }
             return nil
         case Instruction_LAX_absolute_y:
             address, err := instruction.OperandWord()
             if err != nil {
                 return err
             }
             full := address + uint16(cpu.Y)
             page_cross := (address >> 8) != (full >> 8)
             value := cpu.LoadMemory(full)
             cpu.loadA(value)
             cpu.X = value
             cpu.PC += instruction.Length()
             cpu.Cycle += 4
             if page_cross {
                 cpu.Cycle += 1
             }
             return nil
         case Instruction_LAX_absolute:
             address, err := instruction.OperandWord()
             if err != nil {
                 return err
             }
             value := cpu.LoadMemory(address)
             cpu.loadA(value)
             cpu.X = value
             cpu.PC += instruction.Length()
             cpu.Cycle += 4
             return nil
         case Instruction_LAX_zero:
             zero, err := instruction.OperandByte()
             if err != nil {
                 return err
             }
             value := cpu.LoadMemory(uint16(zero))
             cpu.loadA(value)
             cpu.X = value
             cpu.PC += instruction.Length()
             cpu.Cycle += 3
             return nil
        case Instruction_LAX_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            /*
            low := uint16(cpu.LoadMemory(uint16(zero + cpu.X)))
            high := uint16(cpu.LoadMemory(uint16(zero + cpu.X + 1)))
            address := (high<<8) | low
            value := cpu.LoadMemory(address)
            */

            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)

            /* this is a crazy instruction that loads A and X at the same time */
            cpu.X = value
            cpu.loadA(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_CMP_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.X)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doCmp(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_CMP_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            full := address + uint16(cpu.Y)
            page_cross := (address >> 8) != (full >> 8)
            cpu.doCmp(cpu.LoadMemory(full))
            cpu.PC += instruction.Length()
            cpu.Cycle += 4
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_CMP_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            value := cpu.LoadMemory(address)
            cpu.doCmp(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 5
            if page_cross {
                cpu.Cycle += 1
            }
            return nil
        case Instruction_CMP_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doCmp(cpu.LoadMemory(uint16(zero + cpu.X)))
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CMP_zero:
            address, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(uint16(address))
            cpu.doCmp(value)
            cpu.Cycle += 3
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CMP_indirect_x:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address)
            cpu.doCmp(value)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CMP_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            value := cpu.LoadMemory(address)
            cpu.doCmp(value)
            cpu.Cycle += 4
            cpu.PC += instruction.Length()
            return nil
        case Instruction_CMP_immediate:
            value, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            cpu.doCmp(value)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_ISC_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.X)
            cpu.doIsc(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ISC_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.Y)
            cpu.doIsc(full)
            cpu.Cycle += 7
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ISC_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address := uint16(zero + cpu.X)
            cpu.doIsc(address)
            cpu.Cycle += 6
            cpu.PC += instruction.Length()
            return nil
        case Instruction_ISC_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }
            cpu.doIsc(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 6
            return nil
        case Instruction_ISC_zero:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := uint16(zero)
            cpu.doIsc(address)
            cpu.Cycle += 5

            cpu.PC += instruction.Length()
            return nil
        case Instruction_ISC_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }
            address, page_cross := cpu.ComputeIndirectY(relative)
            _ = page_cross
            cpu.doIsc(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 8
            return nil
        case Instruction_ISC_indirect_x:
            /* illegal opcode: (addr) = (addr) + 1, compare(A, (addr)) */
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := cpu.ComputeIndirectX(relative)
            cpu.doIsc(address)
            cpu.PC += instruction.Length()
            cpu.Cycle += 8
            return nil
        case Instruction_DCP_absolute_x:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.X)

            value := cpu.LoadMemory(full) - 1
            cpu.doCmp(value)
            cpu.StoreMemory(full, value)
            cpu.Cycle += 7

            cpu.PC += instruction.Length()
            return nil
        case Instruction_DCP_absolute_y:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            full := address + uint16(cpu.Y)

            value := cpu.LoadMemory(full) - 1
            cpu.doCmp(value)
            cpu.StoreMemory(full, value)
            cpu.Cycle += 7

            cpu.PC += instruction.Length()
            return nil
        case Instruction_DCP_absolute:
            address, err := instruction.OperandWord()
            if err != nil {
                return err
            }

            value := cpu.LoadMemory(address) - 1
            cpu.doCmp(value)
            cpu.StoreMemory(address, value)
            cpu.Cycle += 6

            cpu.PC += instruction.Length()
            return nil
        case Instruction_DCP_zero_x:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := uint16(zero + cpu.X)
            value := cpu.LoadMemory(address) - 1
            cpu.doCmp(value)
            cpu.StoreMemory(address, value)
            cpu.Cycle += 6

            cpu.PC += instruction.Length()
            return nil
        case Instruction_DCP_zero:
            zero, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := uint16(zero)
            value := cpu.LoadMemory(address) - 1
            cpu.doCmp(value)
            cpu.StoreMemory(address, value)
            cpu.Cycle += 5

            cpu.PC += instruction.Length()
            return nil
        case Instruction_DCP_indirect_y:
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address, page_cross := cpu.ComputeIndirectY(relative)
            _ = page_cross
            value := cpu.LoadMemory(address) - 1
            cpu.doCmp(value)
            cpu.StoreMemory(address, value)
            cpu.Cycle += 8

            cpu.PC += instruction.Length()
            return nil
        case Instruction_DCP_indirect_x:
            /* illegal opcode: (addr) = (addr) - 1, compare(A, (addr)) */
            relative, err := instruction.OperandByte()
            if err != nil {
                return err
            }

            address := cpu.ComputeIndirectX(relative)
            value := cpu.LoadMemory(address) - 1
            cpu.doCmp(value)
            cpu.StoreMemory(address, value)
            cpu.Cycle += 8

            cpu.PC += instruction.Length()
            return nil
        case Instruction_CLC:
            cpu.SetCarryFlag(false)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_SEC:
            cpu.SetCarryFlag(true)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_SED:
            cpu.SetDecimalFlag(true)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_CLD:
            cpu.SetDecimalFlag(false)
            cpu.PC += instruction.Length()
            cpu.Cycle += 2
            return nil
        case Instruction_BRK:
            cpu.BRK()
            cpu.Cycle += 7
            return nil
    }

    return fmt.Errorf("unable to execute instruction 0x%x: %v at PC 0x%x", instruction.Kind, instruction.String(), cpu.PC)
}

func (cpu *CPUState) IsIRQAsserted() bool {
    return cpu.APU.IsIRQAsserted() || (cpu.Mapper != nil && cpu.Mapper.IsIRQAsserted())
}

func (cpu *CPUState) Reset() {
    /* https://en.wikipedia.org/wiki/Interrupts_in_65xx_processors
     *
     * http://users.telenet.be/kim1-6502/6502/proman.html#90
     * Cycles   Address Bus   Data Bus    External Operation     Internal Operation
     *
     * 1           ?           ?        Don't Care             Hold During Reset
     * 2         ? + 1         ?        Don't Care             First Start State
     * 3        0100 + SP      ?        Don't Care             Second Start State
     * 4        0100 + SP-1    ?        Don't Care             Third Start State
     * 5        0100 + SP-2    ?        Don't Care             Fourth Start State
     * 6        FFFC        Start PCL   Fetch First Vector
     * 7        FFFD        Start PCH   Fetch Second Vector    Hold PCL
     * 8        PCH PCL     First       Load First OP CODE
     *                      OP CODE
     */
    cpu.Cycle += 6
    low := uint16(cpu.LoadMemory(ResetVector))
    high := uint16(cpu.LoadMemory(ResetVector+1))
    // cpu.PC = (uint16(cpu.LoadMemory(0xfffd)) << 8) | uint16(cpu.LoadMemory(0xfffc))
    cpu.PC = (high<<8) | low
    cpu.SetInterruptDisableFlag(true)
}

func (cpu *CPUState) BRK() {
    cpu.PushStack(byte(cpu.PC >> 8))
    cpu.PushStack(byte(cpu.PC) & 0xff)
    cpu.PushStack(cpu.Status)

    low := uint16(cpu.LoadMemory(BRKVector))
    high := uint16(cpu.LoadMemory(BRKVector+1))
    cpu.PC = (high<<8) | low
    cpu.SetInterruptDisableFlag(true)
    cpu.Cycle += 7
}

func (cpu *CPUState) Interrupt() {
    cpu.PushStack(byte(cpu.PC >> 8))
    cpu.PushStack(byte(cpu.PC) & 0xff)
    cpu.PushStack(cpu.Status)

    /* FIXME: im reasonably sure we should disable the
     * interrupt flag, but recheck if this is the correct logic at some point.
     * The current interrupt flag (which must have been true)
     * will be stored on the stack in cpu.Status
     * RTI will restore the status flag
     */
    cpu.SetInterruptDisableFlag(true)

    low := uint16(cpu.LoadMemory(IRQVector))
    high := uint16(cpu.LoadMemory(IRQVector+1))
    cpu.PC = (high<<8) | low
    cpu.Cycle += 7

    // log.Printf("cpu: interrupt at 0x%x", cpu.PC)
}

/* NMI was set, so jump to the NMI routine */
func (cpu *CPUState) NMI() {
    cpu.PushStack(byte(cpu.PC >> 8))
    cpu.PushStack(byte(cpu.PC) & 0xff)
    cpu.PushStack(cpu.Status)

    low := uint16(cpu.LoadMemory(NMIVector))
    high := uint16(cpu.LoadMemory(NMIVector+1))
    cpu.PC = (high<<8) | low
    cpu.SetInterruptDisableFlag(true)
    cpu.Cycle += 7
}

/* http://wiki.nesdev.com/w/index.php/CPU_power_up_state */
func StartupState() CPUState {
    cpu := CPUState {
        A: 0,
        X: 0,
        Y: 0,
        SP: 0xfd,
        PC: ResetVector,
        Cycle: 0,
        Status: 0x34, // 110100
        Maps: make([][]byte, 256),
    }

    /* http://wiki.nesdev.com/w/index.php/CPU_memory_map */
    memory := NewMemory(0x800)
    cpu.Ram = memory
    cpu.MapMemory(0x0, memory)
    cpu.MapMemory(0x800, memory)
    cpu.MapMemory(0x1000, memory)
    cpu.MapMemory(0x1800, memory)
    cpu.SetStack(0x100)

    cpu.PPU = MakePPU()
    cpu.APU = MakeAPU()

    return cpu
}

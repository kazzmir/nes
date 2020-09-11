package lib

import (
    "fmt"
    "log"
)

/* Mappers work by replacing a range of memory addressable by the cpu.
 * For example, the program memory normally used to hold instructions
 * lives between 0x8000 and 0xffff, which is 32k of memory. If a game
 * wants to use 64kb worth of instructions then some or all of the range
 * 0x8000-0xffff must be remapped to a different part of the 64kb.
 *
 *  0x8000-0xffff -> [first 32k block]
 *       ---      -> [second 32k block]
 *
 * This diagram shows that the address range is mapped to the first block, and
 * no address range is mapped to the second block. Each mapper allows writes
 * to specific address ranges (usually 0x8000 - 0xffff) to allow 'bank switching'
 * which changes the mapping of the address ranges. In the above diagram, the
 * first 32k block is 'page 0' and the second 32k block is 'page 1', so to choose
 * the first block the program can write the value 0 to a mapper register
 *
 *  lda 0
 *  sta #8000
 *
 * To choose the second block, the program can write the value 1 to a mapper register.
 *
 *  lda 1
 *  sta #8000
 *
 * When a new block of memory is mapped in, the cpu mapping then changes. Suppose
 * page 1 is mapped in, then the above diagram would change to.
 *
 *       ---      -> [first 32k block]
 *  0x8000-0xffff -> [second 32k block]
 *
 * At which point reads from any address within 0x8000-0xffff would come from the second
 * 32k block rather than the first.
 *
 * Note that the size of the range being mapped in defines the size of a page. A memory
 * range of 0x1000 (4k) means the 0th page is 0x000-0xfff, the 1th page is 0x1000-0x1fff, etc.
 * A memory range of 0x2000 (8k) means the 0th page is 0x0000-0x1fff, and the 1th page
 * is 0x2000-0x3fff. Therefore a page is a relative measurement.
 *
 * Memory being mapped in can come from either the PRGROM or the CHRROM, and the mapper
 * definition will specify which. Generally all instruction memory will be in the PRGROM
 * and sprite information will be in either PRGROM or CHRROM.
 */

type Mapper interface {
    Write(cpu *CPUState, address uint16, value byte) error
    Read(address uint16) byte
    IsIRQAsserted() bool
}

func MakeMapper(mapper uint32, programRom []byte, chrMemory []byte) (Mapper, error) {
    switch mapper {
        case 0: return MakeMapper0(programRom), nil
        case 1: return MakeMapper1(programRom, chrMemory), nil
        case 2: return MakeMapper2(programRom), nil
        case 3: return MakeMapper3(programRom, chrMemory), nil
        case 4: return MakeMapper4(programRom, chrMemory), nil
        case 9: return MakeMapper9(programRom, chrMemory), nil
        default: return nil, fmt.Errorf("Unimplemented mapper %v", mapper)
    }
}

type Mapper0 struct {
    BankMemory []byte
}

func (mapper *Mapper0) Write(cpu *CPUState, address uint16, value byte) error {
    return fmt.Errorf("mapper0 does not support bank switching at address 0x%x: 0x%x", address, value)
}

func (mapper *Mapper0) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper0) Read(address uint16) byte {
    use := address - uint16(0x8000)
    if len(mapper.BankMemory) == 16*1024 {
        use = use % 0x4000
        return mapper.BankMemory[use]
    } else {
        return mapper.BankMemory[use]
    }
}

func MakeMapper0(bankMemory []byte) Mapper {
    return &Mapper0{
        BankMemory: bankMemory,
    }
}

/* http://wiki.nesdev.com/w/index.php/MMC1 */
type Mapper1 struct {
    BankMemory []byte
    CharacterMemory []byte
    last4kBank int
    /* how many bits to left shift the next value */
    shift int
    /* the value to pass to the mmc */
    register uint8

    mirror byte
    prgBankMode byte
    chrBankMode byte
    prgBank byte

    /* FIXME: this might get mapped from the bank memory, not sure */
    PRGRam []byte
}

/* Read at address 'offset' within the 32k bank given by 'bank'.
 * When bank = 0, the starting address is 0x0000
 * When bank = 1, the starting address is 0x8000
 * etc.
 */
func (mapper *Mapper1) ReadBank(pageSize uint16, bank int, offset uint16) byte {
    base := uint32(bank) * uint32(pageSize)

    final := uint32(offset) + base

    if final >= uint32(len(mapper.BankMemory)) {
        log.Printf("Warning: mapper1: cannot read memory at 0x%x maximum is 0x%x", final, len(mapper.BankMemory))
        return 0
    }

    return mapper.BankMemory[final]
}

func (mapper *Mapper1) Read(address uint16) byte {
    if address >= 0x6000 && address < 0x8000 {
        return mapper.PRGRam[address - uint16(0x6000)]
    }

    baseAddress := address - uint16(0x8000)

    switch mapper.prgBankMode {
        case 0, 1:
            return mapper.ReadBank(0x8000, int(mapper.prgBank >> 1), baseAddress)
        case 2:
            if address < 0xc000 {
                return mapper.ReadBank(0x4000, 0, baseAddress)
            }

            return mapper.ReadBank(0x4000, int(mapper.prgBank), address - 0xc000)
        case 3:
            if address < 0xc000 {
                return mapper.ReadBank(0x4000, int(mapper.prgBank), baseAddress)
            }

            return mapper.ReadBank(0x4000, mapper.last4kBank, address - 0xc000)
    }

    return 0
}

func (mapper *Mapper1) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper1) Write(cpu *CPUState, address uint16, value byte) error {
    if address >= 0x6000 && address < 0x8000 {
        mapper.PRGRam[address - uint16(0x6000)] = value
        return nil
    }

    if cpu.Debug > 0 {
        log.Printf("mapper1: Writing bank switching register 0x%x with value 0x%x", address, value)
    }

    /* if bit 7 is set then clear the shift register */
    clear := value >> 7 == 1

    if clear {
        mapper.shift = 0
        mapper.register = 0
    } else {
        /* shift a single bit into the internal register */
        mapper.register = ((value & 0x1) << mapper.shift) | mapper.register
        mapper.shift += 1

        if mapper.shift == 5 {
            if cpu.Debug > 0 {
                log.Printf("mapper1: write internal register 0x%x to 0x%x", mapper.register, address)
            }

            /* control */
            if address >= 0x8000 && address <= 0x9fff {
                /* CPPMM */
                mapper.mirror = mapper.register & 0x3
                mapper.prgBankMode = (mapper.register >> 2) & 0x3
                mapper.chrBankMode = (mapper.register >> 4) & 0x1

                if cpu.Debug > 0 {
                    log.Printf("mapper1: set control to chr=0x%x prg=0x%x mirror=0x%x", mapper.chrBankMode, mapper.prgBankMode, mapper.mirror)
                }

                switch mapper.mirror {
                    case 0:
                        /* FIXME: 1 screen A */
                        log.Printf("FIXME: mapper1 set mirror to 1 screen A")
                    case 1:
                        /* FIXME: 1 screen B */
                        log.Printf("FIXME: mapper1 set mirror to 1 screen B")
                    case 2:
                        // log.Printf("mapper1: set vertical mirror")
                        /* vertical */
                        cpu.PPU.SetHorizontalMirror(false)
                        cpu.PPU.SetVerticalMirror(true)
                    case 3:
                        // log.Printf("mapper1: set horizontal mirror")
                        /* horizontal */
                        cpu.PPU.SetHorizontalMirror(true)
                        cpu.PPU.SetVerticalMirror(false)
                }
            } else if address >= 0xa000 && address <= 0xbfff {
                /* chr bank 0 */
                if mapper.chrBankMode == 1 {
                    base := uint16(mapper.register) * 0x1000
                    /* FIXME: this is needed for games that have chrrom in the nesfile
                     * such as bubble bobble and zelda2, but doesn't seem to work
                     * for ninja gaiden
                     */
                    if len(mapper.CharacterMemory) != 0 {
                        cpu.PPU.CopyCharacterRom(0x0000, mapper.CharacterMemory[base:base + 0x1000])
                    } else {
                        cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[base:base + 0x1000])
                    }
                } else {
                    base := uint16(mapper.register >> 1) * 0x2000
                    if len(mapper.CharacterMemory) != 0 {
                        cpu.PPU.CopyCharacterRom(0x0000, mapper.CharacterMemory[base:base + 0x2000])
                    } else {
                        cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[base:base + 0x2000])
                    }
                }
            } else if address >= 0xc000 && address <= 0xdfff {
                /* chr bank 1 */
                if mapper.chrBankMode == 1 {
                    base := uint32(mapper.register) * 0x1000
                    if len(mapper.CharacterMemory) != 0 {
                        if int(base + 0x1000) < len(mapper.CharacterMemory) {
                            cpu.PPU.CopyCharacterRom(0x1000, mapper.CharacterMemory[base:base + 0x1000])
                        }
                    } else {
                        cpu.PPU.CopyCharacterRom(0x1000, mapper.BankMemory[base:base + 0x1000])
                    }
                } else {
                    /* ignore in 8k mode */
                }
            } else if address >= 0xe000 {
                /* prg bank */
                mapper.prgBank = mapper.register
                if cpu.Debug > 0 {
                    log.Printf("mapper1: set prg bank 0x%x setting 0x%x", mapper.prgBank, mapper.prgBankMode)
                }
            }

            /* after the 5th write reset the internal register and shift */
            mapper.shift = 0
            mapper.register = 0
        }
    }

    return nil
}

func MakeMapper1(bankMemory []byte, chrMemory []byte) Mapper {
    pages := len(bankMemory) / 0x4000
    return &Mapper1{
        BankMemory: bankMemory,
        CharacterMemory: chrMemory,
        mirror: 0,
        prgBankMode: 3,
        chrBankMode: 0,
        PRGRam: make([]byte, 0x8000 - 0x6000),
        last4kBank: pages-1,
    }
}

type Mapper2 struct {
    BankMemory []byte
    lastBankAddress uint32
    bank byte
}

func (mapper *Mapper2) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper2) Read(address uint16) byte {
    if address < 0xc000 {
        offset := uint32(address - 0x8000)
        return mapper.BankMemory[uint32(mapper.bank) * 0x4000 + offset]
    } else {
        offset := uint32(address - 0xc000)
        return mapper.BankMemory[mapper.lastBankAddress + offset]
    }
}

func (mapper *Mapper2) Write(cpu *CPUState, address uint16, value byte) error {
    if cpu.Debug > 0 {
        log.Printf("Accessing bank switching register 0x%x with value 0x%x", address, value)
    }

    mapper.bank = value

    return nil
}

func MakeMapper2(bankMemory []byte) Mapper {
    return &Mapper2{
        BankMemory: bankMemory,
        lastBankAddress: uint32(len(bankMemory) - 0x4000),
    }
}

type Mapper3 struct {
    ProgramRom []byte
    BankMemory []byte
}

func (mapper *Mapper3) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper3) Read(address uint16) byte {
    if address >= 0x8000 {
        offset := address - 0x8000
        return mapper.ProgramRom[offset]
    }

    return 0
}

func (mapper *Mapper3) Write(cpu *CPUState, address uint16, value byte) error {
    use := value & 0x3
    base := uint16(use) * 0x2000
    cpu.PPU.CopyCharacterRom(0x000, mapper.BankMemory[base:base+0x2000])
    return nil
}

func MakeMapper3(programRom []byte, chrMemory []byte) Mapper {
    return &Mapper3{
        ProgramRom: programRom,
        BankMemory: chrMemory,
    }
}

type Mapper4 struct {
    ProgramRom []byte
    CharacterRom []byte
    SaveRam []byte

    lastBank int

    irqEnabled bool
    irqReload byte
    irqCounter byte
    irqPending bool

    chrMode byte
    prgMode byte
    /* used to select which of the chrRegister/prgRegister to write to */
    registerIndex byte

    chrRegister [6]byte
    prgRegister [2]byte
}

func (mapper *Mapper4) IsIRQAsserted() bool {
    return mapper.irqPending
}

func (mapper *Mapper4) ProgramBlock(page byte) ([]byte, error) {
    pageSize := uint32(0x2000)
    base := pageSize * uint32(page)

    if base + pageSize > uint32(len(mapper.ProgramRom)) {
        return nil, fmt.Errorf("mapper4 program cannot get 0x%x-0x%x max is 0x%x", base, base + pageSize, len(mapper.ProgramRom))
    }

    return mapper.ProgramRom[base:base+pageSize], nil
}

func (mapper *Mapper4) ReadBank(offset uint16, bank int) byte {
    base := bank * 0x2000
    return mapper.ProgramRom[uint32(base)+uint32(offset)]
}

func (mapper *Mapper4) Read(address uint16) byte {
    if address >= 0x6000 && address < 0x8000 {
        return mapper.SaveRam[address - 0x6000]
    }

    switch mapper.prgMode {
        case 0:
            if address >= 0x8000 && address < 0xa000 {
                return mapper.ReadBank(address-0x8000, int(mapper.prgRegister[0]))
            } else if address >= 0xa000 && address < 0xc000 {
                return mapper.ReadBank(address-0xa000, int(mapper.prgRegister[1]))
            } else if address >= 0xc000 && address < 0xe000 {
                return mapper.ReadBank(address-0xc000, mapper.lastBank-1)
            } else {
                return mapper.ReadBank(address-0xe000, mapper.lastBank)
            }
        case 1:
            if address >= 0x8000 && address < 0xa000 {
                return mapper.ReadBank(address-0x8000, mapper.lastBank-1)
            } else if address >= 0xa000 && address < 0xc000 {
                return mapper.ReadBank(address-0xa000, int(mapper.prgRegister[1]))
            } else if address >= 0xc000 && address < 0xe000 {
                return mapper.ReadBank(address-0xc000, int(mapper.prgRegister[0]))
            } else {
                return mapper.ReadBank(address-0xe000, mapper.lastBank)
            }
        default:
            log.Printf("mapper4: unknown prg mode %v", mapper.prgMode)
            return 0
    }

    return 0
}

func (mapper *Mapper4) CharacterBlock(length uint32, page byte) []byte {
    /* Kind of weird but I guess a page is always 0x400 (1k) bytes */
    base := uint32(page) * 0x400
    // log.Printf("mapper4: character rom bank page 0x%x at 0x%x - 0x%x", page, base, base + length)
    return mapper.CharacterRom[base:base+length]
}

func (mapper *Mapper4) SetChrBank(ppu *PPUState) error {
    switch mapper.chrMode {
        case 0:
            ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x800, mapper.chrRegister[0]))
            ppu.CopyCharacterRom(0x0800, mapper.CharacterBlock(0x800, mapper.chrRegister[1]))
            ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x400, mapper.chrRegister[2]))
            ppu.CopyCharacterRom(0x1400, mapper.CharacterBlock(0x400, mapper.chrRegister[3]))
            ppu.CopyCharacterRom(0x1800, mapper.CharacterBlock(0x400, mapper.chrRegister[4]))
            ppu.CopyCharacterRom(0x1c00, mapper.CharacterBlock(0x400, mapper.chrRegister[5]))
            return nil
        case 1:
            ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x400, mapper.chrRegister[2]))
            ppu.CopyCharacterRom(0x0400, mapper.CharacterBlock(0x400, mapper.chrRegister[3]))
            ppu.CopyCharacterRom(0x0800, mapper.CharacterBlock(0x400, mapper.chrRegister[4]))
            ppu.CopyCharacterRom(0x0c00, mapper.CharacterBlock(0x400, mapper.chrRegister[5]))
            ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x800, mapper.chrRegister[0]))
            ppu.CopyCharacterRom(0x1800, mapper.CharacterBlock(0x800, mapper.chrRegister[1]))
            return nil
    }

    return fmt.Errorf("mapper4: unknown chr mode %v", mapper.chrMode)
}

func (mapper *Mapper4) Write(cpu *CPUState, address uint16, value byte) error {
    if address >= 0x6000 && address < 0x8000 {
        mapper.SaveRam[address - 0x6000] = value
        return nil
    }

    // log.Printf("mapper4: write to 0x%x value 0x%x", address, value)
    switch address {
        case 0x8000:
            mapper.chrMode = (value >> 7) & 0x1
            mapper.prgMode = (value >> 6) & 0x1
            mapper.registerIndex = value & 0x7
            mapper.SetChrBank(&cpu.PPU)
        case 0x8001:
            switch mapper.registerIndex {
                case 0, 1, 2, 3, 4, 5:
                    if mapper.registerIndex == 0 || mapper.registerIndex == 1 {
                        value = value & (^byte(1))
                    }
                    mapper.chrRegister[mapper.registerIndex] = value
                    mapper.SetChrBank(&cpu.PPU)
                case 6, 7:
                    /* only use the first 6 bits, top two are ignored */
                    mapper.prgRegister[mapper.registerIndex-6] = value & 0x3f
            }
        case 0xa000:
            mirror := value & 0x1
            /* FIXME: dont set mirroring for nes files that set 4-way mirroring */
            switch mirror {
                case 0:
                    cpu.PPU.SetHorizontalMirror(false)
                    cpu.PPU.SetVerticalMirror(true)
                case 1:
                    cpu.PPU.SetHorizontalMirror(true)
                    cpu.PPU.SetVerticalMirror(false)
            }
        case 0xa001:
            /* prg ram protect */
            log.Printf("FIXME: mapper4: implement prg ram protect 0xa001 value 0x%x", value)
            break
        case 0xc000:
            mapper.irqReload = value
        case 0xc001:
            mapper.irqCounter = value
        case 0xe000:
            mapper.irqEnabled = false
            mapper.irqPending = false
        case 0xe001:
            mapper.irqEnabled = true
        default:
            log.Printf("FIXME: unknown mapper4 write to 0x%x with 0x%x", address, value)
    }

    return nil
}

func (mapper *Mapper4) Scanline() {
    if mapper.irqCounter == 0 {
        mapper.irqCounter = mapper.irqReload
    } else {
        mapper.irqCounter -= 1
        if mapper.irqCounter == 0 && mapper.irqEnabled {
            mapper.irqPending = true
        }
    }
}

func MakeMapper4(programRom []byte, chrMemory []byte) Mapper {
    pageSize := uint16(0x2000)
    pages := len(programRom) / int(pageSize)

    return &Mapper4{
        ProgramRom: programRom,
        CharacterRom: chrMemory,
        SaveRam: make([]byte, 0x2000),
        lastBank: pages-1,
    }
}

type Mapper9 struct {
    ProgramRom []byte
    CharacterRom []byte
    Pages int

    prgRegister byte
    chrRegister [4]byte
}

func (mapper *Mapper9) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper9) ReadBank(offset uint16, bank int) byte {
    base := bank * 0x2000
    return mapper.ProgramRom[uint32(base)+uint32(offset)]
}

func (mapper *Mapper9) Read(address uint16) byte {
    if address >= 0x8000 && address < 0xa000 {
        return mapper.ReadBank(address - 0x8000, int(mapper.prgRegister))
    } else if address >= 0xa000 && address < 0xc000 {
        return mapper.ReadBank(address - 0xa000, mapper.Pages-3)
    } else if address >= 0xc000 && address < 0xe000 {
        return mapper.ReadBank(address - 0xc000, mapper.Pages-2)
    } else {
        return mapper.ReadBank(address - 0xe000, mapper.Pages-1)
    }

    return 0
}

func (mapper *Mapper9) CharacterBlock(pageSize uint16, register byte) []byte {
    base := uint32(register) * uint32(pageSize)
    return mapper.CharacterRom[base:base+uint32(pageSize)]
}

func (mapper *Mapper9) SetChrBank(ppu *PPUState) {
    /* FIXME: Use chrRegister 0 or 2 depending on the ppu latch set at $fd or $fe */
    ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x1000, mapper.chrRegister[1]))
    /* FIXME: Use chrRegister 1 or 3 depending on the ppu latch set at $fd or $fe */
    ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x1000, mapper.chrRegister[2]))
}

func (mapper *Mapper9) Write(cpu *CPUState, address uint16, value byte) error {
    page := address >> 12
    switch page {
        case 0xa:
            mapper.prgRegister = value
            return nil
        case 0xb:
            mapper.chrRegister[0] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xc:
            mapper.chrRegister[1] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xd:
            mapper.chrRegister[2] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xe:
            mapper.chrRegister[3] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xf:
            mirror := value & 0x1
            switch mirror {
                case 0:
                    cpu.PPU.SetHorizontalMirror(false)
                    cpu.PPU.SetVerticalMirror(true)
                case 1:
                    cpu.PPU.SetHorizontalMirror(true)
                    cpu.PPU.SetVerticalMirror(false)
            }

            return nil
    }

    return fmt.Errorf("mapper9: unknown write to address 0x%x", address)
}

func MakeMapper9(programRom []byte, chrMemory []byte) Mapper {
    pageSize := uint16(0x2000)
    pages := len(programRom) / int(pageSize)
    return &Mapper9{
        ProgramRom: programRom,
        CharacterRom: chrMemory,
        Pages: pages,
    }
}

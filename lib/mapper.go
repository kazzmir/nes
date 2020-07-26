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
    Initialize(cpu *CPUState) error
}

func MakeMapper(mapper uint32, programRom []byte, chrMemory []byte) (Mapper, error) {
    switch mapper {
        case 0: return MakeMapper0(programRom), nil
        case 1: return MakeMapper1(programRom), nil
        case 2: return MakeMapper2(programRom), nil
        case 3: return MakeMapper3(programRom, chrMemory), nil
        case 4: return MakeMapper4(programRom, chrMemory), nil
        default: return nil, fmt.Errorf("Unimplemented mapper %v", mapper)
    }
}

type Mapper0 struct {
    BankMemory []byte
}

func (mapper *Mapper0) Write(cpu *CPUState, address uint16, value byte) error {
    return fmt.Errorf("mapper0 does not support bank switching at address 0x%x: 0x%x", address, value)
}

func (mapper *Mapper0) Initialize(cpu *CPUState) error {
    /* map code to 0xc000 for NROM-128.
     * also map to 0x8000, but most games don't seem to care..?
     * http://wiki.nesdev.com/w/index.php/Programming_NROM
     */
    err := cpu.MapMemory(0x8000, mapper.BankMemory)
    if err != nil {
        return err
    }

    /* FIXME: handle this by checking if the nes file uses nrom-256 */
    /* for a 32k rom, dont map the programrom at 0xc000 */
    if len(mapper.BankMemory) == 16*1024 {
        err = cpu.MapMemory(0xc000, mapper.BankMemory)
        if err != nil {
            return err
        }
    }

    return nil
}

func MakeMapper0(bankMemory []byte) Mapper {
    return &Mapper0{
        BankMemory: bankMemory,
    }
}

/* http://wiki.nesdev.com/w/index.php/MMC1 */
type Mapper1 struct {
    BankMemory []byte
    /* how many bits to left shift the next value */
    shift int
    /* the value to pass to the mmc */
    register uint8

    mirror byte
    prgBankMode byte
    chrBankMode byte

    /* FIXME: this might get mapped from the bank memory, not sure */
    PRGRam []byte
}

func (mapper *Mapper1) Write(cpu *CPUState, address uint16, value byte) error {
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
            /*
            err := mapper.BankSwitch(cpu, int(value))
            if err != nil {
                return fmt.Errorf("Warning: could not bank switch to 0x%x: %v\n", value, err)
            }
            */

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
                        break
                    case 1:
                        /* FIXME: 1 screen B */
                        log.Printf("FIXME: mapper1 set mirror to 1 screen B")
                        break
                    case 2:
                        /* vertical */
                        cpu.PPU.SetHorizontalMirror(false)
                        cpu.PPU.SetVerticalMirror(true)
                    case 3:
                        /* horizontal */
                        cpu.PPU.SetHorizontalMirror(true)
                        cpu.PPU.SetVerticalMirror(false)
                }

                /* FIXME: I think passing in register here might be incorrect */
                err := mapper.SetPrgBank(cpu, mapper.register, int(mapper.prgBankMode))
                if err != nil {
                    return err
                }
            } else if address >= 0xa000 && address <= 0xbfff {
                /* chr bank 0 */
                if mapper.chrBankMode == 1 {
                    base := uint16(mapper.register) * 0x1000
                    cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[base:base + 0x1000])
                } else {
                    base := uint16(mapper.register >> 1) * 0x2000
                    cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[base:base + 0x2000])
                }
            } else if address >= 0xc000 && address <= 0xdfff {
                /* chr bank 1 */
                if mapper.chrBankMode == 1 {
                    base := uint16(mapper.register) * 0x1000
                    cpu.PPU.CopyCharacterRom(0x1000, mapper.BankMemory[base:base + 0x1000])
                } else {
                    /* ignore in 8k mode */
                }
            } else if address >= 0xe000 {
                /* prg bank */
                err := mapper.SetPrgBank(cpu, mapper.register, int(mapper.prgBankMode))
                if err != nil {
                    return err
                }
            }

            /* after the 5th write reset the internal register and shift */
            mapper.shift = 0
            mapper.register = 0
        }
    }

    return nil
}

func (mapper *Mapper1) SetPrgBank(cpu *CPUState, bank uint8, setting int) error {
    if cpu.Debug > 0 {
        log.Printf("mapper1: set prg bank 0x%x setting 0x%x", bank, setting)
    }
    err := cpu.UnmapMemory(0x8000, 32 * 1024)
    if err != nil {
        return err
    }
    switch setting {
        case 0, 1:
            page := bank >> 1
            base := uint32(page) * 0x8000
            if base + 0x10000 > uint32(len(mapper.BankMemory)) {
                return fmt.Errorf("Cannot bank switch more than available PRG: tried to swap in 0x%x-0x%x but maximum is 0x%x", base, base + 0x10000, len(mapper.BankMemory))
            }

            if cpu.Debug > 0 {
                log.Printf("mapper1: set 0x8000 to 32kb starting at bank 0x%x", page)
            }
            err := cpu.MapMemory(0x8000, mapper.BankMemory[base:base + 0x10000])
            if err != nil {
                return err
            }
        case 2:
            base := uint32(0)
            cpu.MapMemory(0x8000, mapper.BankMemory[base:base + 0x4000])

            base = uint32(bank) * 0x4000
            err := cpu.MapMemory(0xc000, mapper.BankMemory[base:base + 0x4000])
            if err != nil {
                return err
            }
        case 3:
            base := uint32(bank) * 0x4000
            if cpu.Debug > 0 {
                log.Printf("mapper1: map 0x8000 -> 0x%x:0x%x", base, base + 0x4000)
            }
            if base + 0x4000 > uint32(len(mapper.BankMemory)) {
                return fmt.Errorf("Could not bank switch 0x%x-0x%x. Maximum is 0x%x", base, base + 0x4000, len(mapper.BankMemory))
            }
            err := cpu.MapMemory(0x8000, mapper.BankMemory[base:base + 0x4000])
            if err != nil {
                return err
            }
            /* FIXME: Disch's mapper 002.txt says to map bank {$0f} to 0xc000
             * but if a bank is 16k (0x4000) then the 0xf'th bank would be
             * 0x3c000, which is past the length of the prgrom for .nes files
             * that use mapper 2 such as zelda with 0x20000. the nesdev wiki
             * says to map 'the last bank' rather than explicitly mentioning $0f
             */
            base = uint32(len(mapper.BankMemory)) - 0x4000
            err = cpu.MapMemory(0xc000, mapper.BankMemory[base:base + 0x4000])
            if err != nil {
                return err
            }
    }

    return nil
}

func (mapper *Mapper1) Initialize(cpu *CPUState) error {
    err := cpu.MapMemory(0x6000, mapper.PRGRam)
    if err != nil {
        return err
    }

    /* FIXME: what is the default address on startup? */
    return mapper.SetPrgBank(cpu, 0, 3)
}

func (mapper *Mapper1) BankSwitch(cpu *CPUState, bank int) error {
    return fmt.Errorf("mapper1 bankswitch unimplemented")
}

func MakeMapper1(bankMemory []byte) Mapper {
    return &Mapper1{
        BankMemory: bankMemory,
        mirror: 0,
        prgBankMode: 3,
        chrBankMode: 0,
        PRGRam: make([]byte, 0x8000 - 0x6000),
    }
}

type Mapper2 struct {
    BankMemory []byte
}

func (mapper *Mapper2) Initialize(cpu *CPUState) error {
    if len(mapper.BankMemory) < 16 * 1024 {
        return fmt.Errorf("Expected mapper 2 nes file to have at least 16kb of program rom but the given file had %v bytes\n", len(mapper.BankMemory))
    }

    err := cpu.MapMemory(0x8000, mapper.BankMemory[0:8 * 1024])
    if err != nil {
        return err
    }

    length := len(mapper.BankMemory)
    err = cpu.MapMemory(0xc000, mapper.BankMemory[length-16*1024:length])
    if err != nil {
        return err
    }

    return nil
}

func (mapper *Mapper2) BankSwitch(cpu *CPUState, bank int) error {
    err := cpu.UnmapMemory(0x8000, 16 * 1024)
    if err != nil {
        return err
    }

    /* map a new 16k block int */
    base := bank * 16 * 1024
    return cpu.MapMemory(0x8000, mapper.BankMemory[base:base + 16 * 1024])
}

func (mapper *Mapper2) Write(cpu *CPUState, address uint16, value byte) error {
    if cpu.Debug > 0 {
        log.Printf("Accessing bank switching register 0x%x with value 0x%x", address, value)
    }
    err := mapper.BankSwitch(cpu, int(value))
    if err != nil {
        return fmt.Errorf("Warning: could not bank switch to 0x%x: %v\n", value, err)
    }

    return nil
}

func MakeMapper2(bankMemory []byte) Mapper {
    return &Mapper2{
        BankMemory: bankMemory,
    }
}

type Mapper3 struct {
    ProgramRom []byte
    BankMemory []byte
}

func (mapper *Mapper3) Initialize(cpu *CPUState) error {
    err := cpu.MapMemory(0x8000, mapper.ProgramRom)
    if err != nil {
        return err
    }
    cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[0:0x2000])
    return nil
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

    chrMode byte
    prgMode byte
    /* used to select which of the chrRegister/prgRegister to write to */
    registerIndex byte

    chrRegister [6]byte
    prgRegister [2]byte
}

func (mapper *Mapper4) Initialize(cpu *CPUState) error {
    return mapper.SetPrgBank(cpu)
}

func (mapper *Mapper4) ProgramBlock(page byte) []byte {
    pageSize := uint32(0x2000)
    base := pageSize * uint32(page)

    return mapper.ProgramRom[base:base+pageSize]
}

func (mapper *Mapper4) SetPrgBank(cpu *CPUState) error {
    pageSize := uint16(0x2000)
    pages := len(mapper.ProgramRom) / int(pageSize)
    lastPage := byte(pages - 1)

    /* unmap everything */
    err := cpu.UnmapMemory(0x8000, 0x10000 - 0x8000)
    if err != nil {
        return err
    }

    var order []byte

    /* map every 8k section of memory */
    switch mapper.prgMode {
        case 0:
            order = []byte{mapper.prgRegister[0], mapper.prgRegister[1], lastPage-1, lastPage}
        case 1:
            order = []byte{lastPage-1, mapper.prgRegister[1], mapper.prgRegister[0], lastPage}
        default:
            return fmt.Errorf("mapper4: unknown prg mode %v", mapper.prgMode)
    }

    for i, page := range order {
        err = cpu.MapMemory(0x8000 + uint16(i) * pageSize, mapper.ProgramBlock(page))
        if err != nil {
            return err
        }
    }

    return nil
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
    // log.Printf("mapper4: write to 0x%x value 0x%x", address, value)
    switch address {
        case 0x8000:
            mapper.chrMode = (value >> 7) & 0x1
            mapper.prgMode = (value >> 6) & 0x1
            mapper.registerIndex = value & 0x7
            mapper.SetChrBank(&cpu.PPU)
            mapper.SetPrgBank(cpu)
        case 0x8001:
            updateChr := false
            updatePrg := false
            switch mapper.registerIndex {
                case 0, 1, 2, 3, 4, 5:
                    if mapper.registerIndex == 0 || mapper.registerIndex == 1 {
                        value = value & (^byte(1))
                    }
                    mapper.chrRegister[mapper.registerIndex] = value
                    updateChr = true
                case 6, 7:
                    /* only use the first 6 bits, top two are ignored */
                    mapper.prgRegister[mapper.registerIndex-6] = value & 0x3f
                    updatePrg = true
            }

            if updateChr {
                mapper.SetChrBank(&cpu.PPU)
            }

            if updatePrg {
                mapper.SetPrgBank(cpu)
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
            log.Printf("FIXME: mapper4: implement prg ram protect 0xa001")
            break
        case 0xc000:
            log.Printf("FIXME: mapper4: implement irq reload 0xc000")
        case 0xc001:
            log.Printf("FIXME: mapper4: implement irq set to 0 0xc001")
        case 0xe000:
            log.Printf("FIXME: mapper4: implement clear irq enable and pending 0xe000")
        case 0xe001:
            log.Printf("FIXME: mapper4: implement set irq enable flag 0xe001")
        default:
            log.Printf("FIXME: unknown mapper4 write to 0x%x with 0x%x", address, value)
    }

    return nil
}

func MakeMapper4(programRom []byte, chrMemory []byte) Mapper {
    return &Mapper4{
        ProgramRom: programRom,
        CharacterRom: chrMemory,
    }
}

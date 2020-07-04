package lib

import (
    "fmt"
    "log"
)

type Mapper interface {
    Write(cpu *CPUState, address uint16, value byte) error
    Initialize(cpu *CPUState) error
}

func MakeMapper(mapper uint32, bankMemory []byte) (Mapper, error) {
    switch mapper {
        case 0: return MakeMapper0(bankMemory), nil
        case 1: return MakeMapper1(bankMemory), nil
        case 2: return MakeMapper2(bankMemory), nil
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

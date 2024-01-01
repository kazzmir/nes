package lib

import (
    "fmt"
    "log"
    "encoding/json"
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
    // Write a value to the mapper address space
    Write(cpu *CPUState, address uint16, value byte) error
    // Read a value from the mapper address space
    Read(address uint16) byte
    // True if the irq line is asserted
    IsIRQAsserted() bool
    // Make a deep copy of this object
    Copy() Mapper
    // Return an int that identifies this type of mapper
    Kind() int
    // Return an error that describes any difference to another mapper, or nil if
    // there are no differences
    Compare(Mapper) error
}

type MapperState struct {
    Kind int `json:"kind"`
    Mapper Mapper `json:"mapper"`
}

func (state *MapperState) Set(mapper Mapper){
    state.Mapper = mapper
    state.Kind = mapper.Kind()
}

func (state *MapperState) Compare(other MapperState) error {
    if state.Kind != other.Kind {
        return fmt.Errorf("mapper kind differs me=%v other=%v", state.Kind, other.Kind)
    }

    if (state.Mapper == nil && other.Mapper != nil) ||
       (state.Mapper != nil && other.Mapper == nil){
        return fmt.Errorf("one mapper was nil and the other wasn't. me=%v other=%v", state.Mapper, other.Mapper)
    }

    if state.Mapper == nil && other.Mapper == nil {
        return nil
    }

    return state.Mapper.Compare(other.Mapper)
}

func (state *MapperState) Copy() MapperState {
    return MapperState{
        Kind: state.Kind,
        Mapper: state.Mapper.Copy(),
    }
}

type JustKind struct {
    Kind int `json:"kind"`
}

func unmarshalMapper[T Mapper](data []byte) (T, error) {
    var mapper struct {
        Kind int `json:"kind"`
        Mapper T `json:"mapper"`
    }
    err := json.Unmarshal(data, &mapper)
    if err != nil {
        return mapper.Mapper, err
    }
    return mapper.Mapper, nil
}

var _ json.Unmarshaler = &MapperState{}

func (state *MapperState) UnmarshalJSON(data []byte) error {
    var kind JustKind
    err := json.Unmarshal(data, &kind)
    if err != nil {
        return err
    }
    state.Kind = kind.Kind
    switch state.Kind {
        case 0:
            mapper0, err := unmarshalMapper[*Mapper0](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper0
            return nil
            /*
            var mapper struct {
                Kind int `json:"kind"`
                Mapper0 Mapper0 `json:"mapper"`
            }
            err = json.Unmarshal(data, &mapper)
            if err != nil {
                return err
            }
            state.Mapper = &mapper.Mapper0
            */
        case 1:
            mapper1, err := unmarshalMapper[*Mapper1](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper1
            return nil
        case 2:
            mapper2, err := unmarshalMapper[*Mapper2](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper2
            return nil
        case 3:
            mapper3, err := unmarshalMapper[*Mapper3](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper3
            return nil
        case 4:
            mapper4, err := unmarshalMapper[*Mapper4](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper4
            return nil
        case 7:
            mapper7, err := unmarshalMapper[*Mapper7](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper7
            return nil
        case 9:
            mapper9, err := unmarshalMapper[*Mapper9](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper9
            return nil
    }

    return fmt.Errorf("could not deserialize mapper. unknown mapper type %v", state.Kind)
}

func compareSlice[T comparable](slice1 []T, slice2 []T) error {
    if len(slice1) != len(slice2) {
        return fmt.Errorf("slices differ in size slice1=%v slice2=%v", len(slice1), len(slice2))
    }

    for i := 0; i < len(slice1); i++ {
        if slice1[i] != slice2[i] {
            return fmt.Errorf("slices differ at index %v: slice1=%v slice2=%v", i, slice1[i], slice2[i])
        }
    }

    return nil
}

func MakeMapper(mapper uint32, programRom []byte, chrMemory []byte) (Mapper, error) {
    switch mapper {
        case 0: return MakeMapper0(programRom), nil
        case 1: return MakeMapper1(programRom, chrMemory), nil
        case 2: return MakeMapper2(programRom), nil
        case 3: return MakeMapper3(programRom, chrMemory), nil
        case 4: return MakeMapper4(programRom, chrMemory), nil
        case 7: return MakeMapper7(programRom, chrMemory), nil
        case 9: return MakeMapper9(programRom, chrMemory), nil
        default: return nil, fmt.Errorf("Unimplemented mapper %v", mapper)
    }
}

type Mapper0 struct {
    BankMemory []byte `json:"bank"`
}

func (mapper *Mapper0) Compare(other Mapper) error {
    him, ok := other.(*Mapper0)
    if !ok {
        return fmt.Errorf("other was not a mapper0")
    }

    return compareSlice(mapper.BankMemory, him.BankMemory)
}

/*
func copySlice[T any](x []T) []T {
    return append(nil, x...)
}
*/

func copySlice[T any](x []T) []T {
    var f []T
    return append(f, x...)
}

/* these copy functions can probably be made generic somehow */
func copySlice6[T any](x [6]T) [6]T {
    var y [6]T
    copy(y[:], x[:])
    return y
}

func copySlice4[T any](x [4]T) [4]T {
    var y [4]T
    copy(y[:], x[:])
    return y
}

func copySlice2[T any](x [2]T) [2]T {
    var y [2]T
    copy(y[:], x[:])
    return y
}

func (mapper *Mapper0) Copy() Mapper {
    return &Mapper0{
        BankMemory: copySlice(mapper.BankMemory),
    }
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

func (mapper *Mapper0) Kind() int {
    return 0
}

func MakeMapper0(bankMemory []byte) Mapper {
    return &Mapper0{
        BankMemory: bankMemory,
    }
}

/* http://wiki.nesdev.org/w/index.php/MMC1 */
type Mapper1 struct {
    BankMemory []byte `json:"bankmemory"`
    CharacterMemory []byte `json:"charmemory"`
    Last4kBank int `json:"last4kbank"`
    /* how many bits to left shift the next value */
    Shift int `json:"shift"`
    /* the value to pass to the mmc */
    Register uint8 `json:"register"`

    Mirror byte `json:"mirror"`
    PrgBankMode byte `json:"prgbankmode"`
    ChrBankMode byte `json:"chrbankmode"`
    PrgBank byte `json:"prgbank"`

    /* FIXME: this might get mapped from the bank memory, not sure */
    PRGRam []byte `json:"prgram"`
}

func (mapper *Mapper1) Kind() int {
    return 1
}

func (mapper *Mapper1) Compare(other Mapper) error {
    return fmt.Errorf("mapper1 compare unimplemented")
}

/* Read at address 'offset' within the 32k bank given by 'bank'.
 * When bank = 0, the starting address is 0x0000
 * When bank = 1, the starting address is 0x8000
 * etc.
 */
func (mapper *Mapper1) ReadBank(pageSize uint16, bank int, offset uint16) byte {
    base := uint32(bank) * uint32(pageSize)

    final := uint32(offset) + base

    /*
    if final >= uint32(len(mapper.BankMemory)) {
        log.Printf("Warning: mapper1: cannot read memory at 0x%x maximum is 0x%x", final, len(mapper.BankMemory))
        return 0
    }
    */
    final = final % uint32(len(mapper.BankMemory))

    return mapper.BankMemory[final]
}

func (mapper *Mapper1) Copy() Mapper {
    return &Mapper1{
        BankMemory: copySlice(mapper.BankMemory),
        CharacterMemory: copySlice(mapper.CharacterMemory),
        Last4kBank: mapper.Last4kBank,
        Shift: mapper.Shift,
        Register: mapper.Register,
        Mirror: mapper.Mirror,
        PrgBankMode: mapper.PrgBankMode,
        ChrBankMode: mapper.ChrBankMode,
        PrgBank: mapper.PrgBank,
        PRGRam: copySlice(mapper.PRGRam),
    }
}

func (mapper *Mapper1) Read(address uint16) byte {
    if address >= 0x6000 && address < 0x8000 {
        return mapper.PRGRam[address - uint16(0x6000)]
    }

    baseAddress := address - uint16(0x8000)

    const pageSize32k = 0x8000
    const pageSize16k = 0x4000

    switch mapper.PrgBankMode {
        /* P=0, read in 32k mode */
        case 0, 1:
            return mapper.ReadBank(pageSize32k, int(mapper.PrgBank >> 1), baseAddress)
        /* P=1, S=0, read in 16k mode where 0x8000 is mapped to 0, and 0xc000 is mapped to the program bank */
        case 2:
            if address < 0xc000 {
                return mapper.ReadBank(pageSize16k, 0, baseAddress)
            }

            return mapper.ReadBank(pageSize16k, int(mapper.PrgBank), address - 0xc000)
        /* P=1, S=1, read in 16k mode where 0x8000 is mapped to the program bank, and 0xc000 is mapped to page 0xf */
        case 3:
            if address < 0xc000 {
                return mapper.ReadBank(pageSize16k, int(mapper.PrgBank), baseAddress)
            }

            /* The MMC1 documentation says to map 0xc000-0xffff to page 0xf, but for blaster master
             * the last valid page is 8, so we use the last valid page instead
             */
            return mapper.ReadBank(pageSize16k, 0xf, address - 0xc000)
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
        mapper.Shift = 0
        mapper.Register = 0
    } else {
        /* shift a single bit into the internal register */
        mapper.Register = ((value & 0x1) << mapper.Shift) | mapper.Register
        mapper.Shift += 1

        if mapper.Shift == 5 {
            if cpu.Debug > 0 {
                log.Printf("mapper1: write internal register 0x%x to 0x%x", mapper.Register, address)
            }

            /* control */
            if address >= 0x8000 && address <= 0x9fff {
                /* CPPMM */
                mapper.Mirror = mapper.Register & 0x3
                mapper.PrgBankMode = (mapper.Register >> 2) & 0x3
                mapper.ChrBankMode = (mapper.Register >> 4) & 0x1

                if cpu.Debug > 0 {
                    log.Printf("mapper1: set control to chr=0x%x prg=0x%x mirror=0x%x", mapper.ChrBankMode, mapper.PrgBankMode, mapper.Mirror)
                }

                switch mapper.Mirror {
                    case 0:
                        cpu.PPU.SetScreenAMirror()
                    case 1:
                        cpu.PPU.SetScreenBMirror()
                    case 2:
                        cpu.PPU.SetVerticalMirror()
                    case 3:
                        cpu.PPU.SetHorizontalMirror()
                }
            } else if address >= 0xa000 && address <= 0xbfff {
                /* chr bank 0 */
                if mapper.ChrBankMode == 1 {
                    /* FIXME: base could be 0xf000, so base + 0x1000 could be 0
                     * making base an int prevents this wraparound, but im not 100% sure if its the right thing to do
                     */
                    base := int(uint16(mapper.Register) * 0x1000)
                    /* FIXME: this is needed for games that have chrrom in the nesfile
                     * such as bubble bobble and zelda2, but doesn't seem to work
                     * for ninja gaiden
                     */
                    if len(mapper.CharacterMemory) != 0 {
                        if int(base) < len(mapper.CharacterMemory) && int(base + 0x1000) < len(mapper.CharacterMemory) && base < base + 0x1000 {
                            cpu.PPU.CopyCharacterRom(0x0000, mapper.CharacterMemory[base:base + 0x1000])
                        } else {
                            log.Printf("[mapper1] Warning: could not copy character data from range %v to %v", base, base + 0x1000)
                        }
                    } else {
                        cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[base:base + 0x1000])
                    }
                } else {
                    base := uint16(mapper.Register >> 1) * 0x2000
                    if len(mapper.CharacterMemory) != 0 {
                        cpu.PPU.CopyCharacterRom(0x0000, mapper.CharacterMemory[base:base + 0x2000])
                    } else {
                        cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[base:base + 0x2000])
                    }
                }
            } else if address >= 0xc000 && address <= 0xdfff {
                /* chr bank 1 */
                if mapper.ChrBankMode == 1 {
                    base := uint32(mapper.Register) * 0x1000
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
                mapper.PrgBank = mapper.Register
                if cpu.Debug > 0 {
                    log.Printf("mapper1: set prg bank 0x%x setting 0x%x", mapper.PrgBank, mapper.PrgBankMode)
                }
            }

            /* after the 5th write reset the internal register and shift */
            mapper.Shift = 0
            mapper.Register = 0
        }
    }

    return nil
}

func MakeMapper1(bankMemory []byte, chrMemory []byte) Mapper {
    pages := len(bankMemory) / 0x4000
    return &Mapper1{
        BankMemory: bankMemory,
        CharacterMemory: chrMemory,
        Mirror: 0,
        PrgBankMode: 3,
        ChrBankMode: 0,
        PRGRam: make([]byte, 0x8000 - 0x6000),
        Last4kBank: pages-1,
    }
}

type Mapper2 struct {
    BankMemory []byte `json:"bankmemory"`
    SaveRam []byte `json:"saveram"`
    LastBankAddress uint32 `json:"lastaddress"`
    Bank byte `json:"bank"`
}

func (mapper *Mapper2) Compare(other Mapper) error {
    him, ok := other.(*Mapper2)
    if !ok {
        return fmt.Errorf("other was not a mapper2")
    }

    if mapper.Bank != him.Bank {
        return fmt.Errorf("bank differs: me=%v him=%v", mapper.Bank, him.Bank)
    }

    if mapper.LastBankAddress != him.LastBankAddress {
        return fmt.Errorf("lastBankAddress differs: me=%v him=%v", mapper.LastBankAddress, him.LastBankAddress)
    }

    return compareSlice(mapper.BankMemory, him.BankMemory)
}

func (mapper *Mapper2) Copy() Mapper {
    return &Mapper2{
        BankMemory: copySlice(mapper.BankMemory),
        SaveRam: copySlice(mapper.SaveRam),
        LastBankAddress: mapper.LastBankAddress,
        Bank: mapper.Bank,
    }
}

func (mapper *Mapper2) Kind() int {
    return 2
}

func (mapper *Mapper2) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper2) Read(address uint16) byte {
    if address >= 0x6000 && address < 0x8000 {
        use := address - 0x6000
        return mapper.SaveRam[use]
    }

    if address < 0xc000 {
        offset := uint32(address - 0x8000)
        final := uint32(mapper.Bank) * 0x4000 + offset
        if final < uint32(len(mapper.BankMemory)) {
            return mapper.BankMemory[final]
        } else {
            log.Printf("mapper2: warning: reading invalid address 0x%x. max=0x%x", final, len(mapper.BankMemory))
            return 0
        }
    } else {
        offset := uint32(address - 0xc000)
        return mapper.BankMemory[mapper.LastBankAddress + offset]
    }
}

func (mapper *Mapper2) Write(cpu *CPUState, address uint16, value byte) error {
    if cpu.Debug > 0 {
        log.Printf("Accessing bank switching register 0x%x with value 0x%x", address, value)
    }

    if address >= 0x6000 && address < 0x8000 {
        use := address - 0x6000
        mapper.SaveRam[use] = value
        return nil
    }

    if address >= 0x8000 {
        mapper.Bank = value
    }

    return nil
}

func MakeMapper2(bankMemory []byte) Mapper {
    return &Mapper2{
        BankMemory: bankMemory,
        SaveRam: make([]byte, 0x2000),
        LastBankAddress: uint32(len(bankMemory) - 0x4000),
    }
}

type Mapper3 struct {
    ProgramRom []byte `json:"programrom"`
    BankMemory []byte `json:"bankmemory"`
}

func (mapper *Mapper3) Copy() Mapper {
    return &Mapper3{
        ProgramRom: copySlice(mapper.ProgramRom),
        BankMemory: copySlice(mapper.BankMemory),
    }
}

func (mapper *Mapper3) Kind() int {
    return 3
}

func (mapper *Mapper3) Compare(other Mapper) error {
    return fmt.Errorf("mapper3 compare unimplemented")
}

func (mapper *Mapper3) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper3) Read(address uint16) byte {
    if address >= 0x8000 {
        offset := address - 0x8000
        if uint32(offset) < uint32(len(mapper.ProgramRom)) {
            return mapper.ProgramRom[offset]
        } else {
            log.Printf("mapper3: invalid read at address=0x%x offset into program rom=0x%x", address, offset)
        }
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
    ProgramRom []byte `json:"programrom"`
    CharacterRom []byte `json:"characterrom"`
    SaveRam []byte `json:"saveram"`

    LastBank int `json:"lastbank"`

    IrqEnabled bool `json:"irqenabled"`
    IrqReload byte `json:"irqreload"`
    IrqCounter byte `json:"irqcounter"`
    IrqPending bool `json:"irqpending"`

    WramEnabled bool `json:"wramenabled"`
    WramWrite bool `json:"wramwrite"`

    ChrMode byte `json:"chrmode"`
    PrgMode byte `json:"prgmode"`
    /* used to select which of the chrRegister/prgRegister to write to */
    RegisterIndex byte `json:"register"`

    ChrRegister [6]byte `json:"chrregister"`
    PrgRegister [2]byte `json:"prgregister"`
}

func (mapper *Mapper4) Kind() int {
    return 4
}

func (mapper *Mapper4) Compare(other Mapper) error {
    return fmt.Errorf("mapper4 compare unimplemented")
}

func (mapper *Mapper4) Copy() Mapper {
    return &Mapper4{
        ProgramRom: copySlice(mapper.ProgramRom),
        CharacterRom: copySlice(mapper.CharacterRom),
        SaveRam: copySlice(mapper.SaveRam),
        LastBank: mapper.LastBank,
        IrqEnabled: mapper.IrqEnabled,
        IrqReload: mapper.IrqReload,
        IrqCounter: mapper.IrqCounter,
        IrqPending: mapper.IrqPending,
        WramEnabled: mapper.WramEnabled,
        WramWrite: mapper.WramWrite,
        ChrMode: mapper.ChrMode,
        PrgMode: mapper.PrgMode,
        RegisterIndex: mapper.RegisterIndex,
        ChrRegister: copySlice6(mapper.ChrRegister),
        PrgRegister: copySlice2(mapper.PrgRegister),
    }
}

func (mapper *Mapper4) IsIRQAsserted() bool {
    return mapper.IrqPending
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

    address := uint32(base)+uint32(offset)

    if address < uint32(len(mapper.ProgramRom)){
        return mapper.ProgramRom[address]
    } else {
        log.Printf("mapper4: Warning: reading out of bounds address 0x%x >= 0x%x", address, len(mapper.ProgramRom))
        return 0
    }
}

func (mapper *Mapper4) Read(address uint16) byte {
    if address >= 0x6000 && address < 0x8000 {

        if mapper.WramEnabled {
            use := address - 0x6000
            return mapper.SaveRam[use]
        }

        return 0
    }

    switch mapper.PrgMode {
        case 0:
            if address >= 0x8000 && address < 0xa000 {
                return mapper.ReadBank(address-0x8000, int(mapper.PrgRegister[0]))
            } else if address >= 0xa000 && address < 0xc000 {
                return mapper.ReadBank(address-0xa000, int(mapper.PrgRegister[1]))
            } else if address >= 0xc000 && address < 0xe000 {
                return mapper.ReadBank(address-0xc000, mapper.LastBank-1)
            } else {
                return mapper.ReadBank(address-0xe000, mapper.LastBank)
            }
        case 1:
            if address >= 0x8000 && address < 0xa000 {
                return mapper.ReadBank(address-0x8000, mapper.LastBank-1)
            } else if address >= 0xa000 && address < 0xc000 {
                return mapper.ReadBank(address-0xa000, int(mapper.PrgRegister[1]))
            } else if address >= 0xc000 && address < 0xe000 {
                return mapper.ReadBank(address-0xc000, int(mapper.PrgRegister[0]))
            } else {
                return mapper.ReadBank(address-0xe000, mapper.LastBank)
            }
        default:
            log.Printf("mapper4: unknown prg mode %v", mapper.PrgMode)
            return 0
    }

    return 0
}

func (mapper *Mapper4) CharacterBlock(length uint32, page byte) []byte {
    /* Kind of weird but I guess a page is always 0x400 (1k) bytes */
    base := uint32(page) * 0x400
    // log.Printf("mapper4: character rom bank page 0x%x at 0x%x - 0x%x", page, base, base + length)
    if base+length <= uint32(len(mapper.CharacterRom)) {
        return mapper.CharacterRom[base:base+length]
    } else {
        log.Printf("mapper4: warning: attempting to read out of bounds chr memory 0x%x to 0x%x, maximum 0x%x", base, base+length, len(mapper.CharacterRom))
        return nil
    }
}

func (mapper *Mapper4) SetChrBank(ppu *PPUState) error {
    switch mapper.ChrMode {
        case 0:
            ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x800, mapper.ChrRegister[0]))
            ppu.CopyCharacterRom(0x0800, mapper.CharacterBlock(0x800, mapper.ChrRegister[1]))
            ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x400, mapper.ChrRegister[2]))
            ppu.CopyCharacterRom(0x1400, mapper.CharacterBlock(0x400, mapper.ChrRegister[3]))
            ppu.CopyCharacterRom(0x1800, mapper.CharacterBlock(0x400, mapper.ChrRegister[4]))
            ppu.CopyCharacterRom(0x1c00, mapper.CharacterBlock(0x400, mapper.ChrRegister[5]))
            return nil
        case 1:
            ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x400, mapper.ChrRegister[2]))
            ppu.CopyCharacterRom(0x0400, mapper.CharacterBlock(0x400, mapper.ChrRegister[3]))
            ppu.CopyCharacterRom(0x0800, mapper.CharacterBlock(0x400, mapper.ChrRegister[4]))
            ppu.CopyCharacterRom(0x0c00, mapper.CharacterBlock(0x400, mapper.ChrRegister[5]))
            ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x800, mapper.ChrRegister[0]))
            ppu.CopyCharacterRom(0x1800, mapper.CharacterBlock(0x800, mapper.ChrRegister[1]))
            return nil
    }

    return fmt.Errorf("mapper4: unknown chr mode %v", mapper.ChrMode)
}

func (mapper *Mapper4) Write(cpu *CPUState, address uint16, value byte) error {
    if address >= 0x6000 && address < 0x8000 {

        address -= 0x6000

        if mapper.WramEnabled && mapper.WramWrite {
            mapper.SaveRam[address] = value
        }

        return nil
    }

    // log.Printf("mapper4: write to 0x%x value 0x%x", address, value)
    switch address {
        case 0x8000:
            mapper.ChrMode = (value >> 7) & 0x1
            mapper.PrgMode = (value >> 6) & 0x1

            mapper.RegisterIndex = value & 0x7
            mapper.SetChrBank(&cpu.PPU)
        case 0x8001:
            switch mapper.RegisterIndex {
                case 0, 1, 2, 3, 4, 5:
                    if mapper.RegisterIndex == 0 || mapper.RegisterIndex == 1 {
                        value = value & (^byte(1))
                    }
                    mapper.ChrRegister[mapper.RegisterIndex] = value
                    mapper.SetChrBank(&cpu.PPU)
                case 6, 7:
                    /* only use the first 6 bits, top two are ignored */
                    mapper.PrgRegister[mapper.RegisterIndex-6] = value & 0x3f
            }
        case 0xa000:
            mirror := value & 0x1
            /* FIXME: dont set mirroring for nes files that set 4-way mirroring */
            switch mirror {
                case 0:
                    cpu.PPU.SetVerticalMirror()
                case 1:
                    cpu.PPU.SetHorizontalMirror()
            }
        case 0xa001:
            /* prg ram protect */

            mapper.WramEnabled = (value >> 7) & 0x1 == 0x1
            mapper.WramWrite = (value >> 6) & 0x1 == 0
        case 0xc000:
            mapper.IrqReload = value
        case 0xc001:
            mapper.IrqCounter = value
        case 0xe000:
            mapper.IrqEnabled = false
            mapper.IrqPending = false
        case 0xe001:
            mapper.IrqEnabled = true
        default:
            log.Printf("FIXME: unknown mapper4 write to 0x%x with 0x%x", address, value)
    }

    return nil
}

func (mapper *Mapper4) Scanline() {
    if mapper.IrqCounter == 0 {
        mapper.IrqCounter = mapper.IrqReload
    } else {
        mapper.IrqCounter -= 1
        if mapper.IrqCounter == 0 && mapper.IrqEnabled {
            mapper.IrqPending = true
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
        LastBank: pages-1,
    }
}

type Mapper7 struct {
    ProgramRom []byte `json:"programrom"`
    CharacterRom []byte `json:"characterrom"`
    Bank int `json:"bank"` // 0-7
    Mirror int `json:"mirror"` // 0=1ScA, 1=1ScB
}

func (mapper *Mapper7) Compare(other Mapper) error {
    return fmt.Errorf("mapper7 compare unimplemented")
}

func (mapper *Mapper7) Kind() int {
    return 7
}

func (mapper *Mapper7) Copy() Mapper {
    return &Mapper7{
        ProgramRom: copySlice(mapper.ProgramRom),
        CharacterRom: copySlice(mapper.CharacterRom),
        Bank: mapper.Bank,
        Mirror: mapper.Mirror,
    }
}

func MakeMapper7(programRom []byte, chrMemory []byte) Mapper {
    return &Mapper7{
        ProgramRom: programRom,
        CharacterRom: chrMemory,
        Bank: 0,
        Mirror: 0,
    }
}

func (mapper *Mapper7) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper7) ReadBank(address uint16, bank int) byte {
    if bank < 0 {
        return 0
    }

    final := uint32(bank) * 0x8000 + uint32(address)
    if final < uint32(len(mapper.ProgramRom)) {
        return mapper.ProgramRom[final]
    }

    return 0
}

func (mapper *Mapper7) Read(address uint16) byte {
    if address >= 0x8000 {
        return mapper.ReadBank(address - 0x8000, mapper.Bank)
    }

    return 0
}

func (mapper *Mapper7) Write(cpu *CPUState, address uint16, value byte) error {
    if address >= 0x8000 {
        bank := value & 0b111
        mirror := (value >> 4) & 0x1

        mapper.Bank = int(bank)
        mapper.Mirror = int(mirror)

        switch mirror {
            case 0:
                cpu.PPU.SetScreenAMirror()
            case 1:
                cpu.PPU.SetScreenBMirror()
        }

        return nil
    }

    return fmt.Errorf("invalid mapper7 write address=0x%x value=0x%x", address, value)
}

type Mapper9 struct {
    ProgramRom []byte `json:"programrom"`
    CharacterRom []byte `json:"characterrom"`
    Pages int `json:"pages"`

    PrgRegister byte `json:"prg"`
    ChrRegister [4]byte `json:"chr"`
}

func (mapper *Mapper9) Compare(other Mapper) error {
    return fmt.Errorf("mapper9 compare unimplemented")
}

func (mapper *Mapper9) Kind() int {
    return 9
}

func (mapper *Mapper9) Copy() Mapper {
    return &Mapper9{
        ProgramRom: copySlice(mapper.ProgramRom),
        CharacterRom: copySlice(mapper.CharacterRom),
        Pages: mapper.Pages,
        PrgRegister: mapper.PrgRegister,
        ChrRegister: copySlice4(mapper.ChrRegister),
    }
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
        return mapper.ReadBank(address - 0x8000, int(mapper.PrgRegister))
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
    ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x1000, mapper.ChrRegister[1]))
    /* FIXME: Use chrRegister 1 or 3 depending on the ppu latch set at $fd or $fe */
    ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x1000, mapper.ChrRegister[2]))
}

func (mapper *Mapper9) Write(cpu *CPUState, address uint16, value byte) error {
    page := address >> 12
    switch page {
        case 0xa:
            mapper.PrgRegister = value
            return nil
        case 0xb:
            mapper.ChrRegister[0] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xc:
            mapper.ChrRegister[1] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xd:
            mapper.ChrRegister[2] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xe:
            mapper.ChrRegister[3] = value
            mapper.SetChrBank(&cpu.PPU)
            return nil
        case 0xf:
            mirror := value & 0x1
            switch mirror {
                case 0:
                    cpu.PPU.SetVerticalMirror()
                case 1:
                    cpu.PPU.SetHorizontalMirror()
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

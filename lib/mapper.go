package lib

import (
    "fmt"
    "log"
    "encoding/json"
    "reflect"
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
    // nsf mapper
    IsNSF() bool
}

// OpenBusMapper is an optional capability for cartridge reads that can leave
// the CPU data bus undriven (for example, disabled PRG-RAM).
type OpenBusMapper interface {
    ReadWithOpenBus(address uint16, openBus byte) byte
}

// CharacterMapper is an optional mapper capability for cartridges that handle
// PPU pattern reads. Other cartridges use the PPU's existing pattern memory.
// Implementations may update bank mappings after fetching the returned byte.
type CharacterMapper interface {
    ReadCharacter(ppu *PPUState, address uint16) byte
}

// WritableCharacterMapper is an optional capability for cartridge-managed CHR writes.
type WritableCharacterMapper interface {
    WriteCharacter(ppu *PPUState, address uint16, value byte)
}

// ScanlineMapper is an optional capability for cartridges with scanline clocks.
type ScanlineMapper interface {
    Scanline()
}

// CPUClockMapper advances cartridge hardware using elapsed CPU cycles.
type CPUClockMapper interface {
    ClockCPU(cycles uint64)
}

// ExpansionAudioMapper supplies audio mixed with the internal APU.
type ExpansionAudioMapper interface {
    ExpansionSample() float32
}

// ExpansionMapper identifies CPU registers below the usual cartridge window.
type ExpansionMapper interface {
    MapsExpansion(address uint16) bool
}

// NametableMapper owns routing of PPU nametable accesses, including CPU PPUDATA.
type NametableMapper interface {
    ReadNametable(ppu *PPUState, address uint16) byte
    WriteNametable(ppu *PPUState, address uint16, value byte)
}

// BackgroundTileMapper can supply attributes and pattern bytes per tile.
type BackgroundTileMapper interface {
    ReadBackgroundTile(ppu *PPUState, address, patternBase uint16, row byte) (low, high, palette byte)
}

// SpriteCharacterMapper distinguishes sprite fetches from CPU pattern reads.
type SpriteCharacterMapper interface {
    ReadSpriteCharacter(ppu *PPUState, address uint16) byte
}

// RenderingScanlineMapper sees visible scanlines and the end of rendering.
type RenderingScanlineMapper interface {
    RenderingScanline(line int, rendering bool)
}

var _ OpenBusMapper = (*Mapper1)(nil)
var _ CharacterMapper = (*Mapper1)(nil)
var _ WritableCharacterMapper = (*Mapper1)(nil)
var _ CharacterMapper = (*Mapper9)(nil)
var _ ScanlineMapper = (*Mapper4)(nil)

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
        case 5:
            mapper5, err := unmarshalMapper[*Mapper5](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper5
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
        case 11:
            mapper11, err := unmarshalMapper[*Mapper11](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper11
            return nil
        case 24:
            mapper24, err := unmarshalMapper[*Mapper24](data)
            if err != nil {
                return err
            }
            state.Mapper = mapper24
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

// mirrorBank removes unconnected address lines for standard power-of-two ROMs.
// Non-power-of-two images need board-specific wiring; leave their bank number
// intact so the caller's bounds check rejects holes instead of inventing aliases.
func mirrorBank(bank int, pages int) int {
    if pages > 0 && pages & (pages - 1) == 0 {
        return bank & (pages - 1)
    }
    return bank
}

func MakeMapper(mapper uint32, programRom []byte, chrMemory []byte) (Mapper, error) {
    switch mapper {
        case 0: return MakeMapper0(programRom), nil
        case 1: return MakeMapper1(programRom, chrMemory), nil
        case 2: return MakeMapper2(programRom), nil
        case 3: return MakeMapper3(programRom, chrMemory), nil
        case 4: return MakeMapper4(programRom, chrMemory), nil
        case 5: return MakeMapper5(programRom, chrMemory), nil
        case 7: return MakeMapper7(programRom, chrMemory), nil
        case 9: return MakeMapper9(programRom, chrMemory), nil
        case 11: return MakeMapper11(programRom, chrMemory), nil
        case 24: return MakeMapper24(programRom, chrMemory), nil
        default: return nil, fmt.Errorf("Unimplemented mapper %v", mapper)
    }
}

type Mapper0 struct {
    SaveRam []byte `json:"saveram"`
    BankMemory []byte `json:"bank"`
}

func (mapper *Mapper0) IsNSF() bool {
    return false
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
    if address >= 0x6000 && address < 0x8000 {
        use := address - 0x6000
        mapper.SaveRam[use] = value
        return nil
    }

    // logs are noisy, just ignore the write
    return nil
    // return fmt.Errorf("mapper0 does not support bank switching at address 0x%x: 0x%x", address, value)
}

func (mapper *Mapper0) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper0) Read(address uint16) byte {
    if address >= 0x6000 && address < 0x8000 {
        use := address - 0x6000
        return mapper.SaveRam[use]
    }

    use := address - uint16(0x8000)
    if len(mapper.BankMemory) == 16*1024 {
        use = use % 0x4000
        return mapper.BankMemory[use]
    } else {
        if int(use) < len(mapper.BankMemory) {
            return mapper.BankMemory[use]
        } else {
            log.Printf("mapper0: invalid read at %v beyond %v", use, len(mapper.BankMemory))
            return 0
        }
    }
}

func (mapper *Mapper0) Kind() int {
    return 0
}

func MakeMapper0(bankMemory []byte) Mapper {
    return &Mapper0{
        SaveRam: make([]byte, 0x8000 - 0x6000),
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
    LastWriteCycle uint64 `json:"lastwritecycle"`
    HasLastWriteCycle bool `json:"haslastwritecycle"`

    Mirror byte `json:"mirror"`
    PrgBankMode byte `json:"prgbankmode"`
    ChrBankMode byte `json:"chrbankmode"`
    PrgBank byte `json:"prgbank"`

    ChrRegister0 byte `json:"chrregister0"`
    ChrRegister1 byte `json:"chrregister1"`

    /* FIXME: this might get mapped from the bank memory, not sure */
    PRGRam []byte `json:"prgram"`
}

func (mapper *Mapper1) IsNSF() bool {
    return false
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

    /* blaster master reads addresses higher than available memory, so we wrap around */

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
        LastWriteCycle: mapper.LastWriteCycle,
        HasLastWriteCycle: mapper.HasLastWriteCycle,
        Mirror: mapper.Mirror,
        PrgBankMode: mapper.PrgBankMode,
        ChrBankMode: mapper.ChrBankMode,
        ChrRegister0: mapper.ChrRegister0,
        ChrRegister1: mapper.ChrRegister1,
        PrgBank: mapper.PrgBank,
        PRGRam: copySlice(mapper.PRGRam),
    }
}

func (mapper *Mapper1) Read(address uint16) byte {
    return mapper.ReadWithOpenBus(address, 0)
}

func (mapper *Mapper1) ReadWithOpenBus(address uint16, openBus byte) byte {
    if address >= 0x6000 && address < 0x8000 {
        // Standard MMC1B/C: PRG register bit 4 disables both RAM reads/writes.
        if mapper.PrgBank & 0x10 != 0 {
            return openBus
        }
        return mapper.PRGRam[address - uint16(0x6000)]
    }

    baseAddress := address - uint16(0x8000)

    const pageSize32k = 0x8000
    const pageSize16k = 0x4000

    /* For SUROM (dragon warrior 4), the 5th bit of a chr register is used as the 5th bit of the prg register.
     * FCEUX only uses chr register 0, even though the mmc documentation says that both chr reg 0 and chr reg 1
     * supply a 5th bit to prg.
     */
    hijackedPrgBit := int(mapper.ChrRegister0 & 0x10)
    // Bit 4 controls PRG-RAM, not PRG-ROM bank selection.
    prgBank := int(mapper.PrgBank & 0x0f)

    switch mapper.PrgBankMode {
        /* P=0, read in 32k mode */
        case 0, 1:
            // Combine the SUROM outer bit in 16 KiB units before selecting
            // the 32 KiB pair. PRG register bit 0 is ignored in this mode.
            return mapper.ReadBank(pageSize32k, (prgBank | hijackedPrgBit) >> 1, baseAddress)
        /* P=1, S=0, read in 16k mode where 0x8000 is mapped to 0, and 0xc000 is mapped to the program bank */
        case 2:
            if address < 0xc000 {
                /* hijacked prg bit applies to fixed pages as well */
                return mapper.ReadBank(pageSize16k, 0 + hijackedPrgBit, baseAddress)
            }

            return mapper.ReadBank(pageSize16k, prgBank + hijackedPrgBit, address - 0xc000)
        /* P=1, S=1, read in 16k mode where 0x8000 is mapped to the program bank, and 0xc000 is mapped to page 0xf */
        case 3:
            if address < 0xc000 {
                return mapper.ReadBank(pageSize16k, prgBank + hijackedPrgBit, baseAddress)
            }

            /* hijacked prg bit applies to fixed pages as well */
            return mapper.ReadBank(pageSize16k, 0xf + hijackedPrgBit, address - 0xc000)
    }

    return 0
}

// With 8 KiB CHR-RAM only the low CHR bank bit is connected in 4 KiB mode.
// Higher bits can select outer PRG banks on boards such as SUROM; they must
// not affect CHR-RAM addressing. In 8 KiB mode the RAM is mapped directly.
func (mapper *Mapper1) chrRAMAddress(address uint16) uint16 {
    if mapper.ChrBankMode == 0 {
        return address & 0x1fff
    }
    bank := mapper.ChrRegister0
    if address >= 0x1000 {
        bank = mapper.ChrRegister1
    }
    return uint16(bank & 1) * 0x1000 + (address & 0xfff)
}

func (mapper *Mapper1) ReadCharacter(ppu *PPUState, address uint16) byte {
    if len(mapper.CharacterMemory) == 0 {
        // PPU pattern memory holds the physical RAM, not copies of mapped banks.
        return ppu.VideoMemory[mapper.chrRAMAddress(address)]
    }
    // Keep the existing CHR-ROM bank-copy path unchanged.
    return ppu.VideoMemory[address]
}

func (mapper *Mapper1) WriteCharacter(ppu *PPUState, address uint16, value byte) {
    if len(mapper.CharacterMemory) == 0 {
        ppu.VideoMemory[mapper.chrRAMAddress(address)] = value
    }
}

func (mapper *Mapper1) copyChrBank(ppu *PPUState, destination uint16, register byte, size int) {
    // CHR-RAM uses dynamic address mapping and must never be overwritten here.
    if len(mapper.CharacterMemory) == 0 {
        return
    }
    // Registers select 4 KiB units; widen before computing ROM offsets.
    base := int(register) * 0x1000
    end := base + size
    if end > len(mapper.CharacterMemory) {
        log.Printf("[mapper1] Warning: could not copy character data from range %v to %v", base, end)
        return
    }
    ppu.CopyCharacterRom(destination, mapper.CharacterMemory[base:end], false)
}

// Rebuild both pattern tables from the stored bank registers and current mode.
func (mapper *Mapper1) SetChrBank(ppu *PPUState) {
    if mapper.ChrBankMode == 0 {
        // The low bank bit is ignored when selecting an 8 KiB pair.
        mapper.copyChrBank(ppu, 0x0000, mapper.ChrRegister0 & 0x1e, 0x2000)
    } else {
        mapper.copyChrBank(ppu, 0x0000, mapper.ChrRegister0, 0x1000)
        mapper.copyChrBank(ppu, 0x1000, mapper.ChrRegister1, 0x1000)
    }
}

func (mapper *Mapper1) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper1) Write(cpu *CPUState, address uint16, value byte) error {
    if address >= 0x6000 && address < 0x8000 {
        if mapper.PrgBank & 0x10 == 0 {
            mapper.PRGRam[address - uint16(0x6000)] = value
        }
        return nil
    }

    // RMW instructions issue adjacent writes. Our instruction-level CPU gives
    // both writes the same timestamp, so filter same-cycle writes as well.
    // Every register write extends the window, including ignored/reset writes.
    consecutive := mapper.HasLastWriteCycle && cpu.Cycle - mapper.LastWriteCycle <= 1
    mapper.LastWriteCycle = cpu.Cycle
    mapper.HasLastWriteCycle = true
    // Bit-7 reset writes are always honored, even inside the inhibit window.
    if consecutive && value & 0x80 == 0 {
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
        // Serial reset applies control |= 0x0c, preserving CHR mode/mirroring.
        mapper.PrgBankMode = 3
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
                mapper.SetChrBank(&cpu.PPU)
            } else if address >= 0xa000 && address <= 0xbfff {
                mapper.ChrRegister0 = mapper.Register
                if mapper.ChrBankMode == 0 {
                    mapper.SetChrBank(&cpu.PPU)
                } else {
                    mapper.copyChrBank(&cpu.PPU, 0x0000, mapper.ChrRegister0, 0x1000)
                }
            } else if address >= 0xc000 && address <= 0xdfff {
                mapper.ChrRegister1 = mapper.Register
                if mapper.ChrBankMode == 1 {
                    mapper.copyChrBank(&cpu.PPU, 0x1000, mapper.ChrRegister1, 0x1000)
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
        ChrRegister0: 0,
        ChrRegister1: 0,
        PRGRam: make([]byte, 0x8000 - 0x6000),
        Last4kBank: pages-1,
    }
}

type Mapper2 struct {
    BusConflicts bool `json:"busconflicts"`
    BankMemory []byte `json:"bankmemory"`
    SaveRam []byte `json:"saveram"`
    LastBankAddress uint32 `json:"lastaddress"`
    Bank byte `json:"bank"`
}

func (mapper *Mapper2) IsNSF() bool {
    return false
}

func (mapper *Mapper2) Compare(other Mapper) error {
    him, ok := other.(*Mapper2)
    if !ok {
        return fmt.Errorf("other was not a mapper2")
    }

    if mapper.BusConflicts != him.BusConflicts {
        return fmt.Errorf("bus conflict configuration differs")
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
        BusConflicts: mapper.BusConflicts,
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

    if address < 0x8000 {
        return 0
    }
    if address < 0xc000 {
        offset := uint32(address - 0x8000)
        bank := mirrorBank(int(mapper.Bank), len(mapper.BankMemory) / 0x4000)
        final := uint32(bank) * 0x4000 + offset
        if final < uint32(len(mapper.BankMemory)) {
            return mapper.BankMemory[final]
        } else {
            log.Printf("mapper2: warning: reading invalid address 0x%x. max=0x%x", final, len(mapper.BankMemory))
            return 0
        }
    } else {
        offset := uint32(address - 0xc000)
        final := mapper.LastBankAddress + offset
        if final < uint32(len(mapper.BankMemory)) {
            return mapper.BankMemory[final]
        }
        return 0
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
        if mapper.BusConflicts {
            value &= mapper.Read(address) // ROM drives the OLD mapping during the write.
            cpu.Databus = value
        }
        mapper.Bank = value
    }

    return nil
}

func MakeMapper2(bankMemory []byte) Mapper {
    lastBankAddress := 0
    if len(bankMemory) >= 0x4000 {
        lastBankAddress = len(bankMemory) - 0x4000
    }
    return &Mapper2{
        BankMemory: bankMemory,
        SaveRam: make([]byte, 0x2000),
        LastBankAddress: uint32(lastBankAddress),
    }
}

type Mapper3 struct {
    BusConflicts bool `json:"busconflicts"`
    ProgramRom []byte `json:"programrom"`
    BankMemory []byte `json:"bankmemory"`
}

func (mapper *Mapper3) IsNSF() bool {
    return false
}

func (mapper *Mapper3) Copy() Mapper {
    return &Mapper3{
        BusConflicts: mapper.BusConflicts,
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
        // A 16 KiB CNROM PRG-ROM does not decode A14: mirror it at $C000.
        if len(mapper.ProgramRom) == 0x4000 {
            offset &= 0x3fff
        }
        if uint32(offset) < uint32(len(mapper.ProgramRom)) {
            return mapper.ProgramRom[offset]
        } else {
            log.Printf("mapper3: invalid read at address=0x%x offset into program rom=0x%x", address, offset)
        }
    }

    return 0
}

func (mapper *Mapper3) Write(cpu *CPUState, address uint16, value byte) error {
    if address < 0x8000 {
        return nil
    }
    if mapper.BusConflicts {
        value &= mapper.Read(address)
        cpu.Databus = value
    }
    bank := mirrorBank(int(value), len(mapper.BankMemory) / 0x2000)
    base := bank * 0x2000
    if base + 0x2000 <= len(mapper.BankMemory) {
        cpu.PPU.CopyCharacterRom(0x0000, mapper.BankMemory[base:base+0x2000], false)
    }
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
    IrqReload byte `json:"irqreload"` // IRQ counter latch ($C000).
    IrqReloadPending bool `json:"irqreloadpending"`
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

func (mapper *Mapper4) IsNSF() bool {
    return false
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
        IrqReloadPending: mapper.IrqReloadPending,
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
    pages := len(mapper.ProgramRom) / 0x2000
    if pages == 0 || bank < 0 {
        return 0
    }
    // Standard MMC3 PRG-ROM sizes are powers of two. Unconnected bank
    // address lines alias existing banks; retain the full register value.
    base := mirrorBank(bank, pages) * 0x2000

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

    // return 0
}

func (mapper *Mapper4) CharacterBlock(length uint32, page byte) []byte {
    /* Kind of weird but I guess a page is always 0x400 (1k) bytes */
    pages := len(mapper.CharacterRom) / 0x400
    if pages == 0 {
        return nil
    }
    base := uint32(mirrorBank(int(page), pages)) * 0x400
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
            ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x800, mapper.ChrRegister[0]), false)
            ppu.CopyCharacterRom(0x0800, mapper.CharacterBlock(0x800, mapper.ChrRegister[1]), false)
            ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x400, mapper.ChrRegister[2]), false)
            ppu.CopyCharacterRom(0x1400, mapper.CharacterBlock(0x400, mapper.ChrRegister[3]), false)
            ppu.CopyCharacterRom(0x1800, mapper.CharacterBlock(0x400, mapper.ChrRegister[4]), false)
            ppu.CopyCharacterRom(0x1c00, mapper.CharacterBlock(0x400, mapper.ChrRegister[5]), false)
            return nil
        case 1:
            ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x400, mapper.ChrRegister[2]), false)
            ppu.CopyCharacterRom(0x0400, mapper.CharacterBlock(0x400, mapper.ChrRegister[3]), false)
            ppu.CopyCharacterRom(0x0800, mapper.CharacterBlock(0x400, mapper.ChrRegister[4]), false)
            ppu.CopyCharacterRom(0x0c00, mapper.CharacterBlock(0x400, mapper.ChrRegister[5]), false)
            ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x800, mapper.ChrRegister[0]), false)
            ppu.CopyCharacterRom(0x1800, mapper.CharacterBlock(0x800, mapper.ChrRegister[1]), false)
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

    // MMC3 decodes A15-A13 and A0; A12-A1 are register aliases.
    // Decode only after handling PRG-RAM, whose full address is significant.
    switch address & 0xe001 {
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
            // Four-screen boards hardwire all four nametables; $A000 and
            // its aliases cannot switch them to horizontal/vertical mirroring.
            if cpu.PPU.NametableMirror == NametableMirrorFourScreen {
                return nil
            }
            mirror := value & 0x1
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
            // The written value is ignored; reload from $C000 on the next clock.
            mapper.IrqReloadPending = true
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
    if mapper.IrqCounter == 0 || mapper.IrqReloadPending {
        mapper.IrqCounter = mapper.IrqReload
    } else {
        mapper.IrqCounter -= 1
    }
    mapper.IrqReloadPending = false
    // Standard MMC3B/C asserts on zero after either decrement OR reload.
    // MMC3A's alternate zero-reload behavior is not modeled here.
    if mapper.IrqCounter == 0 && mapper.IrqEnabled {
        mapper.IrqPending = true
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

// Mapper5 implements the MMC5 cartridge hardware used by the US Castlevania
// III. Scanline detection is approximated by PPU rendering callbacks.
type Mapper5 struct {
    ProgramROM, CharacterROM, WorkRAM       []byte
    ExRAM                                   [1024]byte
    PRGMode, CHRMode                        byte
    PRGBanks                                [5]byte // $5113 through $5117
    CHRBanks                                [12]uint16
    CHRUpper                                byte
    LastCHRBackground                       bool
    Protect1, Protect2                      byte
    ExMode, Nametables, FillTile, FillColor byte
    IRQTarget, IRQLine                      byte
    IRQEnabled, IRQPending, InFrame         bool
    MultiplyA, MultiplyB                    byte
}

func MakeMapper5(programROM, characterROM []byte) *Mapper5 {
    return &Mapper5{ProgramROM: programROM, CharacterROM: characterROM,
        WorkRAM: make([]byte, 0x10000), PRGMode: 3,
        PRGBanks: [5]byte{0, 0, 0, 0, 0xff}}
}

func (*Mapper5) Kind() int                         { return 5 }
func (*Mapper5) IsNSF() bool                       { return false }
func (*Mapper5) MapsExpansion(address uint16) bool { return address >= 0x5000 && address < 0x6000 }
func (mapper *Mapper5) IsIRQAsserted() bool             { return mapper.IRQEnabled && mapper.IRQPending }

func (mapper *Mapper5) Copy() Mapper {
    copied := *mapper
    copied.ProgramROM = copySlice(mapper.ProgramROM)
    copied.CharacterROM = copySlice(mapper.CharacterROM)
    copied.WorkRAM = copySlice(mapper.WorkRAM)
    return &copied
}

func (mapper *Mapper5) Compare(other Mapper) error {
    if !reflect.DeepEqual(mapper, other) {
        return fmt.Errorf("MMC5 state differs")
    }
    return nil
}

func (mapper *Mapper5) prgAddress(address uint16) (isROM bool, offset int) {
    if address < 0x8000 {
        return false, int(mapper.PRGBanks[0]&7)*0x2000 + int(address&0x1fff)
    }
    slot := int(address-0x8000) / 0x2000
    register, bankSpan := slot+1, 1
    switch mapper.PRGMode {
    case 0:
        register, bankSpan = 4, 4
    case 1:
        register, bankSpan = 2+2*(slot/2), 2
    case 2:
        if slot < 2 {
            register, bankSpan = 2, 2
        }
    }
    value := mapper.PRGBanks[register]
    isROM = register == 4 || value&0x80 != 0
    bank := int(value & 7)
    if isROM {
        bank = int(value & 0x7f)
    }
    bank = bank&^(bankSpan-1) | slot&(bankSpan-1)
    if isROM {
        bank = mirrorBank(bank, len(mapper.ProgramROM)/0x2000)
    }
    return isROM, bank*0x2000 + int(address&0x1fff)
}

func (mapper *Mapper5) Read(address uint16) byte { return mapper.ReadWithOpenBus(address, 0) }

func (mapper *Mapper5) ReadWithOpenBus(address uint16, openBus byte) byte {
    switch address {
    case 0xfffa, 0xfffb:
        // Fetching the NMI vector also resets the MMC5 scanline detector.
        mapper.InFrame, mapper.IRQPending, mapper.IRQLine = false, false, 0
    case 0x5204:
        var status byte
        if mapper.IRQPending {
            status |= 0x80
        }
        if mapper.InFrame {
            status |= 0x40
        }
        mapper.IRQPending = false
        return status
    case 0x5205:
        return byte(uint16(mapper.MultiplyA) * uint16(mapper.MultiplyB))
    case 0x5206:
        return byte(uint16(mapper.MultiplyA) * uint16(mapper.MultiplyB) >> 8)
    }
    if address >= 0x5c00 && address < 0x6000 && mapper.ExMode >= 2 {
        return mapper.ExRAM[address&0x3ff]
    }
    if address >= 0x6000 {
        isROM, offset := mapper.prgAddress(address)
        data := mapper.WorkRAM
        if isROM {
            data = mapper.ProgramROM
        }
        if offset < len(data) {
            return data[offset]
        }
    }
    return openBus
}

func (mapper *Mapper5) Write(_ *CPUState, address uint16, value byte) error {
    switch {
    case address >= 0x6000:
        isROM, offset := mapper.prgAddress(address)
        if !isROM && mapper.Protect1 == 2 && mapper.Protect2 == 1 && offset < len(mapper.WorkRAM) {
            mapper.WorkRAM[offset] = value
        }
    case address >= 0x5c00:
        if mapper.ExMode < 3 {
            if mapper.ExMode < 2 && !mapper.InFrame {
                value = 0
            }
            mapper.ExRAM[address&0x3ff] = value
        }
    case address >= 0x5113 && address <= 0x5117:
        mapper.PRGBanks[address-0x5113] = value
    case address >= 0x5120 && address <= 0x512b:
        mapper.CHRBanks[address-0x5120] = uint16(value) | uint16(mapper.CHRUpper)<<8
        mapper.LastCHRBackground = address >= 0x5128
    default:
        switch address {
        case 0x5100:
            mapper.PRGMode = value & 3
        case 0x5101:
            mapper.CHRMode = value & 3
        case 0x5102:
            mapper.Protect1 = value & 3
        case 0x5103:
            mapper.Protect2 = value & 3
        case 0x5104:
            mapper.ExMode = value & 3
        case 0x5105:
            mapper.Nametables = value
        case 0x5106:
            mapper.FillTile = value
        case 0x5107:
            mapper.FillColor = (value & 3) * 0x55
        case 0x5130:
            mapper.CHRUpper = value & 3
        case 0x5203:
            mapper.IRQTarget = value
        case 0x5204:
            mapper.IRQEnabled = value&0x80 != 0
        case 0x5205:
            mapper.MultiplyA = value
        case 0x5206:
            mapper.MultiplyB = value
        }
    }
    return nil
}

func (mapper *Mapper5) RenderingScanline(line int, rendering bool) {
    if !rendering || line >= 240 {
        mapper.InFrame = false
        return
    }
    if !mapper.InFrame || line == 0 {
        mapper.InFrame, mapper.IRQLine = true, 0
        return
    }
    mapper.IRQLine++
    if mapper.IRQLine == mapper.IRQTarget {
        mapper.IRQPending = true
    }
}

func (mapper *Mapper5) characterByte(bank, size, offset int) byte {
    bank = mirrorBank(bank, len(mapper.CharacterROM)/size)
    index := bank*size + offset
    if index >= len(mapper.CharacterROM) {
        return 0
    }
    return mapper.CharacterROM[index]
}

func (mapper *Mapper5) readCHR(address uint16, background bool) byte {
    size := 0x2000 >> mapper.CHRMode
    offset := int(address)
    register := (offset/size+1)*(size/0x400) - 1
    if background {
        register = 8 + register%4
    }
    return mapper.characterByte(int(mapper.CHRBanks[register]), size, offset%size)
}

func (mapper *Mapper5) ReadCharacter(ppu *PPUState, address uint16) byte {
    return mapper.readCHR(address, ppu.GetSpriteSize() == SpriteSize8x16 && mapper.LastCHRBackground)
}

func (mapper *Mapper5) ReadSpriteCharacter(_ *PPUState, address uint16) byte {
    return mapper.readCHR(address, false)
}

func (mapper *Mapper5) nametableSource(address uint16) byte {
    return (mapper.Nametables >> (((address >> 10) & 3) * 2)) & 3
}

func (mapper *Mapper5) ReadNametable(ppu *PPUState, address uint16) byte {
    offset := address & 0x3ff
    source := mapper.nametableSource(address)
    switch source {
    case 0, 1:
        return ppu.NametableMemory[uint16(source)*0x400+offset]
    case 2:
        if mapper.ExMode < 2 {
            return mapper.ExRAM[offset]
        }
    case 3:
        if offset >= 0x3c0 {
            return mapper.FillColor
        }
        return mapper.FillTile
    }
    return 0
}

func (mapper *Mapper5) WriteNametable(ppu *PPUState, address uint16, value byte) {
    offset := address & 0x3ff
    source := mapper.nametableSource(address)
    switch source {
    case 0, 1:
        ppu.NametableMemory[uint16(source)*0x400+offset] = value
    case 2:
        if mapper.ExMode < 2 {
            mapper.ExRAM[offset] = value
        }
    }
}

func (mapper *Mapper5) ReadBackgroundTile(ppu *PPUState, address, patternBase uint16, row byte) (byte, byte, byte) {
    tile := mapper.ReadNametable(ppu, address)
    if mapper.ExMode == 1 {
        attribute := mapper.ExRAM[address&0x3ff]
        bank := int(attribute&0x3f) | int(mapper.CHRUpper)<<6
        offset := int(tile)*16 + int(row)
        return mapper.characterByte(bank, 0x1000, offset), mapper.characterByte(bank, 0x1000, offset+8), attribute >> 6
    }
    attributeAddress := 0x23c0 | (address & 0xc00) | ((address >> 4) & 0x38) | ((address >> 2) & 7)
    shift := (address & 2) | ((address >> 4) & 4)
    palette := (mapper.ReadNametable(ppu, attributeAddress) >> shift) & 3
    pattern := patternBase + uint16(tile)*16 + uint16(row)
    background := ppu.GetSpriteSize() == SpriteSize8x16
    return mapper.readCHR(pattern, background), mapper.readCHR(pattern+8, background), palette
}

type Mapper7 struct {
    BusConflicts bool `json:"busconflicts"`
    ProgramRom []byte `json:"programrom"`
    CharacterRom []byte `json:"characterrom"`
    Bank int `json:"bank"` // 0-15 (upper bank bits alias on smaller boards)
    Mirror int `json:"mirror"` // 0=1ScA, 1=1ScB
}

func (mapper *Mapper7) IsNSF() bool {
    return false
}

func (mapper *Mapper7) Compare(other Mapper) error {
    return fmt.Errorf("mapper7 compare unimplemented")
}

func (mapper *Mapper7) Kind() int {
    return 7
}

func (mapper *Mapper7) Copy() Mapper {
    return &Mapper7{
        BusConflicts: mapper.BusConflicts,
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

    bank = mirrorBank(bank, len(mapper.ProgramRom) / 0x8000)
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
        if mapper.BusConflicts {
            value &= mapper.Read(address)
            cpu.Databus = value
        }
        bank := value & 0x0f
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
    // Each pattern table independently selects its FD or FE CHR register.
    Latches [2]byte `json:"latches"`
}

func (mapper *Mapper9) IsNSF() bool {
    return false
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
        Latches: mapper.Latches,
    }
}

func (mapper *Mapper9) IsIRQAsserted() bool {
    return false
}

func (mapper *Mapper9) ReadBank(offset uint16, bank int) byte {
    if bank < 0 || offset >= 0x2000 {
        return 0
    }
    bank = mirrorBank(bank, len(mapper.ProgramRom) / 0x2000)
    final := uint32(bank) * 0x2000 + uint32(offset)
    if final < uint32(len(mapper.ProgramRom)) {
        return mapper.ProgramRom[final]
    }
    return 0
}

func (mapper *Mapper9) Read(address uint16) byte {
    if address < 0x8000 {
        return 0
    }
    if address >= 0x8000 && address < 0xa000 {
        return mapper.ReadBank(address - 0x8000, int(mapper.PrgRegister))
    } else if address >= 0xa000 && address < 0xc000 {
        return mapper.ReadBank(address - 0xa000, mapper.Pages-3)
    } else if address >= 0xc000 && address < 0xe000 {
        return mapper.ReadBank(address - 0xc000, mapper.Pages-2)
    } else {
        return mapper.ReadBank(address - 0xe000, mapper.Pages-1)
    }

    // return 0
}

func (mapper *Mapper9) CharacterBlock(pageSize uint16, register byte) []byte {
    pages := len(mapper.CharacterRom) / int(pageSize)
    if pages == 0 {
        return nil
    }
    base := mirrorBank(int(register & 0x1f), pages) * int(pageSize)
    if base + int(pageSize) > len(mapper.CharacterRom) {
        return nil
    }
    return mapper.CharacterRom[base:base+int(pageSize)]
}

func (mapper *Mapper9) chrRegister(table int) byte {
    index := table * 2
    // Treat absent latch fields in older save states as FE.
    if mapper.Latches[table] != 0xfd {
        index++
    }
    return mapper.ChrRegister[index]
}

func (mapper *Mapper9) SetChrBank(ppu *PPUState) {
    ppu.CopyCharacterRom(0x0000, mapper.CharacterBlock(0x1000, mapper.chrRegister(0)), true)
    ppu.CopyCharacterRom(0x1000, mapper.CharacterBlock(0x1000, mapper.chrRegister(1)), false)
    ppu.CharacterRomLow = 0
    ppu.CharacterRomHigh = 0x2000
}

// ReadCharacter returns the byte from the OLD bank before updating the latch.
// The triggering read itself must not see the newly selected bank.
func (mapper *Mapper9) ReadCharacter(ppu *PPUState, address uint16) byte {
    table := int(address / 0x1000)
    bank := mapper.CharacterBlock(0x1000, mapper.chrRegister(table))
    var value byte
    if len(bank) != 0 {
        value = bank[address & 0xfff]
    }
    var latch byte
    switch {
        case address == 0x0fd8, address >= 0x1fd8 && address <= 0x1fdf:
            latch = 0xfd
        case address == 0x0fe8, address >= 0x1fe8 && address <= 0x1fef:
            latch = 0xfe
    }
    if latch != 0 && mapper.Latches[table] != latch {
        mapper.Latches[table] = latch
        mapper.SetChrBank(ppu)
    }
    return value
}

func (mapper *Mapper9) Write(cpu *CPUState, address uint16, value byte) error {
    page := address >> 12
    if page >= 0xb && page <= 0xe {
        value &= 0x1f
    }
    switch page {
        case 0xa:
            mapper.PrgRegister = value & 0x0f
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
        Latches: [2]byte{0xfe, 0xfe},
    }
}

// Mapper11 is the Color Dreams board: a single latch switches 32 KiB of PRG
// and 8 KiB of CHR. Nametable mirroring is hardwired, not latch-controlled.
type Mapper11 struct {
    ProgramROM []byte `json:"programrom"`
    CharacterROM []byte `json:"characterrom"`
    Register byte `json:"register"`
    BusConflicts bool `json:"busconflicts"`
}

func MakeMapper11(programROM, characterROM []byte) *Mapper11 {
    return &Mapper11{ProgramROM: programROM, CharacterROM: characterROM}
}

func (*Mapper11) Kind() int { return 11 }
func (*Mapper11) IsNSF() bool { return false }
func (*Mapper11) IsIRQAsserted() bool { return false }

func (mapper *Mapper11) Copy() Mapper {
    copied := *mapper
    copied.ProgramROM = copySlice(mapper.ProgramROM)
    copied.CharacterROM = copySlice(mapper.CharacterROM)
    return &copied
}

func (mapper *Mapper11) Compare(other Mapper) error {
    if !reflect.DeepEqual(mapper, other) {
        return fmt.Errorf("Color Dreams mapper state differs")
    }
    return nil
}

func (mapper *Mapper11) Read(address uint16) byte {
    return mapper.ReadWithOpenBus(address, 0)
}

func (mapper *Mapper11) ReadWithOpenBus(address uint16, openBus byte) byte {
    if address < 0x8000 {
        return openBus
    }
    bank := mirrorBank(int(mapper.Register & 3), len(mapper.ProgramROM) / 0x8000)
    offset := bank*0x8000 + int(address - 0x8000)
    if offset >= len(mapper.ProgramROM) {
        return openBus
    }
    return mapper.ProgramROM[offset]
}

func (mapper *Mapper11) Write(cpu *CPUState, address uint16, value byte) error {
    if address >= 0x8000 {
        if mapper.BusConflicts {
            // The old mapping still drives the ROM side of the bus.
            value &= mapper.ReadWithOpenBus(address, value)
            cpu.Databus = value
        }
        mapper.Register = value
    }
    return nil
}

func (mapper *Mapper11) ReadCharacter(_ *PPUState, address uint16) byte {
    bank := mirrorBank(int(mapper.Register >> 4), len(mapper.CharacterROM) / 0x2000)
    offset := bank*0x2000 + int(address & 0x1fff)
    if offset >= len(mapper.CharacterROM) {
        return 0
    }
    return mapper.CharacterROM[offset]
}

// Mapper24 implements Konami VRC6a, whose register select pins use CPU A0/A1
// in order (unlike VRC6b / mapper 26).
type Mapper24 struct {
    ProgramROM, CharacterMemory, WorkRAM []byte
    CharacterRAM bool
    ProgramBank16, ProgramBank8 byte
    CharacterBanks [8]byte
    BankingMode byte
    IRQReload, IRQCounter, IRQControl byte
    IRQPrescaler uint64
    IRQPending bool
    Audio VRC6Audio
}

func MakeMapper24(programROM, characterROM []byte) *Mapper24 {
    characterRAM := len(characterROM) == 0
    if characterRAM {
        characterROM = make([]byte, 0x2000)
    }
    audio := MakeVRC6Audio(nil)
    audio.Pulse1.SetEnable(false)
    audio.Pulse2.SetEnable(false)
    audio.Saw.SetEnable(false)
    return &Mapper24{
        ProgramROM: programROM, CharacterMemory: characterROM, CharacterRAM: characterRAM,
        WorkRAM: make([]byte, 0x2000), Audio: *audio,
    }
}

func (*Mapper24) Kind() int { return 24 }
func (*Mapper24) IsNSF() bool { return false }
func (mapper *Mapper24) IsIRQAsserted() bool { return mapper.IRQPending }
func (mapper *Mapper24) ExpansionSample() float32 { return mapper.Audio.GenerateSample() }

func (mapper *Mapper24) Copy() Mapper {
    copied := *mapper
    copied.ProgramROM = copySlice(mapper.ProgramROM)
    copied.CharacterMemory = copySlice(mapper.CharacterMemory)
    copied.WorkRAM = copySlice(mapper.WorkRAM)
    return &copied
}

func (mapper *Mapper24) Compare(other Mapper) error {
    if !reflect.DeepEqual(mapper, other) {
        return fmt.Errorf("VRC6a mapper state differs")
    }
    return nil
}

func (mapper *Mapper24) Read(address uint16) byte {
    return mapper.ReadWithOpenBus(address, 0)
}

func (mapper *Mapper24) ReadWithOpenBus(address uint16, openBus byte) byte {
    var bank, bankSize int
    switch {
    case address >= 0xe000:
        bankSize = 0x2000
        bank = len(mapper.ProgramROM)/bankSize - 1
    case address >= 0xc000:
        bankSize, bank = 0x2000, int(mapper.ProgramBank8)
    case address >= 0x8000:
        bankSize, bank = 0x4000, int(mapper.ProgramBank16)
    case address >= 0x6000 && mapper.BankingMode&0x80 != 0:
        offset := int(address - 0x6000)
        if offset < len(mapper.WorkRAM) {
            return mapper.WorkRAM[offset]
        }
        return openBus
    default:
        return openBus
    }
    offset := mirrorBank(bank, len(mapper.ProgramROM)/bankSize)*bankSize + int(address)%bankSize
    if offset < 0 || offset >= len(mapper.ProgramROM) {
        return openBus
    }
    return mapper.ProgramROM[offset]
}

func (mapper *Mapper24) Write(_ *CPUState, address uint16, value byte) error {
    if address < 0x8000 {
        if address >= 0x6000 && mapper.BankingMode&0x80 != 0 {
            offset := int(address - 0x6000)
            if offset < len(mapper.WorkRAM) {
                mapper.WorkRAM[offset] = value
            }
        }
        return nil
    }
    register := address & 0xf003
    if mapper.Audio.HandleWrite(register, value) {
        return nil
    }
    switch register & 0xf000 {
    case 0x8000:
        mapper.ProgramBank16 = value & 0x0f
    case 0xc000:
        mapper.ProgramBank8 = value & 0x1f
    case 0xd000:
        mapper.CharacterBanks[register&3] = value
    case 0xe000:
        mapper.CharacterBanks[4+(register&3)] = value
    default:
        switch register {
        case 0xb003:
            mapper.BankingMode = value
        case 0xf000:
            mapper.IRQReload = value
        case 0xf001:
            mapper.IRQControl = value & 7
            mapper.IRQPending = false
            if value&2 != 0 {
                mapper.IRQCounter = mapper.IRQReload
                mapper.IRQPrescaler = 0
            }
        case 0xf002:
            mapper.IRQControl = (mapper.IRQControl &^ 2) | (mapper.IRQControl&1)<<1
            mapper.IRQPending = false
        }
    }
    return nil
}

func (mapper *Mapper24) ClockCPU(cycles uint64) {
    mapper.Audio.clockCPU(cycles)
    if mapper.IRQControl&2 == 0 {
        return
    }
    ticks := cycles
    if mapper.IRQControl&4 == 0 {
        // The internal prescaler counts 341 units at three units per CPU
        // cycle, independently of PPU rendering and video region.
        elapsed := mapper.IRQPrescaler + cycles*3
        ticks, mapper.IRQPrescaler = elapsed/341, elapsed%341
    }
    for range ticks {
        if mapper.IRQCounter == 0xff {
            mapper.IRQCounter = mapper.IRQReload
            mapper.IRQPending = true
        } else {
            mapper.IRQCounter++
        }
    }
}

func (mapper *Mapper24) patternBank(address uint16) byte {
    page := address >> 10 & 7
    register := page
    paired := false
    switch mapper.BankingMode & 3 {
    case 1:
        register, paired = page/2, true
    case 2, 3:
        if page >= 4 {
            register, paired = 4+(page-4)/2, true
        }
    }
    bank := mapper.CharacterBanks[register]
    if paired && mapper.BankingMode&0x20 != 0 {
        bank = bank & 0xfe | byte(page&1)
    }
    return bank
}

func (mapper *Mapper24) characterOffset(bank byte, address uint16) int {
    return mirrorBank(int(bank), len(mapper.CharacterMemory)/0x400)*0x400 + int(address&0x3ff)
}

func (mapper *Mapper24) readCharacterBank(bank byte, address uint16) byte {
    offset := mapper.characterOffset(bank, address)
    if offset < len(mapper.CharacterMemory) {
        return mapper.CharacterMemory[offset]
    }
    return 0
}

func (mapper *Mapper24) writeCharacterBank(bank byte, address uint16, value byte) {
    if mapper.CharacterRAM {
        offset := mapper.characterOffset(bank, address)
        if offset < len(mapper.CharacterMemory) {
            mapper.CharacterMemory[offset] = value
        }
    }
}

func (mapper *Mapper24) ReadCharacter(_ *PPUState, address uint16) byte {
    return mapper.readCharacterBank(mapper.patternBank(address), address)
}

func (mapper *Mapper24) WriteCharacter(_ *PPUState, address uint16, value byte) {
    mapper.writeCharacterBank(mapper.patternBank(address), address, value)
}

func (mapper *Mapper24) nametableBank(address uint16) byte {
    page := byte(address >> 10 & 3)
    switch mapper.BankingMode & 0x2f {
    case 0x20, 0x27:
        return mapper.CharacterBanks[6+page/2]&0xfe | page&1
    case 0x23, 0x24:
        return mapper.CharacterBanks[6+page%2]&0xfe | page>>1
    case 0x28, 0x2f:
        return mapper.CharacterBanks[6+page/2] & 0xfe
    case 0x2b, 0x2c:
        return mapper.CharacterBanks[6+page%2] | 1
    }
    switch mapper.BankingMode & 7 {
    case 0, 6, 7:
        return mapper.CharacterBanks[6+page/2]
    case 1, 5:
        return mapper.CharacterBanks[4+page]
    default:
        return mapper.CharacterBanks[6+page%2]
    }
}

func (mapper *Mapper24) ReadNametable(ppu *PPUState, address uint16) byte {
    bank := mapper.nametableBank(address)
    if mapper.BankingMode&0x10 != 0 {
        return mapper.readCharacterBank(bank, address)
    }
    return ppu.NametableMemory[uint16(bank&1)*0x400+(address&0x3ff)]
}

func (mapper *Mapper24) WriteNametable(ppu *PPUState, address uint16, value byte) {
    bank := mapper.nametableBank(address)
    if mapper.BankingMode&0x10 != 0 {
        mapper.writeCharacterBank(bank, address, value)
    } else {
        ppu.NametableMemory[uint16(bank&1)*0x400+(address&0x3ff)] = value
    }
}

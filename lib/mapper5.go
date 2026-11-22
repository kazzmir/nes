package lib

import (
	"fmt"
	"reflect"
)

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

func MakeMapper5(prg, chr []byte) *Mapper5 {
	return &Mapper5{ProgramROM: prg, CharacterROM: chr,
		WorkRAM: make([]byte, 0x10000), PRGMode: 3,
		PRGBanks: [5]byte{0, 0, 0, 0, 0xff}}
}

func (*Mapper5) Kind() int                         { return 5 }
func (*Mapper5) IsNSF() bool                       { return false }
func (*Mapper5) MapsExpansion(address uint16) bool { return address >= 0x5000 && address < 0x6000 }
func (m *Mapper5) IsIRQAsserted() bool             { return m.IRQEnabled && m.IRQPending }

func (m *Mapper5) Copy() Mapper {
	out := *m
	out.ProgramROM = copySlice(m.ProgramROM)
	out.CharacterROM = copySlice(m.CharacterROM)
	out.WorkRAM = copySlice(m.WorkRAM)
	return &out
}

func (m *Mapper5) Compare(other Mapper) error {
	if !reflect.DeepEqual(m, other) {
		return fmt.Errorf("MMC5 state differs")
	}
	return nil
}

func (m *Mapper5) prgAddress(address uint16) (rom bool, offset int) {
	if address < 0x8000 {
		return false, int(m.PRGBanks[0]&7)*0x2000 + int(address&0x1fff)
	}
	slot := int(address-0x8000) / 0x2000
	reg, span := slot+1, 1
	switch m.PRGMode {
	case 0:
		reg, span = 4, 4
	case 1:
		reg, span = 2+2*(slot/2), 2
	case 2:
		if slot < 2 {
			reg, span = 2, 2
		}
	}
	value := m.PRGBanks[reg]
	rom = reg == 4 || value&0x80 != 0
	bank := int(value & 7)
	if rom {
		bank = int(value & 0x7f)
	}
	bank = bank&^(span-1) | slot&(span-1)
	if rom {
		bank = mirrorBank(bank, len(m.ProgramROM)/0x2000)
	}
	return rom, bank*0x2000 + int(address&0x1fff)
}

func (m *Mapper5) Read(address uint16) byte { return m.ReadWithOpenBus(address, 0) }

func (m *Mapper5) ReadWithOpenBus(address uint16, openBus byte) byte {
	switch address {
	case 0xfffa, 0xfffb:
		// Fetching the NMI vector also resets the MMC5 scanline detector.
		m.InFrame, m.IRQPending, m.IRQLine = false, false, 0
	case 0x5204:
		var status byte
		if m.IRQPending {
			status |= 0x80
		}
		if m.InFrame {
			status |= 0x40
		}
		m.IRQPending = false
		return status
	case 0x5205:
		return byte(uint16(m.MultiplyA) * uint16(m.MultiplyB))
	case 0x5206:
		return byte(uint16(m.MultiplyA) * uint16(m.MultiplyB) >> 8)
	}
	if address >= 0x5c00 && address < 0x6000 && m.ExMode >= 2 {
		return m.ExRAM[address&0x3ff]
	}
	if address >= 0x6000 {
		rom, offset := m.prgAddress(address)
		data := m.WorkRAM
		if rom {
			data = m.ProgramROM
		}
		if offset < len(data) {
			return data[offset]
		}
	}
	return openBus
}

func (m *Mapper5) Write(_ *CPUState, address uint16, value byte) error {
	switch {
	case address >= 0x6000:
		rom, offset := m.prgAddress(address)
		if !rom && m.Protect1 == 2 && m.Protect2 == 1 && offset < len(m.WorkRAM) {
			m.WorkRAM[offset] = value
		}
	case address >= 0x5c00:
		if m.ExMode < 3 {
			if m.ExMode < 2 && !m.InFrame {
				value = 0
			}
			m.ExRAM[address&0x3ff] = value
		}
	case address >= 0x5113 && address <= 0x5117:
		m.PRGBanks[address-0x5113] = value
	case address >= 0x5120 && address <= 0x512b:
		m.CHRBanks[address-0x5120] = uint16(value) | uint16(m.CHRUpper)<<8
		m.LastCHRBackground = address >= 0x5128
	default:
		switch address {
		case 0x5100:
			m.PRGMode = value & 3
		case 0x5101:
			m.CHRMode = value & 3
		case 0x5102:
			m.Protect1 = value & 3
		case 0x5103:
			m.Protect2 = value & 3
		case 0x5104:
			m.ExMode = value & 3
		case 0x5105:
			m.Nametables = value
		case 0x5106:
			m.FillTile = value
		case 0x5107:
			m.FillColor = (value & 3) * 0x55
		case 0x5130:
			m.CHRUpper = value & 3
		case 0x5203:
			m.IRQTarget = value
		case 0x5204:
			m.IRQEnabled = value&0x80 != 0
		case 0x5205:
			m.MultiplyA = value
		case 0x5206:
			m.MultiplyB = value
		}
	}
	return nil
}

func (m *Mapper5) RenderingScanline(line int, rendering bool) {
	if !rendering || line >= 240 {
		m.InFrame = false
		return
	}
	if !m.InFrame || line == 0 {
		m.InFrame, m.IRQLine = true, 0
		return
	}
	m.IRQLine++
	if m.IRQLine == m.IRQTarget {
		m.IRQPending = true
	}
}

func (m *Mapper5) characterByte(bank, size, offset int) byte {
	bank = mirrorBank(bank, len(m.CharacterROM)/size)
	index := bank*size + offset
	if index >= len(m.CharacterROM) {
		return 0
	}
	return m.CharacterROM[index]
}

func (m *Mapper5) readCHR(address uint16, background bool) byte {
	size := 0x2000 >> m.CHRMode
	offset := int(address)
	reg := (offset/size+1)*(size/0x400) - 1
	if background {
		reg = 8 + reg%4
	}
	return m.characterByte(int(m.CHRBanks[reg]), size, offset%size)
}

func (m *Mapper5) ReadCharacter(ppu *PPUState, address uint16) byte {
	return m.readCHR(address, ppu.GetSpriteSize() == SpriteSize8x16 && m.LastCHRBackground)
}

func (m *Mapper5) ReadSpriteCharacter(_ *PPUState, address uint16) byte {
	return m.readCHR(address, false)
}

func (m *Mapper5) nametableSource(address uint16) byte {
	return (m.Nametables >> (((address >> 10) & 3) * 2)) & 3
}

func (m *Mapper5) ReadNametable(ppu *PPUState, address uint16) byte {
	offset := address & 0x3ff
	source := m.nametableSource(address)
	switch source {
	case 0, 1:
		return ppu.NametableMemory[uint16(source)*0x400+offset]
	case 2:
		if m.ExMode < 2 {
			return m.ExRAM[offset]
		}
	case 3:
		if offset >= 0x3c0 {
			return m.FillColor
		}
		return m.FillTile
	}
	return 0
}

func (m *Mapper5) WriteNametable(ppu *PPUState, address uint16, value byte) {
	offset := address & 0x3ff
	source := m.nametableSource(address)
	switch source {
	case 0, 1:
		ppu.NametableMemory[uint16(source)*0x400+offset] = value
	case 2:
		if m.ExMode < 2 {
			m.ExRAM[offset] = value
		}
	}
}

func (m *Mapper5) ReadBackgroundTile(ppu *PPUState, address, patternBase uint16, row byte) (byte, byte, byte) {
	tile := m.ReadNametable(ppu, address)
	if m.ExMode == 1 {
		attr := m.ExRAM[address&0x3ff]
		bank := int(attr&0x3f) | int(m.CHRUpper)<<6
		offset := int(tile)*16 + int(row)
		return m.characterByte(bank, 0x1000, offset), m.characterByte(bank, 0x1000, offset+8), attr >> 6
	}
	attrAddress := 0x23c0 | (address & 0xc00) | ((address >> 4) & 0x38) | ((address >> 2) & 7)
	shift := (address & 2) | ((address >> 4) & 4)
	palette := (m.ReadNametable(ppu, attrAddress) >> shift) & 3
	pattern := patternBase + uint16(tile)*16 + uint16(row)
	background := ppu.GetSpriteSize() == SpriteSize8x16
	return m.readCHR(pattern, background), m.readCHR(pattern+8, background), palette
}

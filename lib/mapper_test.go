package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// MMC1 CHR banking.

func writeMMC1Register(t *testing.T, cpu *CPUState, address uint16, value byte) {
	t.Helper()
	for bit := range 5 {
		// Ordinary serial writes are separated by at least one non-write cycle.
		cpu.Cycle += 2
		if err := cpu.Mapper.Mapper.Write(cpu, address, (value>>bit)&1); err != nil {
			t.Fatal(err)
		}
	}
}

func mmc1CHRCPU() (CPUState, []byte) {
	chr := make([]byte, 0x20000) // All 32 addressable 4 KiB CHR banks.
	for i := range chr {
		chr[i] = byte(i/0x1000) ^ byte(i)
	}
	cpu := StartupState()
	cpu.SetMapper(MakeMapper1(make([]byte, 0x8000), chr))
	return cpu, chr
}

func TestMMC1CHR4KiBOffsetArithmetic(t *testing.T) {
	for _, address := range []uint16{0xa000, 0xc000} {
		for bank := range byte(32) {
			t.Run(fmt.Sprintf("register_%04x_bank_%d", address, bank), func(t *testing.T) {
				cpu, chr := mmc1CHRCPU()
				writeMMC1Register(t, &cpu, 0x8000, 0x1c) // 4 KiB CHR mode.
				writeMMC1Register(t, &cpu, address, bank)
				base := 0
				if address == 0xc000 {
					base = 0x1000
				}
				start := int(bank) * 0x1000
				if !bytes.Equal(cpu.PPU.VideoMemory[base:base+0x1000], chr[start:start+0x1000]) {
					t.Fatalf("PPU $%04x does not contain CHR bank %d (ROM offset $%05x)", base, bank, start)
				}
			})
		}
	}
}

func TestMMC1Last4KiBCHRBank(t *testing.T) {
	for _, size := range []int{0x2000, 0x4000, 0x8000, 0x10000, 0x20000} {
		for _, address := range []uint16{0xa000, 0xc000} {
			t.Run(fmt.Sprintf("CHR_%d_KiB_register_%04x", size/1024, address), func(t *testing.T) {
				cpu, chr := mmc1CHRCPU()
				cpu.Cycle = 30000
				chr = chr[:size]
				cpu.SetMapper(MakeMapper1(make([]byte, 0x8000), chr))
				writeMMC1Register(t, &cpu, 0x8000, 0x1c)
				clear(cpu.PPU.VideoMemory[:0x2000])
				lastBank := byte(size/0x1000 - 1)
				writeMMC1Register(t, &cpu, address, lastBank)
				base := 0
				if address == 0xc000 {
					base = 0x1000
				}
				if !bytes.Equal(cpu.PPU.VideoMemory[base:base+0x1000], chr[size-0x1000:]) {
					t.Fatalf("last CHR bank %d was not mapped at PPU $%04x", lastBank, base)
				}
				for _, offset := range []int{0, 0xfff} {
					want := chr[size-0x1000+offset]
					if got := readPatternByte(&cpu, uint16(base+offset)); got != want {
						t.Fatalf("PPU $%04x: got $%02x, want $%02x", base+offset, got, want)
					}
				}
				other := base ^ 0x1000
				if !bytes.Equal(cpu.PPU.VideoMemory[other:other+0x1000], make([]byte, 0x1000)) {
					t.Fatal("bank write changed the other pattern table")
				}
			})
		}
	}
}

func TestMMC1CHR8KiBOffsetArithmetic(t *testing.T) {
	// Exercise every register value, including odd values (bit 0 is ignored),
	// the bank ending exactly at 64 KiB, and banks above the 64 KiB boundary.
	for bank := range byte(32) {
		t.Run(fmt.Sprintf("register_%d", bank), func(t *testing.T) {
			cpu, chr := mmc1CHRCPU()
			writeMMC1Register(t, &cpu, 0x8000, 0x0c) // 8 KiB CHR mode.
			writeMMC1Register(t, &cpu, 0xa000, bank)
			start := int(bank&0x1e) * 0x1000
			if !bytes.Equal(cpu.PPU.VideoMemory[:0x2000], chr[start:start+0x2000]) {
				t.Fatalf("PPU does not contain CHR pair %d/%d (ROM offset $%05x)", bank&0x1e, bank|1, start)
			}
		})
	}
}

func TestMMC1ControlCHRRemapping(t *testing.T) {
	for _, banks := range [][2]byte{{0, 31}, {5, 2}, {31, 0}, {16, 7}} {
		t.Run(fmt.Sprintf("CHR0_%d_CHR1_%d", banks[0], banks[1]), func(t *testing.T) {
			cpu, chr := mmc1CHRCPU()
			cpu.Cycle = 30000
			cpu.PPU.CopyCharacterRom(0, chr[:0x2000], true)
			writeMMC1Register(t, &cpu, 0x8000, 0x0c) // Start in 8 KiB mode.
			writeMMC1Register(t, &cpu, 0xa000, banks[0])
			writeMMC1Register(t, &cpu, 0xc000, banks[1]) // Stored but inactive.
			assertBanks := func(want [2]byte) {
				t.Helper()
				for table, bank := range want {
					base, start := table*0x1000, int(bank)*0x1000
					if !bytes.Equal(cpu.PPU.VideoMemory[base:base+0x1000], chr[start:start+0x1000]) {
						t.Fatalf("pattern table %d does not map CHR bank %d", table, bank)
					}
					for _, offset := range []int{0, 0xfff} {
						if got := readPatternByte(&cpu, uint16(base+offset)); got != chr[start+offset] {
							t.Fatalf("PPU read $%04x did not use CHR bank %d", base+offset, bank)
						}
					}
				}
			}
			pair := [2]byte{banks[0] & 0x1e, banks[0] | 1}
			assertBanks(pair)
			before := bytes.Clone(cpu.PPU.VideoMemory[:0x2000])
			const control byte = 0x1c
			for bit := range 4 {
				cpu.Cycle += 2
				cpu.StoreMemory(0x8000, (control>>bit)&1)
				if !bytes.Equal(cpu.PPU.VideoMemory[:0x2000], before) {
					t.Fatal("partial control write changed CHR mapping")
				}
			}
			cpu.Cycle += 2
			cpu.StoreMemory(0x8000, control>>4) // Commit 4 KiB mode.
			assertBanks(banks)
			mapper := cpu.Mapper.Mapper.(*Mapper1)
			if mapper.ChrRegister0 != banks[0] || mapper.ChrRegister1 != banks[1] {
				t.Fatal("control write changed stored CHR registers")
			}
			writeMMC1Register(t, &cpu, 0x8000, 0x0c) // Return to 8 KiB mode.
			assertBanks(pair)
			writeMMC1Register(t, &cpu, 0x8000, 0x1c)
			assertBanks(banks) // No CHR register rewrites are needed in either direction.
		})
	}
}

func TestMMC1ControlCHRRemappingPreservesRAM(t *testing.T) {
	cpu := mmc1RAMCPU(0x8000)
	writePatternByte(&cpu, 0x0123, 0x36)
	writePatternByte(&cpu, 0x1123, 0x9c)
	original := bytes.Clone(cpu.PPU.VideoMemory[:0x2000])
	writeMMC1Register(t, &cpu, 0xa000, 1)
	writeMMC1Register(t, &cpu, 0xc000, 0)
	for control := range byte(32) {
		writeMMC1Register(t, &cpu, 0x8000, control)
		lower, upper := byte(0x36), byte(0x9c)
		if control&0x10 != 0 {
			lower, upper = upper, lower
		}
		if readPatternByte(&cpu, 0x0123) != lower || readPatternByte(&cpu, 0x1123) != upper {
			t.Fatalf("control $%02x did not apply CHR-RAM mapping", control)
		}
		if !bytes.Equal(cpu.PPU.VideoMemory[:0x2000], original) {
			t.Fatalf("control $%02x overwrote physical CHR-RAM", control)
		}
	}
}

func mmc1RAMCPU(prgSize int) CPUState {
	prg := make([]byte, prgSize)
	for i := range prg {
		prg[i] = byte(i / 0x4000)
	}
	cpu := StartupState()
	cpu.Cycle = 30000
	cpu.SetMapper(MakeMapper1(prg, nil))
	return cpu
}

func setPatternAddress(cpu *CPUState, address uint16) {
	cpu.LoadMemory(0x2002)
	cpu.StoreMemory(0x2006, byte(address>>8))
	cpu.StoreMemory(0x2006, byte(address))
}

func writePatternByte(cpu *CPUState, address uint16, value byte) {
	setPatternAddress(cpu, address)
	cpu.StoreMemory(0x2007, value)
}

func readPatternByte(cpu *CPUState, address uint16) byte {
	setPatternAddress(cpu, address)
	cpu.LoadMemory(0x2007) // Discard the previous buffered byte.
	// Re-address before returning the buffer so a read at $3EFF does not
	// advance into palette space, whose reads bypass the buffer.
	setPatternAddress(cpu, address)
	return cpu.LoadMemory(0x2007)
}

func TestMMC1CHRRAMRegisterWritesPreserveRAM(t *testing.T) {
	for _, mode := range []byte{0x0c, 0x1c} {
		t.Run(fmt.Sprintf("control_%02x", mode), func(t *testing.T) {
			cpu := mmc1RAMCPU(0x8000)
			writePatternByte(&cpu, 0x0123, 0x36)
			writePatternByte(&cpu, 0x1123, 0x9c)
			original := bytes.Clone(cpu.PPU.VideoMemory[:0x2000])
			writeMMC1Register(t, &cpu, 0x8000, mode)
			for bank := range byte(32) {
				writeMMC1Register(t, &cpu, 0xa000, bank)
				writeMMC1Register(t, &cpu, 0xc000, bank^1)
				lower, upper := byte(0x36), byte(0x9c)
				if mode == 0x1c && bank&1 != 0 {
					lower, upper = upper, lower
				}
				if got := readPatternByte(&cpu, 0x0123); got != lower {
					t.Fatalf("CHR0=$%02x: lower RAM got $%02x, want $%02x", bank, got, lower)
				}
				if got := readPatternByte(&cpu, 0x1123); got != upper {
					t.Fatalf("CHR1=$%02x: upper RAM got $%02x, want $%02x", bank^1, got, upper)
				}
				if !bytes.Equal(cpu.PPU.VideoMemory[:0x2000], original) {
					t.Fatalf("CHR register write $%02x changed physical CHR-RAM", bank)
				}
			}
		})
	}
}

func TestMMC1CHRRAMSwappingAndAliasing(t *testing.T) {
	cpu := mmc1RAMCPU(0x8000)
	writePatternByte(&cpu, 0x0123, 0x36)
	writePatternByte(&cpu, 0x1123, 0x9c)
	writeMMC1Register(t, &cpu, 0x8000, 0x1c)
	writeMMC1Register(t, &cpu, 0xa000, 1)
	writeMMC1Register(t, &cpu, 0xc000, 0)
	writePatternByte(&cpu, 0x0123, 0x57) // Write physical RAM bank 1.
	writeMMC1Register(t, &cpu, 0xc000, 1)
	if got := readPatternByte(&cpu, 0x1123); got != 0x57 {
		t.Fatalf("aliased upper table got $%02x, want $57", got)
	}
	writePatternByte(&cpu, 0x1123, 0x68)
	if got := readPatternByte(&cpu, 0x0123); got != 0x68 {
		t.Fatalf("aliased lower table got $%02x, want $68", got)
	}
	writeMMC1Register(t, &cpu, 0x8000, 0x0c) // Return to fixed 8 KiB mapping.
	if readPatternByte(&cpu, 0x0123) != 0x36 || readPatternByte(&cpu, 0x1123) != 0x68 {
		t.Fatal("switching CHR modes did not preserve both physical RAM banks")
	}
}

func TestMMC1CHRRAMSUROM(t *testing.T) {
	cpu := mmc1RAMCPU(0x80000)
	writePatternByte(&cpu, 0x0123, 0x36)
	for _, control := range []byte{0x0c, 0x1c} {
		writeMMC1Register(t, &cpu, 0x8000, control)
		for _, bank := range []byte{0x10, 0} {
			writeMMC1Register(t, &cpu, 0xa000, bank)
			if got := cpu.LoadMemory(0x8000); got != bank {
				t.Fatalf("outer PRG bank: got %d, want %d", got, bank)
			}
			if got := readPatternByte(&cpu, 0x0123); got != 0x36 {
				t.Fatal("SUROM outer-bank selection corrupted CHR-RAM")
			}
		}
	}
}

func TestMMC1CHRROMWritesIgnored(t *testing.T) {
	cpu, chr := mmc1CHRCPU()
	cpu.Cycle = 30000
	cpu.PPU.CopyCharacterRom(0, chr[:0x2000], true)
	before := readPatternByte(&cpu, 0x0123)
	writePatternByte(&cpu, 0x0123, before^0xff)
	if got := readPatternByte(&cpu, 0x0123); got != before {
		t.Fatal("CHR-RAM write routing made CHR-ROM writable")
	}
}

func TestMMC1CHRRAMCopyAndRestore(t *testing.T) {
	cpu := mmc1RAMCPU(0x8000)
	writePatternByte(&cpu, 0x1123, 0x9c)
	writeMMC1Register(t, &cpu, 0x8000, 0x1c)
	writeMMC1Register(t, &cpu, 0xa000, 1)
	copied := cpu.Copy()
	if got := readPatternByte(&copied, 0x0123); got != 0x9c {
		t.Fatal("copy lost CHR-RAM or its bank mapping")
	}
	writePatternByte(&copied, 0x0123, 0x57)
	if got := readPatternByte(&cpu, 0x0123); got != 0x9c {
		t.Fatal("copy shares CHR-RAM with the original")
	}
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	if got := readPatternByte(&restored, 0x0123); got != 0x9c {
		t.Fatal("save state lost CHR-RAM or its bank mapping")
	}
	writePatternByte(&restored, 0x0123, 0x68)
	if got := readPatternByte(&restored, 0x0123); got != 0x68 {
		t.Fatal("restored CHR-RAM is not writable")
	}
}

func TestMMC1SerialReset(t *testing.T) {
	for control := range byte(32) {
		for _, address := range []uint16{0x8000, 0xbfff, 0xdfff, 0xffff} {
			for _, value := range []byte{0x80, 0xff} {
				t.Run(fmt.Sprintf("control_%02x_address_%04x_value_%02x", control, address, value), func(t *testing.T) {
					cpu := mmc1RAMCPU(0x40000)
					writeMMC1Register(t, &cpu, 0xa000, 6)
					writeMMC1Register(t, &cpu, 0xc000, 11)
					writeMMC1Register(t, &cpu, 0xe000, 3)
					writeMMC1Register(t, &cpu, 0x8000, control)
					mirror := cpu.PPU.NametableMirror
					// Leave an incomplete serial write that reset must discard.
					for _, bit := range []byte{1, 0, 1} {
						cpu.Cycle += 2
						cpu.StoreMemory(0xe000, bit)
					}
					cpu.StoreMemory(address, value)
					mapper := cpu.Mapper.Mapper.(*Mapper1)
					if mapper.Shift != 0 || mapper.Register != 0 {
						t.Fatal("reset did not clear the partial serial write")
					}
					if mapper.PrgBankMode != 3 {
						t.Fatalf("reset left PRG mode %d, want 3", mapper.PrgBankMode)
					}
					if mapper.Mirror != control&3 || mapper.ChrBankMode != control>>4 || cpu.PPU.NametableMirror != mirror {
						t.Fatal("reset changed mirroring or CHR mode instead of only control bits 2-3")
					}
					if mapper.PrgBank != 3 || mapper.ChrRegister0 != 6 || mapper.ChrRegister1 != 11 {
						t.Fatal("reset changed a PRG/CHR bank register")
					}
					if cpu.LoadMemory(0x8000) != 3 || cpu.LoadMemory(0xc000) != 15 {
						t.Fatal("reset did not immediately map the selected/last 16 KiB PRG banks")
					}
				})
			}
		}
	}
}

func TestMMC1SerialWriteAfterReset(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	writeMMC1Register(t, &cpu, 0xe000, 3)
	for _, bit := range []byte{1, 0, 1} {
		cpu.Cycle += 2
		cpu.StoreMemory(0x8000, bit)
	}
	cpu.StoreMemory(0xbfff, 0xff)
	mapper := cpu.Mapper.Mapper.(*Mapper1)
	const nextBank byte = 6
	for bit := range 4 {
		cpu.Cycle += 2
		cpu.StoreMemory(0xe000, (nextBank>>bit)&1)
		if mapper.PrgBank != 3 {
			t.Fatal("bank committed before five fresh bits after reset")
		}
	}
	cpu.Cycle += 2
	cpu.StoreMemory(0xe000, nextBank>>4)
	if mapper.Shift != 0 || mapper.Register != 0 || mapper.PrgBank != nextBank {
		t.Fatal("serial writes did not restart cleanly after reset")
	}
	// A later full control write can still leave the reset-selected PRG mode.
	writeMMC1Register(t, &cpu, 0x8000, 0x08)
	if mapper.PrgBankMode != 2 {
		t.Fatal("control write could not change PRG mode after reset")
	}
}

func TestMMC1ConsecutiveSerialWrites(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	mapper := cpu.Mapper.Mapper.(*Mapper1)
	for _, step := range []struct {
		cycle   uint64
		address uint16
		value   byte
		shift   int
		bits    byte
	}{
		{0, 0x8000, 1, 1, 1},    // The first write at cycle zero must be accepted.
		{0, 0xa000, 0, 1, 1},    // CPU RMW writes share an instruction timestamp.
		{1, 0xc000, 0, 1, 1},    // Filtering is global, not per register address.
		{2, 0xe000, 0, 1, 1},    // Ignored writes still extend the inhibit window.
		{4, 0xe000, 1, 2, 3},    // A gap of two cycles permits the next bit.
		{5, 0xbfff, 0x80, 0, 0}, // Reset writes are honored even consecutively.
		{6, 0xe000, 1, 0, 0},    // The data write following reset is ignored.
		{8, 0xe000, 1, 1, 1},
	} {
		cpu.Cycle = step.cycle
		cpu.StoreMemory(step.address, step.value)
		if mapper.Shift != step.shift || mapper.Register != step.bits {
			t.Fatalf("cycle %d write $%02x to $%04x: shift/bits %d/$%02x, want %d/$%02x", step.cycle, step.value, step.address, mapper.Shift, mapper.Register, step.shift, step.bits)
		}
	}
	// PRG-RAM writes must not start or extend the register inhibit window.
	cpu.Cycle = 10
	cpu.StoreMemory(0x6000, 0x80)
	cpu.Cycle = 11
	cpu.StoreMemory(0xe000, 0)
	if mapper.Shift != 2 || mapper.Register != 1 || cpu.LoadMemory(0x6000) != 0x80 {
		t.Fatal("PRG-RAM write interfered with serial register timing")
	}
}

func TestMMC1RMWSerialWrites(t *testing.T) {
	for _, opcode := range []byte{
		0x0e, 0x4e, 0x2e, 0x6e, 0xce, 0xee, // ASL/LSR/ROL/ROR/DEC/INC absolute.
		0x1e, 0x5e, 0x3e, 0x7e, 0xde, 0xfe, // Same operations, absolute,X.
		0x0f, 0x4f, 0x2f, 0x6f, 0xcf, 0xef, // SLO/SRE/RLA/RRA/DCP/ISC absolute.
		0x1f, 0x5f, 0x3f, 0x7f, 0xdf, 0xff, // Same operations, absolute,X.
	} {
		t.Run(fmt.Sprintf("opcode_%02x", opcode), func(t *testing.T) {
			cpu := mmc1RAMCPU(0x40000)
			mapper := cpu.Mapper.Mapper.(*Mapper1)
			mapper.BankMemory[0x3e000] = 1 // ROM byte read at $E000.
			cpu.PC = 0x0200
			operands := []byte{0x00, 0xe0}
			if opcode&0x10 != 0 {
				cpu.X = 1
				operands = []byte{0xff, 0xdf}
			}
			copy(cpu.Ram[0x0200:], append([]byte{opcode}, operands...))
			before := cpu.Cycle
			if err := cpu.Run(MakeInstructionDescriptiontable()); err != nil {
				t.Fatal(err)
			}
			if mapper.Shift != 1 || mapper.Register != 1 {
				t.Fatalf("RMW shifted more than its dummy write: shift=%d bits=$%02x", mapper.Shift, mapper.Register)
			}
			wantCycles := uint64(6)
			if opcode&0x10 != 0 {
				wantCycles = 7
			}
			if cpu.Cycle-before != wantCycles {
				t.Fatal("mapper filtering changed CPU instruction cycle count")
			}
		})
	}
}

func TestMMC1RMWResetWrites(t *testing.T) {
	for _, value := range []byte{0x7f, 0xff} {
		t.Run(fmt.Sprintf("INC_ROM_%02x", value), func(t *testing.T) {
			cpu := mmc1RAMCPU(0x40000)
			mapper := cpu.Mapper.Mapper.(*Mapper1)
			mapper.BankMemory[0x3e000] = value
			cpu.PC = 0x0200
			copy(cpu.Ram[0x0200:], []byte{0xee, 0x00, 0xe0}) // INC $E000.
			if err := cpu.Run(MakeInstructionDescriptiontable()); err != nil {
				t.Fatal(err)
			}
			if mapper.Shift != 0 || mapper.Register != 0 || mapper.PrgBankMode != 3 {
				t.Fatal("RMW reset was ignored or its following data write was accepted")
			}
		})
	}
}

func TestMMC1NormalCPUSerialWrites(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	cpu.PC = 0x0200
	var code []byte
	for _, bit := range []byte{0, 1, 1, 0, 0} {
		code = append(code, 0xa9, bit, 0x8d, 0x00, 0xe0) // LDA #bit; STA $E000.
	}
	copy(cpu.Ram[0x0200:], code)
	table := MakeInstructionDescriptiontable()
	for range 10 {
		if err := cpu.Run(table); err != nil {
			t.Fatal(err)
		}
	}
	mapper := cpu.Mapper.Mapper.(*Mapper1)
	if mapper.Shift != 0 || mapper.PrgBank != 6 {
		t.Fatal("ordinary CPU serial writes were incorrectly filtered")
	}
}

func TestMMC1SerialWriteTimingCopyAndRestore(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	cpu.Cycle = 100
	cpu.StoreMemory(0xe000, 1)
	copied := cpu.Copy()
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	for _, state := range []*CPUState{&copied, &restored} {
		state.Cycle = 101
		state.StoreMemory(0xe000, 0)
		mapper := state.Mapper.Mapper.(*Mapper1)
		if mapper.Shift != 1 || mapper.Register != 1 {
			t.Fatal("copy/save state lost the consecutive-write inhibit window")
		}
		state.Cycle = 103
		state.StoreMemory(0xe000, 0)
		if mapper.Shift != 2 || mapper.Register != 1 {
			t.Fatal("copy/save state did not accept a write after the inhibit window")
		}
	}
}

func TestMMC1PRGRAMDisable(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	mapper := cpu.Mapper.Mapper.(*Mapper1)
	addresses := []uint16{0x6000, 0x6001, 0x7ffe, 0x7fff}
	for i, address := range addresses {
		cpu.StoreMemory(address, byte(0x40+i)) // RAM is enabled at power-on.
	}
	original := bytes.Clone(mapper.PRGRam)
	writeMMC1Register(t, &cpu, 0xe000, 0x13) // Disable RAM, select PRG bank 3.
	for i, address := range addresses {
		cpu.Databus = byte(0xa0 + i)
		if got := cpu.LoadMemory(address); got != byte(0xa0+i) {
			t.Fatalf("disabled RAM read $%04x: got $%02x, want open bus $%02x", address, got, byte(0xa0+i))
		}
		cpu.StoreMemory(address, byte(0xe0+i))
		if got := cpu.LoadMemory(address); got != byte(0xe0+i) {
			t.Fatal("ignored RAM write did not leave its value on the CPU bus")
		}
	}
	if !bytes.Equal(mapper.PRGRam, original) {
		t.Fatal("writes to disabled PRG-RAM modified its contents")
	}
	writeMMC1Register(t, &cpu, 0xe000, 3)
	for i, address := range addresses {
		if got := cpu.LoadMemory(address); got != byte(0x40+i) {
			t.Fatal("reenabling PRG-RAM did not restore access to its contents")
		}
		cpu.StoreMemory(address, byte(0x50+i))
		if cpu.LoadMemory(address) != byte(0x50+i) {
			t.Fatal("reenabled PRG-RAM is not writable")
		}
	}
}

func TestMMC1PRGRAMDisableCommitsOnFifthWrite(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	for bit := range 4 {
		cpu.Cycle += 2
		cpu.StoreMemory(0xe000, 0) // First four bits of $10.
		cpu.StoreMemory(0x6000, byte(0x20+bit))
		if cpu.LoadMemory(0x6000) != byte(0x20+bit) {
			t.Fatal("incomplete serial write disabled PRG-RAM")
		}
	}
	cpu.Cycle += 2
	cpu.StoreMemory(0xffff, 1) // The fifth bit commits the disable flag.
	cpu.StoreMemory(0x6000, 0xff)
	for range 4 {
		cpu.Cycle += 2
		cpu.StoreMemory(0xe000, 0)
		cpu.Databus = 0xa5
		if cpu.LoadMemory(0x6000) != 0xa5 {
			t.Fatal("incomplete serial write reenabled PRG-RAM")
		}
	}
	cpu.Cycle += 2
	cpu.StoreMemory(0xe000, 0)
	if cpu.LoadMemory(0x6000) != 0x23 {
		t.Fatal("RAM disable did not take effect on the fifth bit or lost RAM data")
	}
}

func TestMMC1PRGRAMDisableDoesNotSelectPRGBank(t *testing.T) {
	for _, pages := range []int{2, 4, 8, 16, 32} {
		for mode := range byte(4) {
			t.Run(fmt.Sprintf("PRG_%d_KiB_mode_%d", pages*16, mode), func(t *testing.T) {
				cpu := mmc1RAMCPU(pages * 0x4000)
				mapper := cpu.Mapper.Mapper.(*Mapper1)
				writeMMC1Register(t, &cpu, 0x8000, mode<<2)
				for _, outer := range []byte{0, 0x10} {
					writeMMC1Register(t, &cpu, 0xa000, outer)
					for bank := range byte(16) {
						writeMMC1Register(t, &cpu, 0xe000, bank)
						var want [4]byte
						addresses := [4]uint16{0x8000, 0xbfff, 0xc000, 0xffff}
						for i, address := range addresses {
							want[i] = cpu.LoadMemory(address)
						}
						writeMMC1Register(t, &cpu, 0xe000, bank|0x10)
						if mapper.PrgBank != bank|0x10 {
							t.Fatal("PRG register did not retain the RAM-disable bit")
						}
						for i, address := range addresses {
							if got := cpu.LoadMemory(address); got != want[i] {
								t.Fatalf("bank $%02x outer $%02x at $%04x: disable changed PRG byte from $%02x to $%02x", bank, outer, address, want[i], got)
							}
						}
					}
				}
			})
		}
	}
}

func TestMMC1PRGRAMDisableSurvivesResetAndControlWrites(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	cpu.StoreMemory(0x6000, 0x42)
	writeMMC1Register(t, &cpu, 0xe000, 0x15)
	cpu.StoreMemory(0x8000, 0x80)
	writeMMC1Register(t, &cpu, 0x8000, 0x08)
	writeMMC1Register(t, &cpu, 0xa000, 0)
	cpu.Databus = 0xa5
	if cpu.LoadMemory(0x6000) != 0xa5 {
		t.Fatal("serial reset or an unrelated register write reenabled PRG-RAM")
	}
	writeMMC1Register(t, &cpu, 0xe000, 5)
	if cpu.LoadMemory(0x6000) != 0x42 {
		t.Fatal("RAM contents did not survive reset/control writes while disabled")
	}
}

func TestMMC1PRGRAMDisableCopyAndRestore(t *testing.T) {
	cpu := mmc1RAMCPU(0x40000)
	cpu.StoreMemory(0x6000, 0x42)
	writeMMC1Register(t, &cpu, 0xe000, 0x13)
	copied := cpu.Copy()
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	for _, state := range []*CPUState{&copied, &restored} {
		state.Databus = 0xa5
		if state.LoadMemory(0x6000) != 0xa5 {
			t.Fatal("copy/save state lost the PRG-RAM disable flag")
		}
		state.StoreMemory(0x6000, 0xff)
		writeMMC1Register(t, state, 0xe000, 3)
		if state.LoadMemory(0x6000) != 0x42 {
			t.Fatal("copy/save state lost protected PRG-RAM contents")
		}
		state.StoreMemory(0x6000, 0x57)
	}
	if cpu.Mapper.Mapper.(*Mapper1).PRGRam[0] != 0x42 {
		t.Fatal("copy/save state shares PRG-RAM with the original")
	}
}

type openBusMapperStub struct {
	Mapper
}

func (*openBusMapperStub) ReadWithOpenBus(address uint16, openBus byte) byte {
	return openBus ^ byte(address)
}

func TestCPUOptionalOpenBusMapper(t *testing.T) {
	cpu := StartupState()
	cpu.SetMapper(&openBusMapperStub{Mapper: MakeMapper0(make([]byte, 0x8000))})
	cpu.Databus = 0xa5
	if got := cpu.LoadMemory(0x6001); got != 0xa4 || cpu.Databus != 0xa4 {
		t.Fatal("CPU did not pass the current data bus/address to the optional mapper read")
	}
	if got := cpu.LoadMemory(0x6000); got != 0xa4 {
		t.Fatal("optional mapper read did not receive the updated data bus")
	}
	cpu.SetMapper(MakeMapper0(make([]byte, 0x8000)))
	cpu.StoreMemory(0x6001, 0x36)
	cpu.Databus = 0xa5
	if cpu.LoadMemory(0x6001) != 0x36 {
		t.Fatal("mapper without open-bus capability did not use normal reads")
	}
}

func TestMMC1SUROMPRGBanking(t *testing.T) {
	for _, pages := range []int{16, 32} { // 256/512 KiB PRG-ROM.
		for prgMode := range byte(4) {
			for chrMode := range byte(2) {
				t.Run(fmt.Sprintf("PRG_%d_KiB_PRG_mode_%d_CHR_mode_%d", pages*16, prgMode, chrMode), func(t *testing.T) {
					cpu := mmc1RAMCPU(pages * 0x4000)
					cpu.StoreMemory(0x6000, 0x42)
					writeMMC1Register(t, &cpu, 0x8000, chrMode<<4|prgMode<<2)
					for _, outer := range []byte{0, 0x10} {
						// In 4 KiB CHR mode, software must keep both outer bits
						// equal. The other CHR bits do not select PRG banks.
						writeMMC1Register(t, &cpu, 0xa000, outer|0x0b)
						writeMMC1Register(t, &cpu, 0xc000, outer|0x04)
						for register := range byte(32) {
							writeMMC1Register(t, &cpu, 0xe000, register)
							bank := int(register & 0x0f)
							var banks [2]int
							switch prgMode {
							case 0, 1:
								banks[0] = int(outer) + (bank & 0x0e)
								banks[1] = banks[0] + 1
							case 2:
								banks = [2]int{int(outer), int(outer) + bank}
							case 3:
								banks = [2]int{int(outer) + bank, int(outer) + 15}
							}
							for _, address := range []uint16{0x8000, 0x8001, 0xbfff, 0xc000, 0xc001, 0xfffa, 0xfffc, 0xffff} {
								want := byte(banks[(address-0x8000)/0x4000] % pages)
								if got := cpu.LoadMemory(address); got != want {
									t.Fatalf("outer=$%02x register=$%02x read $%04x: got bank %d, want %d", outer, register, address, got, want)
								}
							}
							cpu.Databus = 0xa5
							wantRAM := byte(0x42)
							if register&0x10 != 0 {
								wantRAM = 0xa5
							}
							if cpu.LoadMemory(0x6000) != wantRAM {
								t.Fatal("SUROM outer-bank bit interfered with PRG-RAM disable")
							}
						}
					}
				})
			}
		}
	}
}

func TestMMC1SUROM32KiBOuterBankCopyAndRestore(t *testing.T) {
	cpu := mmc1RAMCPU(0x80000)
	writeMMC1Register(t, &cpu, 0x8000, 0x0c)
	writeMMC1Register(t, &cpu, 0xe000, 3) // Bit 0 is ignored in 32 KiB mode.
	writeMMC1Register(t, &cpu, 0x8000, 0) // Switch to 32 KiB mode.
	for _, outer := range []byte{0, 0x10, 0, 0x10} {
		writeMMC1Register(t, &cpu, 0xa000, outer)
		if cpu.LoadMemory(0x8000) != outer+2 || cpu.LoadMemory(0xc000) != outer+3 {
			t.Fatal("CHR0 outer-bank change did not immediately update both PRG windows")
		}
	}
	copied := cpu.Copy()
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	for _, state := range []*CPUState{&copied, &restored} {
		if state.LoadMemory(0x8000) != 18 || state.LoadMemory(0xffff) != 19 {
			t.Fatal("copy/save state lost upper-half 32 KiB PRG mapping")
		}
		writeMMC1Register(t, state, 0xa000, 0)
		if state.LoadMemory(0x8000) != 2 || state.LoadMemory(0xffff) != 3 {
			t.Fatal("copy/save state could not switch back to the lower ROM half")
		}
	}
	if cpu.LoadMemory(0x8000) != 18 {
		t.Fatal("copy/save state changed the original outer bank")
	}
}

// MMC2 latches and rendering fetches.

func mmc2CPU(t *testing.T) CPUState {
	t.Helper()
	chr := make([]byte, 4*0x1000)
	for i := range chr {
		chr[i] = byte(0x10 + i/0x1000)
	}
	cpu := StartupState()
	cpu.Cycle = 30000
	cpu.SetMapper(MakeMapper9(make([]byte, 0x20000), chr))
	for i := range 4 {
		cpu.StoreMemory(uint16(0xb000+i*0x1000), byte(i))
	}
	return cpu
}

func TestMMC2TriggerReadUsesOldBank(t *testing.T) {
	cpu := mmc2CPU(t)
	for _, step := range []struct {
		address uint16
		oldBank byte
		newBank byte
	}{
		{0x0fd8, 0x11, 0x10},
		{0x0fe8, 0x10, 0x11},
		{0x1fd8, 0x13, 0x12},
		{0x1fe8, 0x12, 0x13},
	} {
		if got := cpu.PPU.readPatternMemory(step.address); got != step.oldBank {
			t.Errorf("trigger $%04x returned $%02x, want old bank byte $%02x", step.address, got, step.oldBank)
		}
		if got := cpu.PPU.readPatternMemory(step.address & 0x1000); got != step.newBank {
			t.Errorf("after trigger $%04x: got $%02x, want $%02x", step.address, got, step.newBank)
		}
	}
}

func TestMMC2RenderingFetches(t *testing.T) {
	cpu := mmc2CPU(t)
	cpu.PPU.StoreNametableMemory(0x2000, 0xfd)
	cpu.PPU.VideoAddress = 0
	cpu.PPU.LoadBackgroundTile() // The high plane at $0FD8 selects lower FD.
	mapper := cpu.Mapper.Mapper.(*Mapper9)
	if mapper.Latches[0] != 0xfd {
		t.Fatal("background fetch did not select FD")
	}

	cpu.PPU.Flags = 8 // 8x8 sprites in the upper pattern table.
	cpu.PPU.Scanline = 1
	cpu.PPU.CurrentSprites = []Sprite{
		{Tile: 0xfd, Y: 1, X: 255},
		{Tile: 0, Y: 1},
	}
	cpu.PPU.fetchSpritePatterns()
	if mapper.Latches[1] != 0xfd {
		t.Fatal("sprite fetch did not select FD")
	}
	first, second := cpu.PPU.CurrentSprites[0], cpu.PPU.CurrentSprites[1]
	if first.Pattern[0] != 0x13 || first.Pattern[8] != 0x13 {
		t.Fatal("triggering sprite row did not retain old-bank pattern bytes")
	}
	if second.Pattern[0] != 0x12 || second.Pattern[8] != 0x12 {
		t.Fatal("following sprite did not use the new bank")
	}
	left, right := cpu.PPU.spritePatternBytes(&first, 0x1fd0)
	if left[0] != 0x13 || right[0] != 0x13 {
		t.Fatal("pixel reads lost the cached old-bank row")
	}
}

func TestMMC2CopyAndSaveState(t *testing.T) {
	cpu := mmc2CPU(t)
	cpu.PPU.readPatternMemory(0x0fd8)
	cpu.PPU.readPatternMemory(0x1fd8)
	copyCPU := cpu.Copy()
	if copyCPU.PPU.characterMapper == cpu.PPU.characterMapper {
		t.Fatal("copy shares the original mapper binding")
	}
	copyCPU.PPU.readPatternMemory(0x0fe8)
	if cpu.Mapper.Mapper.(*Mapper9).Latches[0] != 0xfd {
		t.Fatal("copy's latch change affected original")
	}

	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Mapper.Mapper.(*Mapper9).Latches != [2]byte{0xfd, 0xfd} {
		t.Fatal("save state lost latch selections")
	}
	// Restore must rebind the PPU before any register reads or rendering.
	restored.Restore()
	if restored.PPU.characterMapper != restored.Mapper.Mapper.(*Mapper9) {
		t.Fatal("Restore did not bind the PPU to the restored mapper")
	}
	restored.LoadMemory(0x2002)
	restored.StoreMemory(0x2006, 0x0f)
	restored.StoreMemory(0x2006, 0xe8)
	restored.LoadMemory(0x2007)
	if restored.Mapper.Mapper.(*Mapper9).Latches[0] != 0xfe {
		t.Fatal("restored CPU's $2007 read did not update its mapper")
	}
}

// Disconnected bank address lines across cartridge types.

func bankTestROM(pages int, pageSize int) []byte {
	memory := make([]byte, pages*pageSize)
	for i := range memory {
		memory[i] = byte(i/pageSize+1) ^ byte(i)
	}
	return memory
}

func TestDiscreteAndMMC2PRGBankAddressLines(t *testing.T) {
	for _, test := range []struct {
		mapper   uint32
		pageSize int
		pages    []int
		mask     byte
	}{
		{2, 0x4000, []int{2, 4, 8, 16}, 0xff},
		{7, 0x8000, []int{1, 2, 4, 8, 16}, 0x0f},
		{9, 0x2000, []int{4, 8, 16}, 0x0f},
	} {
		for _, pages := range test.pages {
			t.Run(fmt.Sprintf("mapper_%d_pages_%d", test.mapper, pages), func(t *testing.T) {
				prg := bankTestROM(pages, test.pageSize)
				mapper, err := MakeMapper(test.mapper, prg, make([]byte, 0x8000))
				if err != nil {
					t.Fatal(err)
				}
				cpu := StartupState()
				cpu.SetMapper(mapper)
				for raw := range 256 {
					value := byte(raw)
					address := uint16(0x8000)
					if test.mapper == 9 {
						address = 0xa000
					}
					cpu.StoreMemory(address, value)
					switch board := mapper.(type) {
					case *Mapper2:
						if board.Bank != value {
							t.Fatal("ROM size altered the latched UxROM register")
						}
					case *Mapper7:
						if board.Bank != int(value&test.mask) || board.Mirror != int(value>>4&1) {
							t.Fatal("ROM size altered the latched AxROM register")
						}
					case *Mapper9:
						if board.PrgRegister != value&test.mask {
							t.Fatal("MMC2 did not mask the PRG register to four bits")
						}
					}
					selected := int(value&test.mask) & (pages - 1)
					for _, address := range []uint16{0x8000, 0x9fff, 0xa000, 0xbfff, 0xc000, 0xdfff, 0xe000, 0xffff} {
						bank := selected
						if test.mapper == 2 && address >= 0xc000 {
							bank = pages - 1
						}
						if test.mapper == 9 && address >= 0xa000 {
							bank = pages - 3 + int(address-0xa000)/0x2000
						}
						offset := int(address-0x8000) % test.pageSize
						if got, want := cpu.LoadMemory(address), prg[bank*test.pageSize+offset]; got != want {
							t.Fatalf("write $%02x read $%04x: got $%02x, want $%02x (physical bank %d)", value, address, got, want, bank)
						}
					}
				}
			})
		}
	}
}

func TestCNROMCHRBankAddressLines(t *testing.T) {
	for _, pages := range []int{1, 2, 4, 8, 16} {
		t.Run(fmt.Sprintf("pages_%d", pages), func(t *testing.T) {
			chr := bankTestROM(pages, 0x2000)
			cpu := StartupState()
			cpu.SetMapper(MakeMapper3(make([]byte, 0x8000), chr))
			for raw := range 256 {
				cpu.StoreMemory(0x8000, byte(raw))
				base := (raw & (pages - 1)) * 0x2000
				if !bytes.Equal(cpu.PPU.VideoMemory[:0x2000], chr[base:base+0x2000]) {
					t.Fatalf("CHR register $%02x did not alias bank %d", raw, base/0x2000)
				}
			}
			before := bytes.Clone(cpu.PPU.VideoMemory[:0x2000])
			cpu.StoreMemory(0x6000, 0)
			if !bytes.Equal(cpu.PPU.VideoMemory[:0x2000], before) {
				t.Fatal("write below $8000 changed the CHR bank")
			}
		})
	}
}

func TestMMC3CHRBankAddressLines(t *testing.T) {
	for _, pages := range []int{8, 16, 32, 64, 128, 256} {
		for mode := range byte(2) {
			t.Run(fmt.Sprintf("pages_%d_mode_%d", pages, mode), func(t *testing.T) {
				chr := bankTestROM(pages, 0x400)
				cpu := StartupState()
				mapper := MakeMapper4(make([]byte, 0x8000), chr).(*Mapper4)
				cpu.SetMapper(mapper)
				for register := range byte(6) {
					for raw := range 256 {
						cpu.StoreMemory(0x8000, mode<<7|register)
						cpu.StoreMemory(0x8001, byte(raw))
						value, size, destination := byte(raw), 0x400, 0
						if register < 2 {
							value &= 0xfe
							size, destination = 0x800, int(register)*0x800
						} else {
							destination = 0x1000 + int(register-2)*0x400
						}
						if mode == 1 {
							destination ^= 0x1000
						}
						base := (int(value) & (pages - 1)) * 0x400
						if mapper.ChrRegister[register] != value {
							t.Fatal("physical ROM size altered the latched CHR register")
						}
						if !bytes.Equal(cpu.PPU.VideoMemory[destination:destination+size], chr[base:base+size]) {
							t.Fatalf("CHR register %d=$%02x did not alias physical page %d", register, value, base/0x400)
						}
					}
				}
			})
		}
	}
}

func TestMMC2CHRRegisterMasksAndAddressLines(t *testing.T) {
	for _, pages := range []int{2, 4, 8, 16, 32} {
		t.Run(fmt.Sprintf("pages_%d", pages), func(t *testing.T) {
			chr := bankTestROM(pages, 0x1000)
			cpu := StartupState()
			mapper := MakeMapper9(make([]byte, 0x20000), chr).(*Mapper9)
			cpu.SetMapper(mapper)
			for raw := range 256 {
				for register := range 4 {
					cpu.StoreMemory(uint16(0xb000+register*0x1000), byte(raw+register))
					if mapper.ChrRegister[register] != byte(raw+register)&0x1f {
						t.Fatal("MMC2 did not mask the CHR register to five bits")
					}
				}
				for _, trigger := range []uint16{0x0fd8, 0x0fe8, 0x1fd8, 0x1fe8} {
					cpu.PPU.readPatternMemory(trigger)
					table := int(trigger / 0x1000)
					register := table * 2
					if trigger&0xfff == 0xfe8 {
						register++
					}
					base := ((raw + register) & 0x1f & (pages - 1)) * 0x1000
					if got := cpu.PPU.readPatternMemory(uint16(table*0x1000 + 0x123)); got != chr[base+0x123] {
						t.Fatalf("CHR write $%02x trigger $%04x fetched the wrong physical bank", raw, trigger)
					}
					if !bytes.Equal(cpu.PPU.VideoMemory[table*0x1000:(table+1)*0x1000], chr[base:base+0x1000]) {
						t.Fatal("MMC2's PPU bank cache did not match the mirrored CHR bank")
					}
				}
			}
		})
	}
}

func TestBankAddressLinesEmptyAndNonPowerOfTwo(t *testing.T) {
	for _, kind := range []uint32{2, 3, 4, 7, 9} {
		mapper, err := MakeMapper(kind, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		cpu := StartupState()
		cpu.SetMapper(mapper)
		for _, address := range []uint16{0x8000, 0xa000, 0xc000, 0xe000, 0xffff} {
			if mapper.Read(address) != 0 {
				t.Fatalf("empty mapper %d ROM read was nonzero", kind)
			}
		}
		address := uint16(0x8000)
		if kind == 9 {
			address = 0xa000
		}
		if err := mapper.Write(&cpu, address, 0xff); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		kind uint32
		size int
	}{
		{2, 0x4000}, {4, 0x2000}, {7, 0x8000}, {9, 0x2000},
	} {
		// A three-bank image does not imply modulo-three electrical wiring.
		mapper, err := MakeMapper(test.kind, bytes.Repeat([]byte{0x5a}, 3*test.size), nil)
		if err != nil {
			t.Fatal(err)
		}
		cpu := StartupState()
		cpu.SetMapper(mapper)
		if test.kind == 4 {
			cpu.StoreMemory(0x8000, 6)
			cpu.StoreMemory(0x8001, 3)
		} else if test.kind == 9 {
			cpu.StoreMemory(0xa000, 3)
		} else {
			cpu.StoreMemory(0x8000, 3)
		}
		if mapper.Read(0x8000) != 0 {
			t.Fatalf("mapper %d invented modulo-three bank mirroring", test.kind)
		}
	}
}

func discreteBoardROM(t *testing.T, kind uint32, submapper byte, nes2 bool) NESFile {
	t.Helper()
	pageSize, pages, chrPages := 0x4000, 4, 0
	if kind == 3 {
		pageSize, pages, chrPages = 0x8000, 1, 4
	} else if kind == 7 {
		pageSize = 0x8000
	}
	prg := bankTestROM(pages, pageSize)
	prg[0] = 2
	if pages == 4 {
		prg[2*pageSize], prg[3*pageSize] = 1, 4
	}
	chr := bankTestROM(chrPages, 0x2000)
	data := make([]byte, 16)
	copy(data, []byte{'N', 'E', 'S', 0x1a})
	data[4], data[5], data[6] = byte(len(prg)/0x4000), byte(chrPages), byte(kind<<4)
	data[8] = submapper << 4
	if nes2 {
		data[7] = 8
	}
	data = append(append(data, prg...), chr...)
	rom, err := ParseNes(bytes.NewReader(data), false, "synthetic discrete board")
	if err != nil {
		t.Fatal(err)
	}
	return rom
}

func assertDiscreteBoardBank(t *testing.T, cpu *CPUState, kind uint32, value byte) {
	t.Helper()
	physical := int(value & 3)
	switch mapper := cpu.Mapper.Mapper.(type) {
	case *Mapper2:
		if mapper.Bank != value || cpu.LoadMemory(0x8000) != mapper.BankMemory[physical*0x4000] {
			t.Fatalf("mapper 2 latched bank $%02x, want $%02x", mapper.Bank, value)
		}
	case *Mapper3:
		if cpu.PPU.VideoMemory[0] != byte(physical+1) {
			t.Fatalf("mapper 3 selected the wrong CHR bank for value $%02x", value)
		}
	case *Mapper7:
		if mapper.Bank != int(value&0x0f) || cpu.LoadMemory(0x8000) != mapper.ProgramRom[physical*0x8000] {
			t.Fatalf("mapper 7 latched bank $%02x, want $%02x", mapper.Bank, value&0x0f)
		}
		wantMirror := NametableMirrorConfiguration(NametableMirrorScreenA)
		if value&0x10 != 0 {
			wantMirror = NametableMirrorScreenB
		}
		if cpu.PPU.NametableMirror != wantMirror {
			t.Fatal("mapper 7 bus conflict was not applied to the mirroring bit")
		}
	default:
		t.Fatalf("unexpected mapper type for %d", kind)
	}
}

func TestDiscreteBoardSubmapperBusConflicts(t *testing.T) {
	for _, kind := range []uint32{2, 3, 7} {
		for _, test := range []struct {
			nes2 bool
			sub  byte
		}{
			{false, 2}, // Legacy header byte 8 is NOT submapper metadata.
			{true, 0}, {true, 1}, {true, 2},
		} {
			t.Run(fmt.Sprintf("mapper_%d_NES2_%v_sub_%d", kind, test.nes2, test.sub), func(t *testing.T) {
				rom := discreteBoardROM(t, kind, test.sub, test.nes2)
				wantSub := byte(0)
				if test.nes2 {
					wantSub = test.sub
				}
				if rom.Mapper != kind || rom.Submapper != wantSub || rom.Nes2 != test.nes2 {
					t.Fatal("incorrect mapper/submapper metadata")
				}
				mapper, err := rom.MakeMapper()
				if err != nil {
					t.Fatal(err)
				}
				cpu := StartupState()
				cpu.SetMapper(mapper)
				cpu.StoreMemory(0x8000, 0x17)
				want := byte(0x17)
				if test.nes2 && test.sub == 2 {
					want &= 2 // The currently mapped PRG-ROM drives $02.
				}
				if cpu.Databus != want {
					t.Fatal("CPU data bus did not retain the electrically driven write value")
				}
				assertDiscreteBoardBank(t, &cpu, kind, want)
			})
		}
		rom := discreteBoardROM(t, kind, 3, true)
		if _, err := rom.MakeMapper(); err == nil {
			t.Fatalf("mapper %d silently accepted an unsupported board submapper", kind)
		}
	}
}

func TestDiscreteBusConflictUsesOldPRGMapping(t *testing.T) {
	for _, kind := range []uint32{2, 7} {
		rom := discreteBoardROM(t, kind, 2, true)
		mapper, err := rom.MakeMapper()
		if err != nil {
			t.Fatal(err)
		}
		cpu := StartupState()
		cpu.SetMapper(mapper)
		cpu.StoreMemory(0x8000, 0x17) // Old bank 0 drives $02; select bank 2.
		assertDiscreteBoardBank(t, &cpu, kind, 2)
		cpu.StoreMemory(0x8000, 3) // Old bank 2 drives $01, not bank 3's $04.
		assertDiscreteBoardBank(t, &cpu, kind, 1)
	}
	// CNROM's 16 KiB PRG mirror must drive the bus at $C000 too.
	rom := NESFile{Mapper: 3, Nes2: true, Submapper: 2, ProgramRom: make([]byte, 0x4000), CharacterRom: bankTestROM(4, 0x2000)}
	rom.ProgramRom[0] = 2
	mapper, err := rom.MakeMapper()
	if err != nil {
		t.Fatal(err)
	}
	cpu := StartupState()
	cpu.SetMapper(mapper)
	cpu.StoreMemory(0xc000, 0x17)
	assertDiscreteBoardBank(t, &cpu, 3, 2)
}

func TestDiscreteBusConflictCopyAndRestore(t *testing.T) {
	for _, kind := range []uint32{2, 3, 7} {
		rom := discreteBoardROM(t, kind, 2, true)
		mapper, err := rom.MakeMapper()
		if err != nil {
			t.Fatal(err)
		}
		cpu := StartupState()
		cpu.SetMapper(mapper)
		copied := cpu.Copy()
		data, err := json.Marshal(cpu)
		if err != nil {
			t.Fatal(err)
		}
		var restored CPUState
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		restored.Restore()
		for _, state := range []*CPUState{&copied, &restored} {
			state.StoreMemory(0x8000, 0x17)
			assertDiscreteBoardBank(t, state, kind, 2)
		}
	}
}

func TestNES2ExtendedMapperNumber(t *testing.T) {
	rom := discreteBoardROM(t, 2, 2, true)
	// Byte 8's low nibble extends the mapper number; its high nibble is separate.
	header := make([]byte, 16)
	copy(header, []byte{'N', 'E', 'S', 0x1a})
	header[4], header[6], header[7], header[8] = 4, 0x20, 8, 0x21
	data := append(header, rom.ProgramRom...)
	parsed, err := ParseNes(bytes.NewReader(data), false, "extended mapper")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Mapper != 0x102 || parsed.Submapper != 2 {
		t.Fatal("extended mapper number was confused with submapper bits")
	}
	if _, err := parsed.MakeMapper(); err == nil {
		t.Fatal("extended mapper was incorrectly treated as UxROM")
	}
}

// CNROM PRG-ROM mapping.

func TestCNROMPRGMapping(t *testing.T) {
	for _, size := range []int{0x4000, 0x8000} {
		t.Run(fmt.Sprintf("PRG_%d_KiB", size/1024), func(t *testing.T) {
			prg := make([]byte, size)
			for i := range prg {
				prg[i] = byte(i>>8) ^ byte(i) ^ 0x5a
			}
			cpu := StartupState()
			cpu.SetMapper(MakeMapper3(prg, make([]byte, 0x8000)))
			for offset := range 0x8000 {
				address := uint16(0x8000 + offset)
				if got, want := cpu.LoadMemory(address), prg[offset%size]; got != want {
					t.Fatalf("PRG read $%04x: got $%02x, want $%02x", address, got, want)
				}
			}
			// CHR bank switching must not change the fixed PRG mapping.
			for bank := range byte(4) {
				cpu.StoreMemory(0x8000, bank)
				for _, address := range []uint16{0x8000, 0xbfff, 0xc000, 0xffff} {
					want := prg[int(address-0x8000)%size]
					if cpu.LoadMemory(address) != want {
						t.Fatalf("CHR bank %d changed PRG mapping at $%04x", bank, address)
					}
				}
			}
		})
	}
}

func TestCNROMMirroredVectorsAndExecution(t *testing.T) {
	prg := make([]byte, 0x4000)
	copy(prg[0x0100:], []byte{0xa9, 0x42, 0x8d, 0x00, 0x02})       // LDA #$42; STA $0200.
	copy(prg[0x3ffa:], []byte{0x00, 0xc2, 0x00, 0xc1, 0x00, 0xc3}) // NMI/reset/IRQ.
	cpu := StartupState()
	cpu.SetMapper(MakeMapper3(prg, make([]byte, 0x2000)))
	cpu.Reset()
	if cpu.PC != 0xc100 {
		t.Fatalf("mirrored reset vector: got PC $%04x, want $C100", cpu.PC)
	}
	table := MakeInstructionDescriptiontable()
	for range 2 {
		if err := cpu.Run(table); err != nil {
			t.Fatal(err)
		}
	}
	if cpu.LoadMemory(0x0200) != 0x42 || cpu.PC != 0xc105 {
		t.Fatal("CPU did not execute instructions from the mirrored PRG half")
	}
	cpu.NMI()
	if cpu.PC != 0xc200 {
		t.Fatalf("mirrored NMI vector: got PC $%04x, want $C200", cpu.PC)
	}
	cpu.Interrupt()
	if cpu.PC != 0xc300 {
		t.Fatalf("mirrored IRQ vector: got PC $%04x, want $C300", cpu.PC)
	}
}

func mmc3PRGCPU(pages int) CPUState {
	prg := make([]byte, pages*0x2000)
	for i := range prg {
		prg[i] = byte(i/0x2000) ^ byte(i)
	}
	cpu := StartupState()
	cpu.SetMapper(MakeMapper4(prg, make([]byte, 0x2000)))
	return cpu
}

// MMC5 cartridge routing and state.

func TestMMC5PRGModesAndWorkRAM(t *testing.T) {
	prg := make([]byte, 32*0x2000)
	for bank := range 32 {
		prg[bank*0x2000] = byte(bank)
	}
	cpu := StartupState()
	mapper, err := MakeMapper(5, prg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cpu.SetMapper(mapper)
	if got := cpu.LoadMemory(0xe000); got != 31 {
		t.Fatalf("power-on last bank: %d", got)
	}
	for i, value := range []byte{0x85, 0x87, 0x89, 0x8b} {
		cpu.StoreMemory(0x5114+uint16(i), value)
	}
	for mode, want := range [][4]byte{{8, 9, 10, 11}, {6, 7, 10, 11}, {6, 7, 9, 11}, {5, 7, 9, 11}} {
		cpu.StoreMemory(0x5100, byte(mode))
		for slot, bank := range want {
			address := 0x8000 + uint16(slot)*0x2000
			if got := cpu.LoadMemory(address); got != bank {
				t.Fatalf("mode %d, slot %d: got bank %d, want %d", mode, slot, got, bank)
			}
		}
	}
	cpu.StoreMemory(0x5113, 3)
	cpu.StoreMemory(0x5114, 3)
	cpu.StoreMemory(0x6000, 42)
	if cpu.LoadMemory(0x8000) != 0 {
		t.Fatal("WRAM was not protected")
	}
	cpu.StoreMemory(0x5102, 2)
	cpu.StoreMemory(0x5103, 1)
	cpu.StoreMemory(0x6000, 42)
	if cpu.LoadMemory(0x8000) != 42 {
		t.Fatal("banked WRAM windows did not share storage")
	}
}

func TestMMC5PatternAndNametableSources(t *testing.T) {
	chr := make([]byte, 256*0x400)
	for bank := range 256 {
		chr[bank*0x400] = byte(bank)
	}
	cpu := StartupState()
	m := MakeMapper5(nil, chr)
	cpu.SetMapper(m)
	cpu.StoreMemory(0x5101, 3)
	cpu.PPU.SetControllerFlags(0x20) // 8x16 sprites use separate banks.
	cpu.StoreMemory(0x5120, 5)
	cpu.StoreMemory(0x5128, 9)
	if got := m.ReadSpriteCharacter(&cpu.PPU, 0); got != 5 {
		t.Fatalf("sprite bank: %d", got)
	}
	if lo, _, _ := m.ReadBackgroundTile(&cpu.PPU, 0x2000, 0x1000, 0); lo != 9 {
		t.Fatalf("background bank: %d", lo)
	}
	if got := m.ReadCharacter(&cpu.PPU, 0); got != 9 {
		t.Fatalf("CPU CHR read did not use last written set: %d", got)
	}
	cpu.StoreMemory(0x5105, 0xe4) // CIRAM 0, CIRAM 1, ExRAM, fill.
	for tab := range 4 {
		cpu.PPU.StoreNametableMemory(0x2000+uint16(tab)*0x400, byte(tab+1))
	}
	cpu.StoreMemory(0x5106, 4)
	cpu.StoreMemory(0x5107, 2)
	for tab := range 4 {
		if got := cpu.PPU.LoadNametableMemory(0x2000 + uint16(tab)*0x400); got != byte(tab+1) {
			t.Fatalf("nametable %d value=%d", tab, got)
		}
	}
	if cpu.PPU.LoadNametableMemory(0x2fc0) != 0xaa {
		t.Fatal("fill attribute was not replicated across all quadrants")
	}
	cpu.StoreMemory(0x5104, 2)
	cpu.StoreMemory(0x5c10, 0x83)
	if cpu.LoadMemory(0x5c10) != 0x83 || cpu.PPU.LoadNametableMemory(0x2810) != 0 {
		t.Fatal("CPU ExRAM did not disconnect from PPU nametable routing")
	}
	cpu.StoreMemory(0x5104, 1)
	if lo, _, palette := m.ReadBackgroundTile(&cpu.PPU, 0x2010, 0, 0); lo != 12 || palette != 2 {
		t.Fatalf("extended attributes: bank byte=%d palette=%d", lo, palette)
	}
}

func TestMMC5IRQExpansionAndState(t *testing.T) {
	cpu := StartupState()
	m := MakeMapper5(nil, nil)
	cpu.SetMapper(m)
	cpu.StoreMemory(0x5205, 31)
	cpu.StoreMemory(0x5206, 99)
	if lo, hi := cpu.LoadMemory(0x5205), cpu.LoadMemory(0x5206); uint16(hi)<<8|uint16(lo) != 31*99 {
		t.Fatal("bad MMC5 multiplication result")
	}
	cpu.StoreMemory(0x5203, 2)
	cpu.PPU.SetMask(8)
	cpu.PPU.Run(2+2*341, MakeVirtualScreen(256, 240))
	if !m.IRQPending || m.IsIRQAsserted() {
		t.Fatal("scanline match should latch pending IRQ even while disabled")
	}
	cpu.StoreMemory(0x5204, 0x80)
	cpu.PPU.StoreNametableMemory(0x2000, 0x3c)
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	copied := cpu.Copy()
	for _, state := range []*CPUState{&copied, &restored} {
		if !state.Mapper.Mapper.IsIRQAsserted() || state.PPU.LoadNametableMemory(0x2400) != 0x3c {
			t.Fatal("state lost IRQ or nametable routing")
		}
		if got := state.LoadMemory(0x5204); got != 0xc0 || state.Mapper.Mapper.IsIRQAsserted() {
			t.Fatalf("status=$%02x or IRQ not acknowledged", got)
		}
	}
	if !m.IsIRQAsserted() {
		t.Fatal("acknowledging a copy affected the live mapper")
	}
}

// MMC3 PRG-ROM bank address lines.

func TestMMC3PRGBankAliasing(t *testing.T) {
	for _, pages := range []int{4, 8, 16, 32, 64} {
		for mode := range byte(2) {
			t.Run(fmt.Sprintf("pages_%d_mode_%d", pages, mode), func(t *testing.T) {
				cpu := mmc3PRGCPU(pages)
				mapper := cpu.Mapper.Mapper.(*Mapper4)
				for raw := range 256 {
					value := byte(raw)
					cpu.StoreMemory(0x8000, mode<<6|6)
					cpu.StoreMemory(0x8001, value)
					cpu.StoreMemory(0x8000, mode<<6|7)
					cpu.StoreMemory(0x8001, value)
					// Disconnected ROM address lines must not alter the registers.
					if mapper.PrgRegister != [2]byte{value & 0x3f, value & 0x3f} {
						t.Fatalf("write $%02x lost latched bank bits", value)
					}
					selected := int(value&0x3f) % pages
					banks := [4]int{selected, selected, pages - 2, pages - 1}
					if mode == 1 {
						banks[0], banks[2] = banks[2], banks[0]
					}
					for _, address := range []uint16{0x8000, 0x9fff, 0xa000, 0xbfff, 0xc000, 0xdfff, 0xe000, 0xffff} {
						bank := banks[(address-0x8000)/0x2000]
						want := mapper.ProgramRom[bank*0x2000+int(address&0x1fff)]
						if got := cpu.LoadMemory(address); got != want {
							t.Fatalf("register $%02x at $%04x: got $%02x, want $%02x (physical bank %d)", value, address, got, want, bank)
						}
					}
				}
			})
		}
	}
}

func TestMMC3BatmanPRGBankSelections(t *testing.T) {
	cpu := mmc3PRGCPU(16) // Batman has 128 KiB PRG-ROM: sixteen 8 KiB banks.
	for _, banks := range [][2]byte{{0x1c, 0x1d}, {0x12, 0x13}} {
		for index, bank := range banks {
			cpu.StoreMemory(0x8000, byte(6+index))
			cpu.StoreMemory(0x8001, bank)
		}
		for slot, bank := range banks {
			address := uint16(0x8000 + slot*0x2000 + 0x106)
			want := (bank & 0xf) ^ 0x06
			if got := cpu.LoadMemory(address); got != want {
				t.Fatalf("Batman bank $%02x at $%04x: got $%02x, want $%02x", bank, address, got, want)
			}
		}
	}
}

func TestMMC3EmptyPRGRead(t *testing.T) {
	cpu := mmc3PRGCPU(0)
	for _, address := range []uint16{0x8000, 0xa000, 0xc000, 0xe000} {
		if got := cpu.LoadMemory(address); got != 0 {
			t.Fatalf("empty PRG-ROM read at $%04x returned $%02x", address, got)
		}
	}
}

// MMC3 IRQ counter/reload behavior (standard MMC3B/C).

func mmc3IRQCPU() CPUState {
	cpu := StartupState()
	cpu.SetMapper(MakeMapper4(make([]byte, 0x8000), make([]byte, 0x2000)))
	return cpu
}

func TestMMC3IRQReloadRequest(t *testing.T) {
	for _, address := range []uint16{0xc001, 0xdfff} {
		for _, value := range []byte{0, 1, 0x7f, 0xff} {
			t.Run(fmt.Sprintf("address_%04x_value_%02x", address, value), func(t *testing.T) {
				cpu := mmc3IRQCPU()
				mapper := cpu.Mapper.Mapper.(*Mapper4)
				mapper.IrqCounter = 5
				cpu.StoreMemory(0xc000, 3)
				cpu.StoreMemory(0xe001, 0)
				cpu.StoreMemory(address, value)
				if mapper.IsIRQAsserted() {
					t.Fatal("reload write asserted IRQ before a counter clock")
				}
				for i, want := range []byte{3, 2, 1, 0, 3} {
					mapper.Scanline()
					if mapper.IrqCounter != want {
						t.Fatalf("clock %d: got counter %d, want %d", i+1, mapper.IrqCounter, want)
					}
					if mapper.IsIRQAsserted() != (i >= 3) {
						t.Fatalf("clock %d: unexpected IRQ state", i+1)
					}
				}
			})
		}
	}
}

func TestMMC3IRQLatchAndReloadWrites(t *testing.T) {
	cpu := mmc3IRQCPU()
	mapper := cpu.Mapper.Mapper.(*Mapper4)
	mapper.IrqCounter = 5
	cpu.StoreMemory(0xc000, 2)
	mapper.Scanline()
	if mapper.IrqCounter != 4 {
		t.Fatal("latch write immediately reloaded the running counter")
	}
	cpu.StoreMemory(0xc001, 0xff)
	cpu.StoreMemory(0xc001, 1) // Repeated requests do not queue extra reloads.
	cpu.StoreMemory(0xc000, 7) // The next clock uses the latest latch value.
	mapper.Scanline()
	if mapper.IrqCounter != 7 {
		t.Fatal("pending reload did not use the latest IRQ latch")
	}
	mapper.Scanline()
	if mapper.IrqCounter != 6 {
		t.Fatal("reload request was not cleared after one clock")
	}
}

func TestMMC3IRQZeroReload(t *testing.T) {
	for _, test := range []struct {
		name    string
		counter byte
		latch   byte
		request bool
		enabled bool
		want    byte
		irq     bool
	}{
		{"automatic_zero", 0, 0, false, true, 0, true},
		{"forced_zero", 5, 0, true, true, 0, true},
		{"disabled_zero", 5, 0, true, false, 0, false},
		{"decrement_to_zero", 1, 7, false, true, 0, true},
		{"automatic_nonzero", 0, 7, false, true, 7, false},
		{"forced_nonzero", 5, 7, true, true, 7, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cpu := mmc3IRQCPU()
			mapper := cpu.Mapper.Mapper.(*Mapper4)
			mapper.IrqCounter = test.counter
			cpu.StoreMemory(0xc000, test.latch)
			if test.request {
				cpu.StoreMemory(0xc001, 0xff)
			}
			if test.enabled {
				cpu.StoreMemory(0xe001, 0)
			}
			if mapper.IsIRQAsserted() {
				t.Fatal("register write asserted IRQ without a clock")
			}
			mapper.Scanline()
			if mapper.IrqCounter != test.want || mapper.IsIRQAsserted() != test.irq {
				t.Fatalf("counter/IRQ = %d/%v, want %d/%v", mapper.IrqCounter, mapper.IsIRQAsserted(), test.want, test.irq)
			}
		})
	}
}

func TestMMC3IRQAcknowledgePreservesReload(t *testing.T) {
	cpu := mmc3IRQCPU()
	mapper := cpu.Mapper.Mapper.(*Mapper4)
	mapper.IrqCounter = 5
	mapper.IrqPending = true
	cpu.StoreMemory(0xc000, 3)
	cpu.StoreMemory(0xc001, 0xff)
	cpu.StoreMemory(0xe000, 0)
	if mapper.IrqEnabled || mapper.IsIRQAsserted() {
		t.Fatal("$E000 did not disable and acknowledge IRQ")
	}
	mapper.Scanline()
	if mapper.IrqCounter != 3 || mapper.IsIRQAsserted() {
		t.Fatal("disabled clock did not consume the pending reload without IRQ")
	}
	cpu.StoreMemory(0xe001, 0)
	for range 3 {
		mapper.Scanline()
	}
	if !mapper.IsIRQAsserted() {
		t.Fatal("reenabled counter did not assert IRQ on reaching zero")
	}
	cpu.StoreMemory(0xe000, 0)
	cpu.StoreMemory(0xc000, 0)
	cpu.StoreMemory(0xe001, 0)
	if mapper.IsIRQAsserted() {
		t.Fatal("enabling IRQ with counter zero asserted without a clock")
	}
	mapper.Scanline()
	if !mapper.IsIRQAsserted() {
		t.Fatal("zero reload did not reassert IRQ on the next clock")
	}
}

func TestMMC3IRQReloadCopyAndRestore(t *testing.T) {
	cpu := mmc3IRQCPU()
	mapper := cpu.Mapper.Mapper.(*Mapper4)
	mapper.IrqCounter = 5
	cpu.StoreMemory(0xc000, 3)
	cpu.StoreMemory(0xc001, 0xff)
	copied := cpu.Copy()
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	for _, state := range []*CPUState{&copied, &restored} {
		state.PPU.updateMapperScanline()
		if state.Mapper.Mapper.(*Mapper4).IrqCounter != 3 {
			t.Fatal("copy/save state lost the pending reload request")
		}
	}
	mapper.Scanline()
	if mapper.IrqCounter != 3 {
		t.Fatal("clocking copy/save state consumed the original's reload request")
	}
}

// Four-screen cartridge headers and MMC3 nametable wiring.

func mmc3HeaderROM(t *testing.T, flags byte) NESFile {
	t.Helper()
	data := make([]byte, 16+0x8000+0x2000)
	copy(data, []byte{'N', 'E', 'S', 0x1a})
	data[4], data[5], data[6] = 2, 1, 0x40|flags
	data[16+0x7ffd] = 0x80 // Reset vector $8000.
	rom, err := ParseNes(bytes.NewReader(data), false, "synthetic MMC3")
	if err != nil {
		t.Fatal(err)
	}
	return rom
}

func mmc3FourScreenCPU(t *testing.T) CPUState {
	t.Helper()
	rom := mmc3HeaderROM(t, 8)
	cpu := StartupState()
	rom.ConfigureMirroring(&cpu.PPU)
	mapper, err := MakeMapper(rom.Mapper, rom.ProgramRom, rom.CharacterRom)
	if err != nil {
		t.Fatal(err)
	}
	cpu.SetMapper(mapper)
	cpu.Cycle = 30000
	return cpu
}

func fourScreenByte(offset int) byte {
	return byte((offset/0x400+1)*0x33) ^ byte(offset) ^ byte(offset>>8)
}

func fillFourScreenNametables(cpu *CPUState) {
	for offset := range 0x1000 {
		writePatternByte(cpu, uint16(0x2000+offset), fourScreenByte(offset))
	}
}

func TestCartridgeHeaderMirroring(t *testing.T) {
	for _, flags := range []byte{0, 1, 8, 9} {
		t.Run(fmt.Sprintf("flags_%02x", flags), func(t *testing.T) {
			rom := mmc3HeaderROM(t, flags)
			four := flags&8 != 0
			if rom.FourScreenMirror != four || rom.HorizontalMirror != (!four && flags&1 == 0) || rom.VerticalMirror != (!four && flags&1 != 0) {
				t.Fatal("header did not give four-screen mode priority over the H/V bit")
			}
			ppu := MakePPU()
			ppu.NametableMemory[0] = 0x42
			rom.ConfigureMirroring(&ppu)
			wantMode := NametableMirrorConfiguration(NametableMirrorHorizontal)
			wantSize := 0x800
			if flags&1 != 0 {
				wantMode = NametableMirrorVertical
			}
			if four {
				wantMode, wantSize = NametableMirrorFourScreen, 0x1000
			}
			if ppu.NametableMirror != wantMode || len(ppu.NametableMemory) != wantSize || ppu.NametableMemory[0] != 0x42 {
				t.Fatal("header mirroring was not applied with the correct RAM size/content")
			}
		})
	}
	ppu := MakePPU()
	NESFile{HorizontalMirror: true, VerticalMirror: true, FourScreenMirror: true}.ConfigureMirroring(&ppu)
	if ppu.NametableMirror != NametableMirrorFourScreen {
		t.Fatal("four-screen configuration lost priority over conflicting H/V settings")
	}
}

func TestMMC3FourIndependentNametables(t *testing.T) {
	cpu := mmc3FourScreenCPU(t)
	fillFourScreenNametables(&cpu)
	for offset := range 0x1000 {
		address := uint16(0x2000 + offset)
		if got, want := readPatternByte(&cpu, address), fourScreenByte(offset); got != want {
			t.Fatalf("nametable read $%04x: got $%02x, want $%02x", address, got, want)
		}
	}
	// $3000-$3EFF shadows the first $F00 bytes of nametable space.
	for offset := range 0xf00 {
		address := uint16(0x3000 + offset)
		if got := readPatternByte(&cpu, address); got != fourScreenByte(offset) {
			t.Fatalf("nametable shadow read $%04x returned $%02x", address, got)
		}
	}
	writePatternByte(&cpu, 0x3eff, 0x91)
	if readPatternByte(&cpu, 0x2eff) != 0x91 {
		t.Fatal("write at the end of the nametable shadow did not reach $2EFF")
	}
	writePatternByte(&cpu, 0x3f00, 0x2a)
	setPatternAddress(&cpu, 0x3f00)
	if cpu.LoadMemory(0x2007) != 0x2a {
		t.Fatal("four-screen mapping interfered with palette RAM")
	}
	setPatternAddress(&cpu, 0x2000)
	if cpu.LoadMemory(0x2007) != fourScreenByte(0xf00) {
		t.Fatal("palette read buffer did not fetch from the fourth nametable")
	}
	before := bytes.Clone(cpu.PPU.NametableMemory)
	cpu.PPU.SetFourScreenMirror()
	if !bytes.Equal(cpu.PPU.NametableMemory, before) {
		t.Fatal("reapplying four-screen mode erased nametable RAM")
	}
}

func TestMMC3FourScreenIgnoresMirroringAliases(t *testing.T) {
	cpu := mmc3FourScreenCPU(t)
	fillFourScreenNametables(&cpu)
	before := bytes.Clone(cpu.PPU.NametableMemory)
	for offset := uint16(0); offset < 0x2000; offset += 2 {
		for _, value := range []byte{0, 1, 0xfe, 0xff} {
			cpu.StoreMemory(0xa000+offset, value)
			if cpu.PPU.NametableMirror != NametableMirrorFourScreen {
				t.Fatalf("$%04x=$%02x overrode hardwired four-screen mode", 0xa000+offset, value)
			}
		}
	}
	if !bytes.Equal(cpu.PPU.NametableMemory, before) {
		t.Fatal("MMC3 mirroring writes changed nametable contents")
	}
	// Odd addresses in the same range must still control PRG-RAM normally.
	cpu.StoreMemory(0xbfff, 0x80)
	cpu.StoreMemory(0x6000, 0x42)
	cpu.StoreMemory(0xbfff, 0xc0)
	cpu.StoreMemory(0x6000, 0xff)
	if cpu.LoadMemory(0x6000) != 0x42 {
		t.Fatal("four-screen mirroring guard blocked the PRG-RAM control register")
	}
}

func TestMMC3FourScreenCopyAndRestore(t *testing.T) {
	cpu := mmc3FourScreenCPU(t)
	fillFourScreenNametables(&cpu)
	copied := cpu.Copy()
	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	for _, state := range []*CPUState{&copied, &restored} {
		cpuValue := readPatternByte(state, 0x2fff)
		if len(state.PPU.NametableMemory) != 0x1000 || cpuValue != fourScreenByte(0xfff) {
			t.Fatal("copy/save state lost the extra nametable RAM")
		}
		state.StoreMemory(0xbffe, 1)
		if state.PPU.NametableMirror != NametableMirrorFourScreen {
			t.Fatal("copy/save state lost hardwired four-screen mode")
		}
		writePatternByte(state, 0x2fff, 0x57)
	}
	if readPatternByte(&cpu, 0x2fff) != fourScreenByte(0xfff) {
		t.Fatal("copy/save state shares nametable RAM with the original")
	}
}

// MMC3 register address decoding.

func TestMMC3RegisterAliases(t *testing.T) {
	prg, chr := make([]byte, 0x10000), make([]byte, 0x2000)
	for i := range chr {
		chr[i] = byte(i / 0x400)
	}
	type registerWrite struct {
		name     string
		address  uint16
		value    byte
		register byte
	}
	cases := []registerWrite{
		{"bank_select", 0x8000, 0xc6, 0},
		{"mirror_horizontal", 0xa000, 1, 0},
		{"mirror_vertical", 0xa000, 0, 0},
		{"ram_enable", 0xa001, 0x80, 0},
		{"ram_protect", 0xa001, 0xc0, 0},
		{"ram_disable", 0xa001, 0, 0},
		{"irq_latch", 0xc000, 5, 0},
		// Reload behavior is tested separately; here verify address decoding.
		{"irq_reload", 0xc001, 3, 0},
		{"irq_disable", 0xe000, 0, 0},
		{"irq_enable", 0xe001, 0, 0},
	}
	for index := range byte(8) {
		value := byte(5)
		if index >= 6 {
			value = 0x45 // Also exercise PRG bank register masking.
		}
		cases = append(cases, registerWrite{fmt.Sprintf("bank_data_%d", index), 0x8001, value, index})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cpu := StartupState()
			mapper := MakeMapper4(prg, chr).(*Mapper4)
			mapper.ChrRegister = [6]byte{0, 2, 4, 5, 6, 7}
			mapper.PrgRegister = [2]byte{1, 2}
			mapper.RegisterIndex = test.register
			mapper.IrqCounter, mapper.IrqReload = 9, 7
			mapper.IrqEnabled = test.address != 0xe001
			mapper.IrqPending = true
			mapper.WramEnabled = test.value&0x80 == 0
			mapper.WramWrite = test.value&0x40 != 0
			cpu.SetMapper(mapper)
			initial := *mapper
			initialMirror := NametableMirrorConfiguration(NametableMirrorVertical)
			if test.address == 0xa000 && test.value == 0 {
				initialMirror = NametableMirrorHorizontal
			}
			cpu.PPU.NametableMirror = initialMirror
			cpu.StoreMemory(test.address, test.value)
			want := *mapper
			wantPattern := bytes.Clone(cpu.PPU.VideoMemory[:0x2000])
			wantMirror := cpu.PPU.NametableMirror

			// A1-A12 are not decoded: check every same-parity address in the
			// register's 8 KiB range, including its canonical address.
			for offset := uint16(0); offset < 0x2000; offset += 2 {
				address := test.address + offset
				*mapper = initial
				clear(cpu.PPU.VideoMemory[:0x2000])
				cpu.PPU.NametableMirror = initialMirror
				cpu.StoreMemory(address, test.value)
				if !reflect.DeepEqual(*mapper, want) {
					t.Fatalf("alias $%04x did not produce the same mapper state as $%04x", address, test.address)
				}
				if cpu.PPU.NametableMirror != wantMirror || !bytes.Equal(cpu.PPU.VideoMemory[:0x2000], wantPattern) {
					t.Fatalf("alias $%04x did not produce the same PPU mapping as $%04x", address, test.address)
				}
			}
		})
	}
}

func TestMMC3AliasesPreservePRGRAMAddressing(t *testing.T) {
	cpu := StartupState()
	mapper := MakeMapper4(make([]byte, 0x8000), make([]byte, 0x2000)).(*Mapper4)
	cpu.SetMapper(mapper)
	cpu.StoreMemory(0xbfff, 0x80) // Alias of $A001: enable writable PRG-RAM.
	for i, address := range []uint16{0x6000, 0x6001, 0x7ffe, 0x7fff} {
		cpu.StoreMemory(address, byte(i+1))
	}
	for i, address := range []uint16{0x6000, 0x6001, 0x7ffe, 0x7fff} {
		if got := cpu.LoadMemory(address); got != byte(i+1) {
			t.Fatalf("PRG-RAM $%04x: got %d, want %d", address, got, i+1)
		}
	}
	cpu.StoreMemory(0xbfff, 0xc0) // Write protect through the same alias.
	cpu.StoreMemory(0x6000, 0xff)
	if cpu.LoadMemory(0x6000) != 1 {
		t.Fatal("aliased write-protect register did not protect PRG-RAM")
	}
}

// PPU mapper interfaces and scanline clocks.

// This cartridge deliberately is not a Mapper9. Embedding Mapper supplies the
// unrelated CPU-side methods, which these PPU tests must never call.
type patternMapperStub struct {
	Mapper
	reads []uint16
}

func (mapper *patternMapperStub) ReadCharacter(_ *PPUState, address uint16) byte {
	mapper.reads = append(mapper.reads, address)
	return byte(address)
}

type scanlineMapperStub struct {
	Mapper
	clocks int
}

func (mapper *scanlineMapperStub) Scanline() {
	mapper.clocks++
}

func TestPPUScanlineMapper(t *testing.T) {
	ppu := MakePPU()
	mapper := &scanlineMapperStub{}
	ppu.setMapper(mapper)
	ppu.Mask = 8
	screen := MakeVirtualScreen(256, 240)
	for _, scanline := range []int{0, 260} {
		ppu.Scanline = scanline
		ppu.ScanlineCycle = 340
		ppu.Run(1, screen)
	}
	if mapper.clocks != 2 {
		t.Fatalf("got %d scanline clocks, want 2", mapper.clocks)
	}
	ppu.setMapper(MakeMapper0(make([]byte, 0x8000)))
	if ppu.characterMapper != nil || ppu.writableCharacterMapper != nil || ppu.scanlineMapper != nil {
		t.Fatal("switching cartridges did not clear optional mapper bindings")
	}
	ppu.updateMapperScanline()
	if mapper.clocks != 2 {
		t.Fatal("clocked the previous cartridge")
	}
}

func TestPPUScanlineMapperCopyAndRestore(t *testing.T) {
	cpu := StartupState()
	mapper := MakeMapper4(make([]byte, 0x8000), make([]byte, 0x2000)).(*Mapper4)
	mapper.IrqCounter = 1
	mapper.IrqEnabled = true
	cpu.SetMapper(mapper)
	copied := cpu.Copy()
	copyMapper := copied.Mapper.Mapper.(*Mapper4)
	if copied.PPU.scanlineMapper != copyMapper {
		t.Fatal("copy did not bind its own scanline mapper")
	}
	copied.PPU.updateMapperScanline()
	if !copyMapper.IrqPending || mapper.IrqPending {
		t.Fatal("copy's scanline clock did not affect only its own mapper")
	}

	data, err := json.Marshal(cpu)
	if err != nil {
		t.Fatal(err)
	}
	var restored CPUState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Restore()
	restoredMapper := restored.Mapper.Mapper.(*Mapper4)
	if restored.PPU.scanlineMapper != restoredMapper {
		t.Fatal("Restore did not bind the restored scanline mapper")
	}
	restored.PPU.updateMapperScanline()
	if !restoredMapper.IrqPending || mapper.IrqPending {
		t.Fatal("restored scanline clock did not affect only its own mapper")
	}
}

func TestPPUCharacterMapper(t *testing.T) {
	ppu := MakePPU()
	mapper := &patternMapperStub{}
	ppu.setMapper(mapper)
	if got := ppu.readPatternMemory(0x1234); got != 0x34 {
		t.Fatalf("pattern read returned $%02x, want $34", got)
	}
	ppu.VideoMemory[0x2000] = 0x56
	if got := ppu.readPatternMemory(0x2000); got != 0x56 || len(mapper.reads) != 1 {
		t.Fatal("non-pattern read was routed to the character mapper")
	}

	ppu.Flags = 8
	ppu.Scanline = 1
	ppu.CurrentSprites = []Sprite{{Tile: 2, Y: 1}}
	ppu.fetchSpritePatterns()
	if len(mapper.reads) != 3 || mapper.reads[1] != 0x1020 || mapper.reads[2] != 0x1028 {
		t.Fatalf("sprite fetch addresses: %v", mapper.reads)
	}
	low, high := ppu.spritePatternBytes(&ppu.CurrentSprites[0], 0x1020)
	if low[0] != 0x20 || high[0] != 0x28 || len(mapper.reads) != 3 {
		t.Fatal("sprite pixels did not use the cached mapper reads")
	}

	// Switching to a cartridge without either capability clears both bindings.
	ppu.setMapper(MakeMapper1(make([]byte, 0x8000), nil))
	if ppu.writableCharacterMapper == nil {
		t.Fatal("CHR-RAM cartridge did not bind its write capability")
	}
	ppu.setMapper(MakeMapper0(make([]byte, 0x8000)))
	if ppu.writableCharacterMapper != nil {
		t.Fatal("switching cartridges did not clear the CHR write binding")
	}
	ppu.VideoMemory[0x1234] = 0x78
	if got := ppu.readPatternMemory(0x1234); got != 0x78 || len(mapper.reads) != 3 {
		t.Fatal("ordinary mapper did not restore the pattern-memory fallback")
	}
}

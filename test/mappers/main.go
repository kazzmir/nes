// A headless mapper regression checker. Run from the repository root:
//
// go run ./test/mappers
// go run ./test/mappers -rom "path/to/mapper9.nes"
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	nes "github.com/kazzmir/nes/lib"
)

const defaultROM = "roms/Mike Tyson's Punch-Out!! (U) (PRG 1) [h1].nes"

// Avoid the latch-trigger addresses at the end of each pattern table. Comparing
// this whole prefix also distinguishes real CHR banks that begin with blank tiles.
const probeSize = 0xFD0

func setPPUAddress(cpu *nes.CPUState, address uint16) {
	cpu.LoadMemory(0x2002) // Reset the $2006 write toggle.
	cpu.StoreMemory(0x2006, byte(address>>8))
	cpu.StoreMemory(0x2006, byte(address))
}

func trigger(cpu *nes.CPUState, address uint16) {
	setPPUAddress(cpu, address)
	// Even though $2007 returns buffered data, this performs a PPU bus read
	// at address. The new latch selection applies to subsequent fetches.
	cpu.LoadMemory(0x2007)
}

func readPrefix(cpu *nes.CPUState, base uint16) []byte {
	setPPUAddress(cpu, base)
	cpu.LoadMemory(0x2007) // Discard the old read buffer.
	out := make([]byte, probeSize)
	for i := range out {
		out[i] = cpu.LoadMemory(0x2007)
	}
	return out
}

// Pick distinguishable banks from the supplied ROM, rather than assuming its
// first four banks contain different graphics. No ROM data is modified.
func distinctBanks(chr []byte) ([]byte, error) {
	var banks []byte
	for bank := 0; bank < len(chr)/0x1000 && bank < 32; bank++ {
		prefix := chr[bank*0x1000 : bank*0x1000+probeSize]
		unique := true
		for _, previous := range banks {
			if bytes.Equal(prefix, chr[int(previous)*0x1000:int(previous)*0x1000+probeSize]) {
				unique = false
				break
			}
		}
		if unique {
			banks = append(banks, byte(bank))
			if len(banks) == 4 {
				return banks, nil
			}
		}
	}
	return nil, fmt.Errorf("need four distinguishable 4 KiB CHR banks, found %d", len(banks))
}

// checkMMC2 uses only CPU-visible register writes and $2007 reads to exercise
// banking. It never calls SetChrBank or reads PPU.VideoMemory directly.
// report is called once per assertion; the return value counts failed assertions.
func checkMMC2(rom nes.NESFile, report func(string, error)) (int, error) {
	if rom.Mapper != 9 {
		return 0, fmt.Errorf("ROM uses mapper %d, expected MMC2 (9)", rom.Mapper)
	}
	banks, err := distinctBanks(rom.CharacterRom)
	if err != nil {
		return 0, err
	}
	mapper, err := nes.MakeMapper(rom.Mapper, rom.ProgramRom, rom.CharacterRom)
	if err != nil {
		return 0, err
	}
	cpu := nes.StartupState()
	// Skip the power-on interval during which PPU register writes are ignored.
	cpu.Cycle = 30000
	cpu.SetMapper(mapper)
	cpu.PPU.CopyCharacterRom(0, rom.CharacterRom[:0x2000], true)
	cpu.StoreMemory(0x2000, 0) // $2007 increments by one byte.

	// $B000/$C000 select lower FD/FE banks; $D000/$E000 select upper FD/FE.
	for i, bank := range banks {
		cpu.StoreMemory(uint16(0xB000+i*0x1000), bank)
	}
	// Establish known latch states; do not rely on power-on latch values.
	trigger(&cpu, 0x0FE8)
	trigger(&cpu, 0x1FE8)

	failures := 0
	assertBanks := func(name string, lower, upper byte) {
		for half, bank := range []byte{lower, upper} {
			base := uint16(half * 0x1000)
			got := readPrefix(&cpu, base)
			want := rom.CharacterRom[int(bank)*0x1000 : int(bank)*0x1000+probeSize]
			var mismatch error
			for i := range want {
				if got[i] != want[i] {
					mismatch = fmt.Errorf("PPU $%04X: got $%02X, want $%02X (CHR bank %d)", int(base)+i, got[i], want[i], bank)
					break
				}
			}
			if mismatch != nil {
				failures++
			}
			report(fmt.Sprintf("%s / table %d", name, half), mismatch)
		}
	}

	assertBanks("both latches FE", banks[1], banks[3])
	trigger(&cpu, 0x0FD8)
	assertBanks("lower FD, upper unchanged", banks[0], banks[3])
	// Unlike the upper table, lower-table latch addresses are exact matches.
	trigger(&cpu, 0x0FE9)
	assertBanks("lower $0FE9 must not select FE", banks[0], banks[3])
	trigger(&cpu, 0x0FE8)
	assertBanks("lower FE", banks[1], banks[3])
	trigger(&cpu, 0x0FD9)
	assertBanks("lower $0FD9 must not select FD", banks[1], banks[3])

	// Every address in these inclusive upper-table ranges must trigger.
	for offset := uint16(0); offset < 8; offset++ {
		trigger(&cpu, 0x1FD8+offset)
		assertBanks(fmt.Sprintf("upper FD via $%04X", 0x1FD8+offset), banks[1], banks[2])
		trigger(&cpu, 0x1FE8+offset)
		assertBanks(fmt.Sprintf("upper FE via $%04X", 0x1FE8+offset), banks[1], banks[3])
	}
	trigger(&cpu, 0x1FD7)
	trigger(&cpu, 0x1FE0)
	assertBanks("outside upper FD range must not switch", banks[1], banks[3])
	trigger(&cpu, 0x1FD8)
	trigger(&cpu, 0x1FE7)
	trigger(&cpu, 0x1FF0)
	assertBanks("outside upper FE range must not switch", banks[1], banks[2])

	// Changing an inactive register must not change the visible bank; its new
	// value must become visible when that latch is subsequently selected.
	cpu.StoreMemory(0xE000, banks[0])
	assertBanks("write inactive upper FE register", banks[1], banks[2])
	trigger(&cpu, 0x1FE8)
	assertBanks("select updated upper FE register", banks[1], banks[0])
	cpu.StoreMemory(0xE000, banks[1])
	assertBanks("write active upper FE register", banks[1], banks[1])
	return failures, nil
}

func main() {
	path := flag.String("rom", defaultROM, "mapper-9 iNES ROM used for CHR bank data")
	flag.Parse()
	rom, err := nes.ParseNesFile(*path, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("MMC2 latch check: %s\n", *path)
	checks := 0
	failures, err := checkMMC2(rom, func(name string, err error) {
		checks++
		if err != nil {
			fmt.Printf("FAIL %s: %v\n", name, err)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("%d/%d assertions passed\n", checks-failures, checks)
	if failures != 0 {
		os.Exit(1)
	}
}

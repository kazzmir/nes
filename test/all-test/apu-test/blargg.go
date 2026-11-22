package aputest

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	nes "github.com/kazzmir/nes/lib"
	test_utils "github.com/kazzmir/nes/test/all-test/utils"
)

var blarggROMs = []struct {
	name     string
	wantCode byte // Nonzero only for a known failure; remove when fixed.
}{
	{"1-len_ctr", 3},
	{"2-len_table", 1},
	{"3-irq_flag", 0},
	{"4-jitter", 3},
	{"5-len_timing", 3},
	{"6-irq_flag_timing", 3},
	{"7-dmc_basics", 7},
	{"8-dmc_rates", 3},
}

func runBlarggROM(path string) (byte, string, error) {
	rom, err := nes.ParseNesFile(path, false)
	if err != nil {
		return 0, "", err
	}
	mapper, err := rom.MakeMapper()
	if err != nil {
		return 0, "", err
	}
	cpu := nes.StartupState()
	cpu.SetRegion(nes.RegionNTSC)
	cpu.SetMapper(mapper)
	rom.ConfigureMirroring(&cpu.PPU)
	cpu.Reset()

	screen := nes.MakeVirtualScreen(256, 240)
	table := nes.MakeInstructionDescriptiontable()
	deadline := cpu.Cycle + 8_000_000
	for cpu.Cycle < deadline {
		start := cpu.Cycle
		if _, err := cpu.Step(table, screen); err != nil {
			return 0, "", err
		}
		cpu.APU.Run(float64(cpu.Cycle-start)/2, 100, &cpu)

		// Inspect cartridge RAM directly so polling does not change CPU open bus.
		if mapper.Read(0x6001) != 0xde || mapper.Read(0x6002) != 0xb0 || mapper.Read(0x6003) != 0x61 {
			continue
		}
		code := mapper.Read(0x6000)
		if code == 0x80 { // Running.
			continue
		}
		if code >= 0x80 {
			return 0, "", fmt.Errorf("unsupported test status $%02x at PC $%04x", code, cpu.PC)
		}
		var output []byte
		for address := uint16(0x6004); address < 0x8000; address++ {
			value := mapper.Read(address)
			if value == 0 {
				return code, strings.TrimSpace(string(output)), nil
			}
			output = append(output, value)
		}
		return 0, "", fmt.Errorf("unterminated result text")
	}
	return 0, "", fmt.Errorf("cycle limit exceeded at PC $%04x", cpu.PC)
}

// Strip the ROM's repeated title and result footer, keeping the diagnostic.
func blarggDiagnostic(name string, code byte, output string) string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == name || line == "Failed" || line == fmt.Sprintf("Failed #%d", code) {
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return fmt.Sprintf("code %d", code)
	}
	return strings.Join(lines, ": ")
}

// RunBlargg runs the standalone NTSC APU tests from the provided ROM directory.
// An unexpected result (including a pass that should retire a known failure)
// makes the suite fail without preventing the other ROMs from running.
func RunBlargg(directory string) bool {
	allExpected := true
	for _, test := range blarggROMs {
		name := "Blargg APU " + test.name
		code, output, err := runBlarggROM(filepath.Join(directory, test.name+".nes"))
		if err != nil {
			log.Printf("%s: %v", test_utils.Failure(name), err)
			allExpected = false
			continue
		}
		if code == 0 {
			log.Print(test_utils.Success(name))
		} else {
			log.Printf("%s: %s", test_utils.Failure(name), blarggDiagnostic(test.name, code, output))
		}
		if code != test.wantCode {
			log.Printf("%s: result %d differs from baseline %d", name, code, test.wantCode)
			allExpected = false
		}
	}
	return allExpected
}

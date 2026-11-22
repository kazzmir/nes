package aputest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func localAPUROM(t *testing.T, path string) string {
	t.Helper()
	path = filepath.Join("..", "..", "..", path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("local APU test ROM not installed: %s", path)
	} else if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAPUFrameCounterROMs(t *testing.T) {
	// Tests 9/10 remain known failures.
	for number := 1; number <= 8; number++ {
		t.Run(fmt.Sprintf("test_%d", number), func(t *testing.T) {
			test := apuTests[number-1]
			rom := localAPUROM(t, test.Rom)
			passed, err := doTest(rom, test.PassAddress, test.FailAddress)
			if err != nil {
				t.Fatal(err)
			}
			if !passed {
				t.Fatalf("ROM reached its failure loop at $%04x", test.FailAddress)
			}
		})
	}
}

func TestAPUROMCycleLimit(t *testing.T) {
	rom := localAPUROM(t, apuTests[2].Rom)
	// Neither endpoint occurs in this ROM's result loop; it must time out.
	passed, err := doTest(rom, 0x1234, 0x5678)
	if passed || err == nil || !strings.Contains(err.Error(), "cycle limit") {
		t.Fatalf("missing bounded failure: passed=%v err=%v", passed, err)
	}
}

func TestRunReportsAPUFailure(t *testing.T) {
	test := apuTests[2]
	test.Rom = localAPUROM(t, test.Rom)
	test.PassAddress, test.FailAddress = 0x1234, 0x81a4 // Reset vector: fail immediately.
	original := apuTests
	apuTests = []APUTest{test}
	defer func() { apuTests = original }()
	passed, err := Run(false)
	if passed || err != nil {
		t.Fatalf("suite hid ROM failure: passed=%v err=%v", passed, err)
	}
}

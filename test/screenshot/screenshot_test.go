package screenshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBadDudesLeavesGreyStartup(t *testing.T) {
	rom := filepath.Join("..", "..", "roms", "Bad Dudes (U).nes")
	if _, err := os.Stat(rom); os.IsNotExist(err) {
		t.Skip("local Bad Dudes ROM not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	// About 200 frames: enough to display the Data East logo. The old
	// immediate/batched NMI loop leaves every pixel the same grey forever.
	screen, err := Run(rom, 6_000_000)
	if err != nil {
		t.Fatal(err)
	}
	colors := make(map[uint32]bool)
	for _, pixel := range screen.Buffer {
		colors[pixel] = true
	}
	if len(colors) < 3 {
		t.Fatalf("startup did not draw its logo: got %d distinct colors", len(colors))
	}
}

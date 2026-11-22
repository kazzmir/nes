package screenshot

import (
	"path/filepath"
	"strconv"
	"testing"
)

func TestMMC5Screenshots(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "test-roms", "mmc5test")
	for _, cycles := range mmc5Checkpoints {
		t.Run(strconv.FormatInt(cycles, 10), func(t *testing.T) {
			if err := checkMMC5Screenshot(dir, cycles); err != nil {
				t.Fatal(err)
			}
		})
	}
}

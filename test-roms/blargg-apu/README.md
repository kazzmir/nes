# Blargg's NES APU tests

These eight standalone NTSC ROMs are by Shay Green (Blargg). The ROMs and
accompanying `readme.txt` come from the
[`apu_test` directory of nes-test-roms](https://github.com/christopherpow/nes-test-roms/tree/95d8f621ae55cee0d09b91519a8989ae0e64753b/apu_test),
which also contains the assembly source. The combined ROM is not needed.

From the repository root, run `go run ./test/all-test -apu` along with the
legacy APU suite, or `go test ./test/all-test/apu-test -run TestBlarggAPUROMs -v`
to run just these ROMs. The runner reads each ROM's status and diagnostic text
from cartridge RAM. `blargg.go` lists the current expected failure codes;
unexpected results, including passes for formerly failing ROMs, fail the run.
Update the list as fixes land.

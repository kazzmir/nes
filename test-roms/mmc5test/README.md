# MMC5 visual regression

`mmc5test.nes` is copied unchanged from the `mmc5test` directory of
[nes-test-roms](https://github.com/christopherpow/nes-test-roms/tree/95d8f62/mmc5test).

The five PNGs capture distinct frames at checkpoints of 180,000, 300,000,
600,000, 1,200,000, and 2,400,000 elapsed CPU cycles from reset, with no input.
Each checkpoint compares the last **completed frame**, not a partially
rendered frame. These are golden images from this emulator (including its
palette), not FCEUX screenshots or a full MMC5 hardware conformance test.

Run the screenshot suite from the repository root with
`go run ./test/all-test -screenshot`, or just these five comparisons with
`go test ./test/all-test/screenshot -run TestMMC5Screenshots -v`.

To regenerate a baseline intentionally, run
`go run ./test/make-screenshot test-roms/mmc5test/mmc5test.nes <cycles>` and
move `images/mmc5test-<cycles>.png` to `test-roms/mmc5test/<cycles>.png`.

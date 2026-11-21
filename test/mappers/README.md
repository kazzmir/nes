# Mapper regression checker

Run from the repository root:

```sh
go run ./test/mappers
# Or supply another mapper-9 ROM:
go run ./test/mappers -rom "path/to/game.nes"
```

The default cartridge is the local file:

`roms/Mike Tyson's Punch-Out!! (U) (PRG 1) [h1].nes`

Its iNES header declares mapper 9 (MMC2), 128 KiB PRG-ROM, and 128 KiB
CHR-ROM. The checker uses four distinguishable CHR banks from this cartridge.
It does not modify the ROM, execute its game code, or require a display/audio
device. ROM contents are not bundled with the checker.

## MMC2 latch checks

All mapper writes and PPU reads go through CPU-visible memory ports. The checker
accounts for the PPU startup write-inhibit interval and the $2007 read buffer.
It verifies:

- Lower-table FD/FE selection at exactly $0FD8/$0FE8.
- Nearby lower-table addresses do not trigger the latches.
- Upper-table selection at every address in $1FD8–$1FDF and $1FE8–$1FEF.
- Reads immediately outside the upper ranges do not trigger the latches.
- Switching one pattern table leaves the other unchanged.
- Writes to active/inactive upper CHR registers respect the selected latch.

Comparisons read a prefix of each pattern table, avoiding latch addresses so
that verification itself cannot change latch state. Power-on latch values are
not assumed. This isolates the banking defect; it is not a gameplay, rendering-
fetch, or precise latch-switch-timing test.

Exit codes: **0** = all assertions passed, **1** = regression detected,
**2** = invalid input/setup error. The default ROM now passes **52/52 assertions**.
Before MMC2 latch support was implemented, it failed 18 of these assertions.

The unit tests include the same latch regression using synthetic CHR banks, as
well as probe, bank-selection, and input-validation tests. They do not require a
local ROM:

```sh
go test ./test/mappers ./lib
```

`lib/mapper_test.go` additionally checks triggering-read ordering, background
and sprite fetches, independent copies, and JSON save-state restoration.

# Mapper bug tracker

Findings from the mapper review. Checked items are implemented; unchecked items
remain open. Function references are preferred over line numbers so this list
survives edits. Hardware-dependent behavior should be covered by board-specific
regression tests before marking an item fixed.

## Fixed

- [x] **MMC1B/C: PRG-RAM disable was ignored.** PRG register bit 4 now
  disables reads and writes at $6000–7FFF without changing RAM contents.
  Disabled CPU reads return open bus through an optional `OpenBusMapper`
  capability; clearing bit 4 restores access. Only bits 0–3 select PRG-ROM.
  RAM starts enabled for compatibility, and serial reset leaves the disable
  bit unchanged.
  - Code: `lib/mapper.go` (`Mapper1.ReadWithOpenBus`, `Read`, `Write`),
    `lib/cpu.go` (generic open-bus-aware cartridge reads).
  - Tests: `lib/mapper_test.go` covers RAM boundaries, ignored writes, open
    bus, fifth-write commit timing, reenable/reset, all PRG modes and bank
    values through 512 KiB, copies/save states, and the optional read hook.
  - Revision scope: standard MMC1B/C behavior. Early MMC1A lacks RAM disable;
    identifying/selecting that revision is not implemented by the loader.
- [x] **MMC1: control changes left stale CHR-ROM mappings.** A completed
  control write rebuilds both pattern tables using the stored CHR registers
  and the new 4/8 KiB mode. Shared copy logic keeps ROM offsets wide and
  accepts slices ending exactly at the ROM boundary; CHR-RAM is not copied.
  - Code: `lib/mapper.go` (`Mapper1.SetChrBank`, `copyChrBank`, `Write`).
  - Tests: `lib/mapper_test.go` covers both mode-change directions without
    rewriting bank registers, inactive CHR1 writes, odd/high/final banks,
    partial serial writes, CPU-visible reads, and CHR-RAM preservation.
- [x] **MMC1: last valid 4 KiB CHR bank was rejected.** Allow the source slice
  end to equal the CHR-ROM length in both CHR bank-register handlers.
  - Code: `lib/mapper.go` (`Mapper1.Write`).
  - Tests: `lib/mapper_test.go` checks all 32 banks in both registers and
    the last bank of 8/16/32/64/128 KiB CHR-ROMs, including CPU-visible PPU
    reads at each end and preservation of the other pattern table.
- [x] **CNROM: 16 KiB PRG-ROM was not mirrored at $C000–FFFF.** Mask A14
  for 16 KiB PRG-ROMs while retaining the full mapping for 32 KiB ROMs.
  This also makes the reset, NMI, and IRQ vectors accessible.
  - Code: `lib/mapper.go` (`Mapper3.Read`).
  - Tests: `lib/mapper_test.go` checks the entire CPU PRG range for both ROM
    sizes, unchanged PRG mapping during CHR switching, mirrored interrupt
    vectors, and CPU execution from the mirrored half.
  - Local ROM: `roms/Joust (U) .nes` has 16 KiB PRG-ROM. Previously its reset
    vector read as zero, leaving execution stuck at $0000.
- [x] **MMC3: PRG bank selections outside the physical ROM returned zero.**
  Alias selected 8 KiB banks to available PRG-ROM banks at read time, without
  changing the latched register bits. This models disconnected address lines
  on standard power-of-two MMC3 PRG-ROM sizes.
  - Code: `lib/mapper.go` (`Mapper4.ReadBank`).
  - Tests: `lib/mapper_test.go` covers all 256 register write values, both PRG
    modes, ROM sizes from 32 to 512 KiB, all slot boundaries, and empty ROMs.
    Batman-specific tests exercise $1C/$1D and $12/$13 on a 128 KiB ROM.
  - Local ROM: `roms/Batman (U) .nes`. Previously selected nonexistent PRG
    banks and stayed black; correct aliasing allows startup and gameplay.
- [x] **MMC3B/C: IRQ reload semantics were incorrect.** $C001 ignores the
  written value and requests a one-shot reload from the $C000 latch on the
  next counter clock. Assert IRQ when the resulting counter is zero and IRQ
  is enabled, including zero-valued reloads. Preserve reload requests across
  copies/save states; $E000 acknowledges/disables IRQ without canceling reload.
  - Code: `lib/mapper.go` (`Mapper4.Write`, `Scanline`, `Copy`).
  - Tests: `lib/mapper_test.go` covers ignored values/aliases, latest-latch
    selection, repeated reload requests, zero reloads, enable/acknowledge,
    and copies/save states.
  - Revision scope: this implements standard MMC3B/C behavior. MMC3A's
    alternate zero-reload IRQ behavior is not modeled.
- [x] **MMC3: register aliases were ignored.** Decode register writes with
  `address & 0xE001`, after handling PRG-RAM with its original address.
  - Code: `lib/mapper.go` (`Mapper4.Write`).
  - Tests: `lib/mapper_test.go` checks every alias of all eight register ports,
    all eight bank-data selections, PPU mapping effects, and PRG-RAM addressing
    and write protection. IRQ reload semantics are covered separately above.
- [x] **MMC1: consecutive-cycle serial writes were accepted.** Ignore data
  writes within one cycle of the preceding register write, including across
  register addresses. Ignored writes extend the inhibit window; bit-7 reset
  writes are always honored. PRG-RAM writes do not affect the window.
  - Code: `lib/mapper.go` (`Mapper1.Write`, `Copy`). Save states preserve the
    last-write timestamp and whether it is valid.
  - CPU timing: RMW writes currently share an instruction-level timestamp;
    same-timestamp data writes are also filtered. This is not a full
    cycle-by-cycle CPU bus implementation.
  - Tests: `lib/mapper_test.go` covers timing boundaries, ordinary CPU serial
    writes, official/unofficial absolute and absolute-X RMW instructions,
    RMW reset cases, and copies/save states.
- [x] **MMC1: serial reset did not restore PRG mode 3.** Bit-7 writes clear
  the partial serial write and apply control `| 0x0C`, immediately selecting
  PRG mode 3 without changing mirroring, CHR mode, or bank registers.
  - Code: `lib/mapper.go` (`Mapper1.Write`).
  - Tests: `lib/mapper_test.go` covers all 32 control values, reset writes in
    all four register ranges, values $80/$FF, immediate PRG mapping, and a
    fresh five-bit serial write after reset.
- [x] **MMC1: CHR-RAM bank writes copied PRG-ROM into graphics memory.**
  Keep the physical 8 KiB RAM in PPU pattern memory; map CPU/rendering reads
  and CPU writes through MMC1's CHR registers. In 4 KiB mode only bank bit 0
  selects RAM, allowing swaps and aliases; 8 KiB mode maps RAM directly.
  Higher CHR bits remain available for SUROM PRG selection without touching RAM.
  - Code: `lib/mapper.go` (`Mapper1.ReadCharacter`, `WriteCharacter`, `Write`),
    `lib/ppu.go` (optional `WritableCharacterMapper` routing).
  - Tests: `lib/mapper_test.go` covers all CHR register values in both modes,
    writable aliases, mode changes, SUROM outer PRG banking, copies/save states.
- [x] **MMC1: CHR offset arithmetic overflow.** Compute CHR offsets in `int`
  before multiplication in both 4 KiB and 8 KiB modes, so offsets and slice ends
  do not wrap at 64 KiB. The separate last-bank bounds-check fix is listed above.
  - Code: `lib/mapper.go` (`Mapper1.Write`).
  - Tests: `lib/mapper_test.go` covers both 4 KiB registers near/above 64 KiB
    and all 32 register values in 8 KiB mode, including odd values and banks
    ending exactly at 64 KiB or 128 KiB.
- [x] **MMC2 (mapper 9): missing CHR latches.** Implement independent FD/FE
  selections for both pattern tables, exact lower-table trigger addresses, and
  inclusive upper-table trigger ranges. The triggering read returns the old
  bank's byte. Connect CPU $2007 reads and background/sprite pattern fetches;
  preserve latch state across copies and save states.
  - Code: `lib/mapper.go` (`Mapper9.ReadCharacter`, `SetChrBank`, `Copy`),
    `lib/ppu.go` (pattern fetches).
  - Tests: `lib/mapper_test.go`, `test/mappers/`.
  - Local ROM: `roms/Mike Tyson's Punch-Out!! (U) (PRG 1) [h1].nes`.
    The standalone checker now passes 52/52 assertions (previously 18 failed).
  - Fix: `7237632`.
- [x] **MMC2: unmasked bank registers and unsafe CHR bank slicing.** Mask PRG
  registers to four bits and CHR registers to five bits; wrap CHR selection to
  available banks instead of slicing outside CHR-ROM.
  - Code: `lib/mapper.go` (`Mapper9.Write`, `CharacterBlock`).
  - Fix: `7237632`. Add dedicated disconnected-bit/wraparound tests below.

The PPU's optional character-read and scanline interfaces are bound outside
`PPU.Run()`; scanline binding/copy/restore tests are in `lib/mapper_test.go`.
Refactor: `179265c`. This does **not** fix MMC3 IRQ accuracy.

## Open

### NROM (mapper 0)

- [ ] **Copies lose cartridge RAM.** `Mapper0.Copy` omits `SaveRam`, leaving
  copied cartridges with a nil RAM slice that can panic on reads/writes.
  Copy the RAM and test independent mutations of original and copy.

### MMC1 (mapper 1)

- [ ] **512 KiB PRG banking is wrong in 32 KiB mode.** `Mapper1.Read` does not
  apply the SUROM outer-bank bit in this mode. RAM-disable bit separation is
  fixed above; outer-bank selection still needs tests and a fix for both halves.

### MMC3 (mapper 4)

- [ ] **IRQ clocks use scanline boundaries rather than qualified PPU A12
  rising edges.** Replace the approximation in `lib/ppu.go` with appropriate
  bus-edge timing/filtering. Test pattern-table configurations and
  mid-scanline changes, not just one clock per rendered scanline.
- [ ] **Four-screen mirroring is not preserved.** Parse and retain the header's
  four-screen flag in `lib/nes.go`; prevent MMC3 $A000 writes from overriding
  hardwired four-screen mirroring. Check PPU nametable storage/mapping too.

### Bank selection across mappers

- [ ] **Respect disconnected bank address bits / available ROM size.** Review
  `Mapper2.Write`/`Read`, `Mapper3.Write`, `Mapper4.CharacterBlock`, and
  `Mapper7.ReadBank`/`Write`. MMC3 PRG aliasing is fixed above; CHR aliasing and
  other mappers remain open. Hardware-supported selections currently can
  return zero, leave stale graphics, or panic instead of aliasing an existing
  bank. Model the actual board's wiring; also account for bus conflicts where
  applicable rather than applying one unconditional rule to every board.
- [ ] **MMC2 PRG reads can still index outside ROM.** Register masking is
  fixed, but `Mapper9.ReadBank` still assumes the selected PRG bank exists.
  Handle disconnected PRG address bits for smaller supported ROMs and test
  the behavior separately from CHR latches.
- [ ] **Add dedicated MMC2 register-mask/CHR-wrap tests.** The latch checker
  deliberately selects valid, distinguishable banks; it does not exercise
  the register masking and CHR wraparound fix.

## Validation

From the repository root:

```sh
go test ./lib/... ./test/mappers
go run ./test/mappers
```

The standalone checker requires the local mapper-9 ROM (or `-rom path/to/rom`).
The Go unit tests use synthetic cartridge data and do not require ROM files.
The checker isolates latch banking; it is not proof of cycle-accurate rendering
or complete MMC2 gameplay compatibility.

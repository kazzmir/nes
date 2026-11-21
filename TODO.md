# Mapper bug tracker

Findings from the mapper review. Checked items are implemented; unchecked items
remain open. Function references are preferred over line numbers so this list
survives edits. Hardware-dependent behavior should be covered by board-specific
regression tests before marking an item fixed.

## Fixed

- [x] **Disconnected ROM bank address lines were not respected.** Standard
  power-of-two ROM sizes now mask unconnected bank bits at access time:
  UxROM PRG, CNROM CHR, MMC3 CHR, AxROM PRG, and MMC2 PRG/CHR. MMC3 PRG
  uses the same resolver. MMC3's aligned 2 KiB CHR selections and MMC2's
  four/five-bit register masks remain separate from physical ROM aliasing.
  CNROM/AxROM can also select connected higher bits on larger ROM boards.
  - Code: `lib/mapper.go` (`mirrorBank`, mapper 2/3/4/7/9 access paths).
    Empty images and unsupported bank holes are bounds-checked rather than
    panicking; non-power-of-two images do not get invented modulo aliases.
  - Board wiring: NES 2.0 submapper 1 selects no bus conflicts and submapper
    2 selects bus conflicts for mappers 2/3/7. Conflicting writes AND the CPU
    value with the OLD PRG mapping, including AxROM's mirror bit and the CPU
    data bus. Copies/save states retain this board setting.
  - Loader: `lib/nes.go` preserves the NES 2.0 submapper and full 12-bit
    mapper ID. `NESFile.MakeMapper` configures the board in the emulator and
    screenshot loader. Unknown discrete-board submappers are rejected.
  - Compatibility: legacy iNES and unspecified submapper 0 retain the
    existing no-conflict default; they do not identify the physical board.
  - Tests: `lib/mapper_test.go` covers all 256 write values across multiple
    ROM sizes, both MMC3 CHR modes, MMC2 latch selections/register masks,
    fixed PRG windows, bus-conflict variants, old-bank bus values, metadata,
    empty/irregular images, and copy/save-state preservation.
  - Fix: `d21bac1`.
- [x] **MMC2 PRG reads could index outside ROM.** Alias disconnected bank
  bits and bounds-check reads, including malformed/empty images and addresses
  below the PRG-ROM window. This does not add MMC2 cartridge-RAM emulation.
  - Code: `lib/mapper.go` (`Mapper9.ReadBank`, `Read`).
  - Tests: `lib/mapper_test.go` (`TestDiscreteAndMMC2PRGBankAddressLines`,
    `TestBankAddressLinesEmptyAndNonPowerOfTwo`).
- [x] **Dedicated MMC2 register-mask/CHR-wrap tests were missing.**
  `TestMMC2CHRRegisterMasksAndAddressLines` now exercises all CHR register
  values, both latch selections in both tables, and ROM sizes 8–128 KiB.
  PRG register masking and small-ROM aliases have separate coverage above.
- [x] **MMC3: four-screen mirroring was not preserved.** Parse iNES header
  bit 3 with priority over the H/V bit, configure four independent 1 KiB
  nametables backed by 4 KiB RAM, and ignore MMC3 $A000 mirroring writes
  (including aliases) on four-screen boards. Normal H/V cartridges retain
  their existing behavior and 2 KiB allocation.
  - Code: `lib/nes.go` (`NESFile.ConfigureMirroring`, `ParseNes`),
    `lib/ppu.go` (`SetFourScreenMirror`, nametable mapping), `lib/mapper.go`
    (`Mapper4.Write`). Both the emulator and screenshot loader apply the
    shared header configuration.
  - Tests: `lib/mapper_test.go` covers header precedence, all 4 KiB of CPU-
    visible nametable space, $3000–$3EFF shadows (including the corrected
    end boundary), palette reads/buffering, all mirroring aliases, PRG-RAM
    control, repeated configuration, copies, and save states.
  - Local ROM: `roms/Gauntlet (U) .nes` declares four-screen mode. A headless
    run through the screenshot loader reached its title screen after ten
    million CPU cycles with four-screen configuration and no warnings.
- [x] **MMC1/SUROM: 512 KiB PRG banking ignored the outer bit in 32 KiB mode.**
  Combine CHR0 bit 4 with the four PRG-ROM bank bits before selecting the
  32 KiB pair. PRG bit 0 remains ignored in this mode, and PRG bit 4 remains
  a RAM-disable control rather than a ROM bank bit.
  - Code: `lib/mapper.go` (`Mapper1.ReadWithOpenBus`).
  - Tests: `lib/mapper_test.go` covers both ROM halves, all PRG modes, both
    CHR modes with matching outer bits, all 32 PRG register values, 256 KiB
    aliasing, immediate outer-bank changes, and copies/save states.
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
  - Fix: `7237632`. Dedicated mask/address-line tests are now listed above.

The PPU's optional character-read and scanline interfaces are bound outside
`PPU.Run()`; scanline binding/copy/restore tests are in `lib/mapper_test.go`.
Refactor: `179265c`. This does **not** fix MMC3 IRQ accuracy.

## Open

### NROM (mapper 0)

- [ ] **Copies lose cartridge RAM.** `Mapper0.Copy` omits `SaveRam`, leaving
  copied cartridges with a nil RAM slice that can panic on reads/writes.
  Copy the RAM and test independent mutations of original and copy.

### MMC3 (mapper 4)

- [ ] **IRQ clocks use scanline boundaries rather than qualified PPU A12
  rising edges.** Replace the approximation in `lib/ppu.go` with appropriate
  bus-edge timing/filtering. Test pattern-table configurations and
  mid-scanline changes, not just one clock per rendered scanline.

### Bank selection across mappers

- [ ] **Identify ambiguous legacy boards and irregular ROM wiring.** iNES
  and NES 2.0 submapper 0 do not distinguish conflicting/non-conflicting
  discrete boards. Add reliable board identification or explicit configuration
  before changing their compatibility default. Non-power-of-two ROM layouts
  also need board-specific mapping; current fallback bounds-checks bank holes
  instead of guessing modulo wiring.

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

After the bank-address-line changes, headless runs through the screenshot
loader completed ten million CPU cycles each for Contra, Joust, Battletoads,
and the local Mike Tyson's Punch-Out!! ROM without CPU errors or warnings.
NES 2.0 bus-conflict variants are covered by synthetic header/board tests;
the local ROM collection does not provide identified submapper-2 examples.

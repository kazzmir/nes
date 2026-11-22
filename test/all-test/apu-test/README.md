# APU frame-counter ROM tests

The local ROMs and assembly listings live in `test-roms/apu/`. They are not
bundled with the Go regression tests; tests skip when the ROMs are absent.

From the repository root, check test 3 specifically:

```sh
go test ./test/all-test/apu-test -run 'TestAPUFrameCounterROMs/test_3$' -v
```

Run the synthetic unit tests and ROM regressions:

```sh
go test ./lib -run TestAPU -v
go test ./test/all-test/apu-test -v
```

## Test 3 fix

`test_3.nes` expects a pulse length-counter clock just before a delayed $4017
frame reset, followed by the five-step reset's own quarter/half-frame clock.
The old `APU.Run` reset the sequencer before processing elapsed frame time,
losing the preceding clock and leaving the pulse active at $8154. The ROM
then reached its failure loop at $8166 instead of its pass loop at $815B.

The fix advances the old sequence up to the reset, gives the reset priority
at a coincident boundary, and advances any remaining time afterward. It also
uses a parity-dependent 3/4 CPU-cycle reset delay and preserves pending mode
changes in copies/save states. ROM bytes and expected result addresses are
unchanged.

Verified results:

| ROMs | Before | After |
| --- | --- | --- |
| 1, 2, 7, 8 | Passed | Passed |
| 3, 4 | Failed | Passed |
| 5, 6, 9, 10 | Failed | Still failing |

## Tests 5 and 6 fix

The five-step sequence now clocks length at CPU cycles 14913 and 37281,
with no clock at 29829. A delayed `$4017` reset also cannot clock the frame
units again just one CPU cycle after a sequencer tick. The old code could
exhaust the pulse length counter before these ROMs read `$4015`.

Tests 1–8 now pass; 9 and 10 still fail. Four-step timing remains approximate,
and CPU/APU register accesses are still instruction-batched.

`doTest` now has a two-second emulated-cycle limit rather than hanging on an
unexpected execution path. `Run` returns false if any ROM fails, instead of
always returning true. The legacy `go run ./test/all-test -apu` command still
logs the full ten-ROM results, but its main program does not use that boolean
for its exit status; use the Go regression tests for automated confirmation.

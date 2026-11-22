# Klaus Dormann 6502 functional test for NES

`6502_functional_test.nes` is a mapper-0 (NROM-256) ROM adapted from
[Klaus Dormann's test suite](https://github.com/Klaus2m5/6502_65C02_functional_tests/tree/7954e2dbb49c469ea286070bf46cdd71aeb29e4b).
The corresponding assembly source and GPL license are included here.

```sh
go run ./test/all-test -cpu
go test ./test/all-test/cpu-test -v
```

There is no video output. In a debugger, `$FFF0` is the success loop; any
other self-loop is a failure, with the active test number at `$0200`.
The ROM boots at `$8000`, initializes NES RAM, and copies the self-modifying
branch-range test into RAM at `$0300`. It tests actual `BRK`/`RTI` using
read-only cartridge vectors. The NES Ricoh 2A03 stores the decimal flag but
ignores it for arithmetic, so the upstream BCD arithmetic cases are disabled.
The separate BCD and 65C02 suites are not applicable to the NES. The separate
interrupt suite expects a custom external IRQ/NMI feedback port, which this
NROM cartridge does not provide.

To rebuild, pass the absolute path to the upstream AS65 1.42 assembler (or a
wrapper command for it):

```sh
python3 test-roms/klaus-6502/build.py /absolute/path/to/as65
```

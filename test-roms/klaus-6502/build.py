#!/usr/bin/env python3
"""Rebuild the NES fixture with AS65 1.42 (or a wrapper for it)."""

from pathlib import Path
import subprocess
import sys
import tempfile


def main():
    root = Path(__file__).resolve().parent
    with tempfile.TemporaryDirectory() as directory:
        source = Path(directory) / "6502_functional_test.a65"
        source.write_bytes((root / source.name).read_bytes())
        subprocess.run([*sys.argv[1:], "-s2", source.name], cwd=directory, check=True)
        records = source.with_suffix(".hex").read_text().splitlines()

    prg = bytearray([0xFF]) * 0x8000
    used = set()
    for line in records:
        if not line.startswith(":"):
            raise ValueError(f"invalid Intel HEX record: {line}")
        record = bytes.fromhex(line[1:])
        count, high, low, kind = record[:4]
        if len(record) != count + 5 or sum(record) % 256:
            raise ValueError(f"invalid Intel HEX record: {line}")
        if kind == 1:
            break
        if kind != 0:
            raise ValueError(f"unsupported Intel HEX record: {line}")
        address = high * 256 + low
        if 0x0300 <= address < 0x0500 and address + count <= 0x0500:
            address += 0xF000 - 0x0300  # Startup copies this section to RAM.
        if address < 0x8000 or address + count > 0x10000:
            raise ValueError(f"record outside cartridge: {line}")
        offset = address - 0x8000
        span = range(offset, offset + count)
        if used.intersection(span):
            raise ValueError(f"overlapping cartridge record: {line}")
        used.update(span)
        prg[offset:offset + count] = record[4:-1]
    if prg[0x7FF0:0x7FF3] != b"\x4c\xf0\xff" or prg[0x7FFC:0x7FFE] != b"\x00\x80":
        raise ValueError("missing PASS loop or reset vector")
    (root / "6502_functional_test.nes").write_bytes(b"NES\x1a\x02" + bytes(11) + prg)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        sys.exit("usage: python3 build.py /absolute/path/to/as65 [wrapper args...]")
    main()

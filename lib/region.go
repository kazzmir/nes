package lib

import (
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strings"
)

// Region selects the hardware timing. Zero is NTSC for old save states.
type Region uint8

const (
	RegionNTSC Region = iota
	RegionPAL
)

func (r Region) String() string {
	if r == RegionPAL {
		return "PAL"
	}
	return "NTSC"
}

func ParseRegionOverride(value string) (*Region, error) {
	var r Region
	switch strings.ToLower(value) {
	case "auto":
		return nil, nil
	case "ntsc":
		r = RegionNTSC
	case "pal":
		r = RegionPAL
	default:
		return nil, fmt.Errorf("invalid region %q (expected auto, ntsc, or pal)", value)
	}
	return &r, nil
}

type regionTiming struct {
	cpuHz                        float64
	scanlines                    int
	ppuNumerator, ppuDenominator uint64
}

func (r Region) timing() regionTiming {
	if r == RegionPAL {
		return regionTiming{1662607.125, 312, 16, 5}
	}
	return regionTiming{CPUSpeed, 262, 3, 1}
}

func detectRegion(header, prg, chr []byte, name string) Region {
	if isNes2(header) {
		switch header[12] & 3 {
		case 0:
			return RegionNTSC
		case 1:
			return RegionPAL
		case 3:
			return RegionNTSC // Dendy is distinct hardware; currently unsupported.
		}
		// Multi-region: continue to the cartridge and filename hints.
	}
	crc := crc32.Update(crc32.ChecksumIEEE(prg), crc32.IEEETable, chr)
	// CRCs cover PRG followed by CHR, not the header or trainer.
	switch crc {
	case 0x9a2db086: // Super Mario Bros. (Europe), confirmed with FCEUX.
		return RegionPAL
	}
	if !isNes2(header) && !isDiskDude(header) && header[9]&1 != 0 {
		return RegionPAL
	}
	base := strings.ToLower(filepath.Base(name))
	for _, tag := range []string{"(e)", "(europe)", "(pal)", "(f)", "(g)", "(i)", "(france)", "(germany)", "(italy)"} {
		if strings.Contains(base, tag) {
			return RegionPAL
		}
	}
	return RegionNTSC
}

package lib

type PPUState struct {
    Flags byte
    Mask byte
    Status byte
    Scanline int
    Cycle uint64
}

func (ppu *PPUState) SetControllerFlags(value byte) {
    ppu.Flags = value
}

func (ppu *PPUState) SetMask(value byte) {
    ppu.Mask = value
}

func (ppu *PPUState) SetVerticalBlankFlag(on bool){
    bit := byte(1<<7)
    if on {
        ppu.Status = ppu.Status | bit
    } else {
        ppu.Status = ppu.Status & (^bit)
    }
}

func (ppu *PPUState) ReadStatus() byte {
    out := ppu.Status
    ppu.SetVerticalBlankFlag(false)
    return out
}

/* given a number of cycles to process */
func (ppu *PPUState) Run(cycles uint64) {
    ppu.Cycle += cycles
    cyclesPerScanline := uint64(113)
    /* number of cycles per scanline is actually 113 and 2/3 */
    for ppu.Cycle > cyclesPerScanline {
        ppu.Scanline += 1
        if ppu.Scanline == 241 {
            ppu.SetVerticalBlankFlag(true)
        }
        if ppu.Scanline == 242 {
            ppu.Scanline = 0
            ppu.SetVerticalBlankFlag(false)
        }

        ppu.Cycle -= cyclesPerScanline
    }
}

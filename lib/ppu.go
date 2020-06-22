package lib

import (
    "log"
)

type PPUState struct {
    Flags byte
    Mask byte
    Status byte
    Scanline int
    Cycle uint64
    VideoAddress uint16
    WriteState byte
}

func (ppu *PPUState) SetControllerFlags(value byte) {
    ppu.Flags = value
}

func (ppu *PPUState) SetMask(value byte) {
    ppu.Mask = value
}

func (ppu *PPUState) GetNMIOutput() bool {
    return ppu.Flags & (1<<7) == 1<<7
}

func (ppu *PPUState) WriteAddress(value byte){
    switch ppu.WriteState {
        /* write high byte */
        case 0:
            /* highest available page is 3f. after that it is mirrored down */
            ppu.VideoAddress = uint16(value & 0x3f) << 8
            ppu.WriteState = 1
        case 1:
            ppu.VideoAddress = ppu.VideoAddress | uint16(value)
            ppu.WriteState = 0
    }
}

func (ppu *PPUState) WriteData(value byte){
    log.Printf("Writing 0x%x to PPU at 0x%x\n", value, ppu.VideoAddress)
    ppu.VideoAddress += 2
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

/* give a number of PPU cycles to process
 * returns whether nmi is set or not
 */
func (ppu *PPUState) Run(cycles uint64) bool {
    /* http://wiki.nesdev.com/w/index.php/PPU_rendering */
    ppu.Cycle += cycles
    cyclesPerScanline := uint64(341)
    nmi := false
    for ppu.Cycle > cyclesPerScanline {
        ppu.Scanline += 1
        if ppu.Scanline == 241 {
            /* FIXME: this happens on the second tick after transitioning to scanline 241 */
            log.Printf("Set vertical blank to true\n")
            ppu.SetVerticalBlankFlag(true)
            /* Only set NMI to true if the bit 7 of PPUCTRL is set */
            nmi = ppu.GetNMIOutput()
        }
        if ppu.Scanline == 260 {
            ppu.Scanline = 0
            ppu.SetVerticalBlankFlag(false)
        }

        ppu.Cycle -= cyclesPerScanline
    }

    return nmi
}

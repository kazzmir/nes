package lib

import (
    "log"
    "fmt"
    "time"

    "github.com/veandco/go-sdl2/sdl"
)

type PPUState struct {
    Flags byte
    Mask byte
    Status byte
    Scanline int
    Cycle uint64
    VideoAddress uint16
    WriteState byte

    /* for scrolling */
    FineX byte

    VideoMemory []byte
}

func MakePPU() PPUState {
    return PPUState{
        VideoMemory: make([]byte, 64 * 1024),
    }
}

func (ppu *PPUState) WriteScroll(value byte){
    switch ppu.WriteState {
        case 0:
            courseX := value >> 3
            ppu.VideoAddress = uint16(courseX)
            /* lowest 3 bits of the value are the fine x */
            ppu.FineX = value & 0x7
        case 1:
            y := value & 0x7
            ppu.VideoAddress = ppu.VideoAddress | (uint16(y) << 12)
            coarseY := value >> 3
            ppu.VideoAddress = ppu.VideoAddress | (uint16(coarseY) << 5)
    }
}

func (ppu *PPUState) SetControllerFlags(value byte) {
    ppu.Flags = value
}

func (ppu *PPUState) GetNameTableBaseAddress() uint16 {
    base_nametable_index := ppu.Flags & 0x3

    switch base_nametable_index {
        case 0: return 0x2000
        case 1: return 0x2400
        case 2: return 0x2800
        case 3: return 0x2c00
    }

    return 0x2000
}

func (ppu *PPUState) GetBackgroundPatternTableBase() uint16 {
    background_table_index := (ppu.Flags >> 4) & 0x1 == 0x1

    switch background_table_index {
        case true: return 0x1000
        case false: return 0x0000
    }

    return 0x0000
}

func (ppu *PPUState) ControlString() string {
    vram_increment_index := (ppu.Flags >> 2) & 0x1 == 0x1
    sprite_table_index := (ppu.Flags >> 3) & 0x1 == 0x1
    background_table_index := (ppu.Flags >> 4) & 0x1 == 0x1
    sprite_size_index := (ppu.Flags >> 5) & 0x1 == 0x1
    master_slave_index := (ppu.Flags >> 6) & 0x1 == 0x1
    nmi := (ppu.Flags >> 7) & 0x1 == 0x1

    base_nametable_address := ppu.GetNameTableBaseAddress()

    var vram_increment int
    switch vram_increment_index {
        case true: vram_increment = 32
        case false: vram_increment = 1
    }

    var sprite_table uint16
    switch sprite_table_index {
        case true: sprite_table = 0x1000
        case false: sprite_table = 0x0000
    }

    var background_table uint16
    switch background_table_index {
        case true: background_table = 0x1000
        case false: background_table = 0x0000
    }

    var sprite_size string
    switch sprite_size_index {
        case true: sprite_size = "8x16"
        case false: sprite_size = "8x8"
    }

    var master_slave string
    switch master_slave_index {
        case true: master_slave = "output on ext"
        case false: master_slave = "read from ext"
    }

    return fmt.Sprintf("Nametable=0x%x Vram-increment=%v Sprite-table=%v Background-table=%v Sprite-size=%v Master/slave=%v NMI=%v", base_nametable_address, vram_increment, sprite_table, background_table, sprite_size, master_slave, nmi)
}

func (ppu *PPUState) CopyCharacterRom(data []byte) {
    for i := 0; i < len(data); i++ {
        ppu.VideoMemory[i] = data[i]
    }
}

func (ppu *PPUState) IsBackgroundEnabled() bool {
    /* FIXME: also include background_leftmost_8? */
    background := (ppu.Mask >> 3) & 0x1 == 0x1
    return background
}

func (ppu *PPUState) MaskString() string {
    greyscale := ppu.Mask & 0x1 == 0x1
    background_leftmost_8 := (ppu.Mask >> 1) & 0x1 == 0x1
    sprite_leftmost_8 := (ppu.Mask >> 2) & 0x1 == 0x1
    background := (ppu.Mask >> 3) & 0x1 == 0x1
    sprite := (ppu.Mask >> 4) & 0x1 == 0x1
    emphasize_red := (ppu.Mask >> 5) & 0x1 == 0x1
    emphasize_green := (ppu.Mask >> 6) & 0x1 == 0x1
    emphasize_blue := (ppu.Mask >> 7) & 0x1 == 0x1

    return fmt.Sprintf("Greyscale=%v Background-leftmost-8-pixels=%v sprite-leftmost-8-pixels=%v background=%v sprite=%v red=%v green=%v blue=%v", greyscale, background_leftmost_8, sprite_leftmost_8, background, sprite, emphasize_red, emphasize_green, emphasize_blue)
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

func (ppu *PPUState) GetVRamIncrement() uint16 {
    switch (ppu.Flags >> 2) & 0x1 {
        case 0: return 1
        case 1: return 32
    }

    return 1
}

func (ppu *PPUState) WriteData(value byte){
    log.Printf("Writing 0x%x to PPU at 0x%x\n", value, ppu.VideoAddress)
    ppu.VideoMemory[ppu.VideoAddress] = value
    ppu.VideoAddress += ppu.GetVRamIncrement()
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
    ppu.WriteState = 0
    return out
}

func (ppu *PPUState) Render(renderer *sdl.Renderer) {
    if ppu.IsBackgroundEnabled() {
        renderer.SetDrawColor(0, 0, 0, 255)
        renderer.Clear()
        renderer.SetScale(2, 2)
        log.Printf("Render background")
        nametableBase := ppu.GetNameTableBaseAddress()

        attributeTableBase := nametableBase + 0x3c0
        _ = attributeTableBase

        patternTable := ppu.GetBackgroundPatternTableBase()

        /*
        palette := [][]uint8{
            []uint8{255, 255, 255, 255},
            []uint8{255, 0, 0, 255},
            []uint8{0, 255, 0, 255},
            []uint8{0, 0, 255, 255},
        }
        */

        /* blargg's 2c02 palette
         *   http://wiki.nesdev.com/w/index.php/PPU_palettes
         */
        palette := [][]uint8{
            []uint8{84, 84, 84}, // 00
            []uint8{0, 30, 116}, // 01
            []uint8{8, 16, 144}, // 02
            []uint8{48, 0, 136}, // 03
            []uint8{68, 0, 100}, // 04
            []uint8{92, 0, 48},  // 05
            []uint8{84, 4, 0},   // 06
            []uint8{60, 24, 0},  // 07
            []uint8{32, 42, 0},  // 08
            []uint8{8, 58, 0},   // 09
            []uint8{0, 64, 0},   // 0a
            []uint8{0, 60, 0},   // 0b
            []uint8{0, 50, 60},  // 0c
            []uint8{0, 0, 0},    // 0d
            []uint8{152, 150, 152}, // 0e
            []uint8{8, 76, 196},   // 0f
            []uint8{48, 50, 236}, // 10
            []uint8{92, 30, 228}, // 11
            []uint8{136, 20, 176},
            []uint8{160, 20, 100},
            []uint8{152, 34, 32},
            []uint8{120, 60, 0},
            []uint8{84, 90, 0},
            []uint8{40, 114, 0},
            []uint8{8, 124, 0},
            []uint8{0, 118, 40},
            []uint8{0, 102, 120},
            []uint8{0, 0, 0},
            []uint8{236, 238, 236},
            []uint8{76, 154, 236},
            []uint8{120, 124, 236},
            []uint8{176, 98, 236},
            []uint8{228, 84, 236},
            []uint8{236, 88, 180},
            []uint8{236, 106, 100},
            []uint8{212, 136, 32},
            []uint8{160, 170, 0},
            []uint8{116, 196, 0},
            []uint8{76, 208, 32},
            []uint8{56, 204, 108},
            []uint8{56, 180, 204},
            []uint8{60, 60, 60},
            []uint8{236, 238, 236},
            []uint8{168, 204, 236},
            []uint8{188, 188, 236},
            []uint8{212, 178, 236},
            []uint8{236, 174, 236},
            []uint8{236, 174, 212},
            []uint8{236, 180, 176},
            []uint8{228, 196, 144},
            []uint8{204, 210, 120},
            []uint8{180, 222, 120},
            []uint8{168, 226, 144},
            []uint8{152, 226, 180},
            []uint8{160, 214, 228},
            []uint8{160, 162, 160},
        }

        tile_x := 0
        tile_y := 0
        for address := uint16(0); address < 0x3c0; address++ {
            // log.Printf("Render nametable 0x%x: 0x%x %v, %v", nametableBase + address, ppu.VideoMemory[nametableBase + address], x, y)
            tileIndex := ppu.VideoMemory[nametableBase + address]
            tileAddress := patternTable + uint16(tileIndex) * 16
            leftBytes := ppu.VideoMemory[tileAddress:tileAddress+8]
            rightBytes := ppu.VideoMemory[tileAddress+8:tileAddress+16]

            _ = leftBytes
            _ = rightBytes
            _ = palette

            /* pattern attribute x = tile_x / 4
             * pattern_attribute y = tile_y / 4
             */

            pattern_attribute_address := attributeTableBase + uint16(tile_x / 4 + (tile_y / 4) * (32/4))
            pattern_attribute_value := ppu.VideoMemory[pattern_attribute_address]
            pattern_attribute_top_left := pattern_attribute_value & 0x3
            pattern_attribute_top_right := (pattern_attribute_value >> 2) & 0x3
            pattern_attribute_bottom_left := (pattern_attribute_value >> 4) & 0x3
            pattern_attribute_bottom_right := (pattern_attribute_value >> 6) & 0x3

            /* x to x+4
             * top left = x:x+1, y:y+1
             * top right = x+2:x+3, y:y+1
             * bottom left = x:x+1, y+2:y+3
             * bottom right = x+2:x+3, y+2:y+3
             */

            pattern_x := tile_x & 0x3
            pattern_y := tile_y & 0x3

            var color_set byte
            if pattern_x < 2 && pattern_y < 2 {
                color_set = pattern_attribute_top_left
            } else if pattern_x < 2 && pattern_y >= 2 {
                color_set = pattern_attribute_bottom_left
            } else if pattern_x >= 2 && pattern_y < 2 {
                color_set = pattern_attribute_top_right
            } else {
                color_set = pattern_attribute_bottom_right
            }

            // log.Printf("Tile %v, %v = color set %v", tile_x, tile_y, color_set)

            /* the actual palette to use */
            palette_base := 0x3f00 + uint16(color_set) * 4

            lastColor := byte(0xff)
            for y := 0; y < 8; y++ {
                for x := 0; x < 8; x++ {
                    low := (leftBytes[y] >> (7-x)) & 0x1
                    high := ((rightBytes[y] >> (7-x)) & 0x1) << 1
                    colorIndex := high | low

                    _ = colorIndex

                    if colorIndex != lastColor {
                        /* Calling SetDrawColor seems to be quite slow, so we cache the draw color */
                        palette_color := ppu.VideoMemory[palette_base + uint16(colorIndex)]
                        // log.Printf("Pixel %v, %v = %v", tile_x*8 + x, tile_y*8 + y, palette_color)
                        renderer.SetDrawColorArray(palette[palette_color]...)
                        // renderer.SetDrawColor(palette[colorIndex][0], palette[colorIndex][1], palette[colorIndex][2], 255)
                        lastColor = palette_color
                    }

                    renderer.DrawPoint(int32(tile_x * 8 + x), int32(tile_y * 8 + y))
                }
            }

            // log.Printf("Render nametable 0x%x: 0x%x %v, %v = %v %v", nametableBase + address, ppu.VideoMemory[nametableBase + address], tile_x, tile_y, leftBytes, rightBytes)

            tile_x += 1
            if tile_x >= 32 {
                tile_y += 1
                tile_x = 0
            }
        }

        // renderer.Flush()
        // renderer.Present()
    }
}

/* give a number of PPU cycles to process
 * returns whether nmi is set or not
 */
func (ppu *PPUState) Run(cycles uint64, renderer *sdl.Renderer) (bool, bool) {
    /* http://wiki.nesdev.com/w/index.php/PPU_rendering */
    ppu.Cycle += cycles
    cyclesPerScanline := uint64(341)
    nmi := false
    didDraw := false
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

            start := time.Now()
            ppu.Render(renderer)
            log.Printf("Took %v to render", time.Now().Sub(start))
            didDraw = true
        }

        ppu.Cycle -= cyclesPerScanline
    }

    return nmi, didDraw
}
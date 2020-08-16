package lib

import (
    "log"
    "fmt"
    _ "time"
)

type PPUState struct {
    Flags byte
    Mask byte
    /* http://wiki.nesdev.com/w/index.php/PPU_registers#PPUSTATUS */
    Status byte

    /* counts in the y direction during rendering, from 0-262 */
    Scanline int
    /* counts in the x direction during rendering, from 0-340 */
    ScanlineCycle int
    TemporaryVideoAddress uint16 /* the t register */
    VideoAddress uint16 /* the v register */
    WriteState byte /* for writing to the video address or the t register */

    HorizontalNametableMirror bool
    VerticalNametableMirror bool

    /* for scrolling */
    FineX byte

    /* maps an nes integer 0-256 to an RGB value */
    Palette [][]uint8

    /* current sprites that will render on this scanline. a bit of a hack */
    CurrentSprites []Sprite

    VideoMemory []byte

    /* the 2kb SRAM stored on the NES board.
     * use nametable mirroring to map addresses to these ranges
     * Nametable[0:0x400] should appear at 0x2000 and 0x2800 in vertical mirroring
     * and 0x2000 and 0x2400 in horizontal mirroring.
     * Nametable[0x400:0x800] should appear at 0x2400 and 0x2c00 in vertical mirroring
     * and 0x2800 and 0x2c00 in horizontal mirroring.
     *
     * http://wiki.nesdev.com/w/index.php/PPU_nametables
     */
    NametableMemory []byte

    /* sprite memory */
    OAM []byte
    OAMAddress int

    /* makes the ppu print stuff via log.Printf if set to a value > 0 */
    Debug uint8

    InternalVideoBuffer byte

    /* how many times to shift out of the BackgroundPixels before needing to load a new tile */
    Shifts byte

    /* each pixel is 4 bits, each tile is 8 pixels
     * so first 32 bits is the first tile
     * and second 32 bits is the next tile.
     * As each pixel is rendered, the entire
     * thing is shifted 4 pixels to the right.
     * After 8 pixels are rendered, a new tile
     * is read from memory, its pixels computed, and
     * loaded into the upper 32 its of BackgroundPixels
     */
    BackgroundPixels uint64
    RawBackgroundPixels uint32

    /* not sure if this is needed */
    HasSetSprite0 bool
}

func MakePPU() PPUState {
    return PPUState{
        VideoMemory: make([]byte, 64 * 1024), // FIXME: video memory is not this large..
        NametableMemory: make([]byte, 2 * 1024), // 2kb SRAM
        OAM: make([]byte, 256),
        Scanline: 0,
        Palette: get2c02Palette(),
    }
}

func (ppu *PPUState) ToggleDebug() {
    ppu.Debug = 1 - ppu.Debug
}

func (ppu *PPUState) SetHorizontalMirror(value bool){
    if ppu.Debug > 0 {
        log.Printf("Set horizontal mirror to %v", value)
    }
    ppu.HorizontalNametableMirror = value
}

func (ppu *PPUState) SetVerticalMirror(value bool){
    if ppu.Debug > 0 {
        log.Printf("Set vertical mirror to %v", value)
    }
    ppu.VerticalNametableMirror = value
}

func (ppu *PPUState) SetOAMAddress(value byte){
    ppu.OAMAddress = int(value)
}

func (ppu *PPUState) WriteOAM(value byte){
    if ppu.OAMAddress < len(ppu.OAM) {
        ppu.OAM[ppu.OAMAddress] = value
        ppu.OAMAddress += 1
    }
}

func (ppu *PPUState) ReadOAM(address byte) byte {
    return ppu.OAM[address]
}

func (ppu *PPUState) CopyOAM(data []byte){

    maxOAM := len(ppu.OAM)
    for i := 0; i < len(data); i++ {
        address := byte(i + ppu.OAMAddress)
        if int(address) >= maxOAM {
            break
        }
        ppu.OAM[address] = data[i]
    }
}

func (ppu *PPUState) WriteScroll(value byte){
    /* t: yyy NN YYYYY XXXXX
     *
     * $2005 first write (w is 0)
     * t: ....... ...HGFED = d: HGFED...
     * x:              CBA = d: .....CBA
     * w:                  = 1
     *
     * $2005 second write (w is 1)
     * t: CBA..HG FED..... = d: HGFEDCBA
     * w:                  = 0
     */

    if ppu.Debug > 0 {
        log.Printf("PPU: Write scroll 0x%x write state %v scanline %v cycle %v", value, ppu.WriteState, ppu.Scanline, ppu.ScanlineCycle)
    }
    switch ppu.WriteState {
        case 0:
            coarseX := value >> 3
            fineY, nametable, coarseY, _ := deconstructVideoAddress(ppu.TemporaryVideoAddress)
            // ppu.TemporaryVideoAddress = (ppu.TemporaryVideoAddress & (^coarseXBits)) | uint16(coarseX)
            ppu.TemporaryVideoAddress = ppu.ConstructVideoAddress(fineY, nametable, coarseY, coarseX)
            /* lowest 3 bits of the value are the fine x */
            ppu.FineX = value & 0x7
            ppu.WriteState = 1
        case 1:
            // ppu.FineY = value & 0x7
            fineY := value & 0x7
            /* 0b1 0000 0000 0000 */
            coarseY := value >> 3
            /*
            ppu.TemporaryVideoAddress = ppu.TemporaryVideoAddress | (uint16(fineY) << 12)
            ppu.TemporaryVideoAddress = ppu.TemporaryVideoAddress | (uint16(coarseY) << 5)
            */
            _, nametable, _, coarseX := deconstructVideoAddress(ppu.TemporaryVideoAddress)
            ppu.TemporaryVideoAddress = ppu.ConstructVideoAddress(fineY, nametable, coarseY, coarseX)
            ppu.WriteState = 0

            if ppu.Debug > 0 {
                fineY, nametable, coarseY, coarseX := deconstructVideoAddress(ppu.TemporaryVideoAddress)
                log.Printf("PPU: scroll set finey %v nametable %v coarseY %v coarseX %v t 0x%x", fineY, nametable, coarseY, coarseX, ppu.TemporaryVideoAddress)
            }
    }
}

func (ppu *PPUState) SetControllerFlags(value byte) {
    if ppu.Debug > 0 {
        log.Printf("PPU set controller flags to 0x%x. VBlank is %v. Scanline %v cycle %v", value, ppu.IsVerticalBlankFlagSet(), ppu.Scanline, ppu.ScanlineCycle)
    }

    nametable := value & 3

    nametableBits := uint16(0b1100_00000000)
    ppu.TemporaryVideoAddress = (ppu.TemporaryVideoAddress & (^nametableBits)) | (uint16(nametable) << 10)

    if ppu.Debug > 0 {
        log.Printf("PPU temporary video address 0x%x. Nametable %v", ppu.TemporaryVideoAddress, nametable)
    }

    ppu.Flags = value
}

func (ppu *PPUState) GetNameTableIndex() byte {
    /* upper left is 0
     * upper right is 1
     * lower left is 2
     * lower right is 3
     */
    return ppu.Flags & 0x3
}

func (ppu *PPUState) GetNameTableBaseAddress(index byte) uint16 {

    switch index {
        case 0: return 0x2000
        case 1: return 0x2400
        case 2: return 0x2800
        case 3: return 0x2c00
    }

    return 0x2000
}

func (ppu *PPUState) GetSpritePatternTableBase() uint16 {
    index := (ppu.Flags >> 3) & 0x1
    switch index {
        case 1: return 0x1000
        case 0: return 0x0000
    }

    return 0x0000
}

func (ppu *PPUState) GetBackgroundPatternTableBase() uint16 {
    background_table_index := (ppu.Flags >> 4) & 0x1 == 0x1

    switch background_table_index {
        case true: return 0x1000
        case false: return 0x0000
    }

    return 0x0000
}

type SpriteSize int
const (
    SpriteSize8x16 = iota
    SpriteSize8x8
)

func (ppu *PPUState) GetSpriteSize() SpriteSize {
    sprite_size_index := (ppu.Flags >> 5) & 0x1 == 0x1
    if sprite_size_index {
        return SpriteSize8x16
    }

    return SpriteSize8x8
}

func (ppu *PPUState) ControlString() string {
    vram_increment_index := (ppu.Flags >> 2) & 0x1 == 0x1
    master_slave_index := (ppu.Flags >> 6) & 0x1 == 0x1
    nmi := (ppu.Flags >> 7) & 0x1 == 0x1

    base_nametable_address := ppu.GetNameTableBaseAddress(ppu.GetNameTableIndex())

    var vram_increment int
    switch vram_increment_index {
        case true: vram_increment = 32
        case false: vram_increment = 1
    }

    sprite_table := ppu.GetSpritePatternTableBase()
    background_table := ppu.GetBackgroundPatternTableBase()

    var sprite_size string
    switch ppu.GetSpriteSize() {
        case SpriteSize8x16: sprite_size = "8x16"
        case SpriteSize8x8: sprite_size = "8x8"
    }

    var master_slave string
    switch master_slave_index {
        case true: master_slave = "output on ext"
        case false: master_slave = "read from ext"
    }

    return fmt.Sprintf("Nametable=0x%x Vram-increment=%v Sprite-table=0x%x Background-table=0x%x Sprite-size=%v Master/slave=%v NMI=%v", base_nametable_address, vram_increment, sprite_table, background_table, sprite_size, master_slave, nmi)
}

func (ppu *PPUState) CopyCharacterRom(base uint32, data []byte) {
    for i := uint32(0); i < uint32(len(data)); i++ {
        ppu.VideoMemory[base + i] = data[i]
    }
}

func (ppu *PPUState) IsBackgroundEnabled() bool {
    /* FIXME: also include background_leftmost_8? */
    background := (ppu.Mask >> 3) & 0x1 == 0x1
    return background
}

func (ppu *PPUState) IsSpriteEnabled() bool {
    /* FIXME: what about sprite_leftmost_8 */
    sprite := (ppu.Mask >> 4) & 0x1 == 0x1
    return sprite
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
    if ppu.Debug > 0 {
        log.Printf("PPU: Write video address 0x%x write state %v scanline %v cycle %v", value, ppu.WriteState, ppu.Scanline, ppu.ScanlineCycle)
    }
    switch ppu.WriteState {
        /* write high byte */
        case 0:
            /* highest available page is 3f. after that it is mirrored down */
            ppu.TemporaryVideoAddress = uint16(value & 0x3f) << 8 | (uint16(ppu.TemporaryVideoAddress) & 0xff)
            ppu.WriteState = 1
        case 1:
            /* keep upper 8 bits of the temporary address and replace the lower 8 bits with
             * the value
             */
            ppu.TemporaryVideoAddress = (ppu.TemporaryVideoAddress & (uint16(0xff) << 8)) | uint16(value)
            ppu.WriteState = 0
            ppu.VideoAddress = ppu.TemporaryVideoAddress

            if ppu.Debug > 0 {
                log.Printf("PPU: Video address is now 0x%x", ppu.VideoAddress)
            }
    }
}

func (ppu *PPUState) GetVRamIncrement() uint16 {
    switch (ppu.Flags >> 2) & 0x1 {
        case 0: return 1
        case 1: return 32
    }

    return 1
}

func (ppu *PPUState) WriteVideoMemory(value byte){
    actualAddress := ppu.VideoAddress

    /* Mirror writes to the universal background color */
    if actualAddress == 0x3f04 || actualAddress == 0x3f10 ||
       actualAddress == 0x3f14 || actualAddress == 0x3f18 ||
       actualAddress == 0x3f1c {
        actualAddress = 0x3f00
    }

    /*
    if (ppu.VideoAddress >= 0x2400 && ppu.VideoAddress <= 0x2800) || (ppu.VideoAddress >= 0x2c00 && ppu.VideoAddress < 0x3000) {
        log.Printf("Writing to mirrored video memory at 0x%x", ppu.VideoAddress)
    }
    */

    // log.Printf("PPU Writing 0x%x at video memory 0x%x", value, ppu.VideoAddress)

    if ppu.Debug > 0 {
        log.Printf("PPU: Writing 0x%x to video memory at 0x%x actual 0x%x at scanline %v and cycle %v\n", value, ppu.VideoAddress, actualAddress, ppu.Scanline, ppu.ScanlineCycle)
    }

    if actualAddress >= 0x2000 && actualAddress < 0x3000 {
        ppu.StoreNametableMemory(actualAddress, value)
    } else {
        ppu.VideoMemory[actualAddress] = value
    }
    ppu.VideoAddress += ppu.GetVRamIncrement()
}

func (ppu *PPUState) ReadVideoMemory() byte {
    if int(ppu.VideoAddress) >= len(ppu.VideoMemory) {
        log.Printf("Warning: attemping to read more than available video memory 0x%x at 0x%x", len(ppu.VideoMemory), ppu.VideoAddress)
        return 0
    }

    var value byte

    if ppu.VideoAddress >= 0x2000 && ppu.VideoAddress < 0x3000 {
        value = ppu.LoadNametableMemory(ppu.VideoAddress)
    } else {
        value = ppu.VideoMemory[ppu.VideoAddress]
    }

    if ppu.Debug > 0 {
        log.Printf("PPU: read from video memory 0x%x = 0x%x", ppu.VideoAddress, value)
    }

    /* for reading from palette memory we don't do a dummy read
     * Reading palette data from $3F00-$3FFF works differently. The palette data is placed immediately on the data bus, and hence no dummy read is required. Reading the palettes still updates the internal buffer though, but the data placed in it is the mirrored nametable data that would appear "underneath" the palette. (Checking the PPU memory map should make this clearer.)
     */
    if ppu.VideoAddress >= 0x3f00 && ppu.VideoAddress <= 0x3fff {
        ppu.InternalVideoBuffer = value
        ppu.VideoAddress += ppu.GetVRamIncrement()
        return value
    }

    old := ppu.InternalVideoBuffer
    ppu.InternalVideoBuffer = value
    ppu.VideoAddress += ppu.GetVRamIncrement()
    return old
}

func (ppu *PPUState) GetSpriteZeroHit() bool {
    bit := byte(1<<6)
    return ppu.Status & bit == bit
}

func (ppu *PPUState) SetSpriteZeroHit(on bool){
    bit := byte(1<<6)
    if on {
        ppu.Status = ppu.Status | bit
    } else {
        ppu.Status = ppu.Status & (^bit)
    }
}

func (ppu *PPUState) SetVerticalBlankFlag(on bool){
    bit := byte(1<<7)
    if on {
        ppu.Status = ppu.Status | bit
    } else {
        ppu.Status = ppu.Status & (^bit)
    }
}

func (ppu *PPUState) IsVerticalBlankFlagSet() bool {
    return ppu.Status & (1<<7) == 1<<7
}

func (ppu *PPUState) ReadStatus() byte {
    if ppu.Debug > 0 {
        log.Printf("Read PPU status")
    }
    out := ppu.Status
    ppu.SetVerticalBlankFlag(false)
    ppu.WriteState = 0
    return out
}

type Sprite struct {
    tile byte
    x, y byte
    flip_horizontal bool
    flip_vertical bool
    palette byte
    priority byte
    sprite0 bool
}

func (ppu *PPUState) GetSprites() []Sprite {
    var out []Sprite
    position := 0
    spriteCount := 0
    for position < len(ppu.OAM) {
        y := ppu.OAM[position]
        position += 1
        tile := ppu.OAM[position]
        position += 1

        data := ppu.OAM[position]
        position += 1

        x := ppu.OAM[position]
        position += 1

        palette := data & 0x3
        priority := (data >> 5) & 0x1
        flip_horizontal := (data >> 6) & 0x1 == 0x1
        flip_vertical := (data >> 7) & 0x1 == 0x1

        out = append(out, Sprite{
            tile: tile,
            x: x,
            y: y + 1, // sprites are offset by 1 pixel
            flip_horizontal: flip_horizontal,
            flip_vertical: flip_vertical,
            palette: palette,
            priority: priority,
            sprite0: spriteCount == 0,
        })

        spriteCount += 1
    }

    return out
}

func get2c02Palette() [][]uint8 {
    /* blargg's 2c02 palette
     *   http://wiki.nesdev.com/w/index.php/PPU_palettes
     */
    return [][]uint8{
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

        []uint8{0, 0, 0},    // 0e
        []uint8{0, 0, 0},    // 0f

        []uint8{152, 150, 152}, // 10
        []uint8{8, 76, 196},    // 11
        []uint8{48, 50, 236},   // 12
        []uint8{92, 30, 228},   // 13
        []uint8{136, 20, 176},  // 14
        []uint8{160, 20, 100},  // 15
        []uint8{152, 34, 32},   // 16
        []uint8{120, 60, 0},    // 17
        []uint8{84, 90, 0},     // 18
        []uint8{40, 114, 0},    // 19
        []uint8{8, 124, 0},     // 1a
        []uint8{0, 118, 40},    // 1b
        []uint8{0, 102, 120},   // 1c
        []uint8{0, 0, 0},       // 1d
        []uint8{0, 0, 0},       // 1e
        []uint8{0, 0, 0},       // 1f

        []uint8{236, 238, 236}, // 20
        []uint8{76, 154, 236},  // 21
        []uint8{120, 124, 236}, // 22
        []uint8{176, 98, 236},  // 23
        []uint8{228, 84, 236},  // 24
        []uint8{236, 88, 180},  // 25
        []uint8{236, 106, 100}, // 26
        []uint8{212, 136, 32},  // 27
        []uint8{160, 170, 0},   // 28
        []uint8{116, 196, 0},   // 29
        []uint8{76, 208, 32},   // 2a
        []uint8{56, 204, 108},  // 2b
        []uint8{56, 180, 204},  // 2c
        []uint8{60, 60, 60},    // 2d
        []uint8{0, 0, 0},       // 2e
        []uint8{0, 0, 0},       // 2f

        []uint8{236, 238, 236}, // 30
        []uint8{168, 204, 236}, // 31
        []uint8{188, 188, 236}, // 32
        []uint8{212, 178, 236}, // 33
        []uint8{236, 174, 236}, // 34
        []uint8{236, 174, 212}, // 35
        []uint8{236, 180, 176}, // 36
        []uint8{228, 196, 144}, // 37
        []uint8{204, 210, 120}, // 38
        []uint8{180, 222, 120}, // 39
        []uint8{168, 226, 144}, // 3a
        []uint8{152, 226, 180}, // 3b
        []uint8{160, 214, 228}, // 3c
        []uint8{160, 162, 160}, // 3d
        []uint8{0, 0, 0},       // 3e
        []uint8{0, 0, 0},       // 3f
    }
}

type VirtualScreen struct {
    Width int
    Height int
    Buffer []uint32
}

func (screen *VirtualScreen) Area() int {
    return screen.Width * screen.Height
}

func (screen *VirtualScreen) GetRGBA(x int, y int) (uint8, uint8, uint8, uint8) {
    if x < 0 || int(x) >= screen.Width || y < 0 || int(y) >= screen.Height {
        return 0, 0, 0, 0
    }

    address := (y * screen.Width) + x
    pixel := screen.Buffer[address]

    r := uint8((pixel >> 24) & 0xff)
    g := uint8((pixel >> 16) & 0xff)
    b := uint8((pixel >> 8) & 0xff)
    a := uint8((pixel >> 0) & 0xff)

    return r, g, b, a
}

func (screen *VirtualScreen) DrawPoint(x int32, y int32, rgb []uint8){
    if x < 0 || int(x) >= screen.Width || y < 0 || int(y) >= screen.Height {
        return
    }

    address := (y * int32(screen.Width)) + x
    r := uint32(rgb[0])
    g := uint32(rgb[1])
    b := uint32(rgb[2])
    a := uint32(255)
    /* putpixel with RGBA 8888 */
    screen.Buffer[address] = (r << 24) | (g << 16) | (b << 8) | a
}

func (screen *VirtualScreen) Clear() {
    max := len(screen.Buffer)
    /* for debugging screen glitches */

    // color := (uint32(255) << 24) | (uint32(20) << 16) | (uint32(20) << 8) | uint32(255)
    var color uint32 = 0
    for i := 0; i < max; i++ {
        screen.Buffer[i] = color
    }
}

func (screen *VirtualScreen) CopyFrom(copyFrom *VirtualScreen){
    copy(screen.Buffer, copyFrom.Buffer)
}

func (screen *VirtualScreen) Copy() VirtualScreen {
    out := MakeVirtualScreen(screen.Width, screen.Height)
    copy(out.Buffer, screen.Buffer)
    return out
}

func MakeVirtualScreen(width int, height int) VirtualScreen {
    return VirtualScreen{
        Width: width,
        Height: height,
        /* 300kb of memory */
        Buffer: make([]uint32, width * height),
    }
}

func color_set_value(value uint8) []uint8 {
    switch value {
    case 0: return []uint8{128, 128, 128}
    case 1: return []uint8{255, 0, 0}
    case 2: return []uint8{0, 255, 0}
    case 3: return []uint8{0, 0, 255}
    }

    return nil
}

func hue2rgb(p float32, q float32, t float32) float32 {
    if t < 0 {
        t += 1
    }
    if t > 1 {
        t -= 1
    }
    if t < 1.0/6.0 {
        return p + (q - p) * 6 * t
    }
    if t < 1.0/2.0 {
        return q
    }
    if t < 2.0/3.0 {
        return p + (q - p) * (2.0/3.0 - t) * 6
    }
    return p;
}

func hslToRgb(h float32, s float32, l float32) (byte, byte, byte){
    var r float32
    var g float32
    var b float32

    if s == 0 {
        r = l
        g = l
        b = l
    } else {
        var q float32
        if l < 0.5 {
            q = l * (1 + s)
        } else {
            q = l + s - l * s
        }
        p := 2 * l - q
        r = hue2rgb(p, q, h + 1.0/3.0)
        g = hue2rgb(p, q, h)
        b = hue2rgb(p, q, h - 1.0/3.0)
    }

    return byte(r * 255), byte(g * 255), byte(b * 255)
}

func (ppu *PPUState) getBackgroundPixel() []uint8 {
    if !ppu.IsBackgroundEnabled() {
        return nil
    }

    /* The pixel value from the pattern table, a value from 0-3
     * where 0 means transparent
     */
    if (ppu.RawBackgroundPixels >> (ppu.FineX * 2)) & 0b11 == 0 {
        // log.Printf("Background pixel at %v %v", ppu.Scanline, ppu.ScanlineCycle)
        return nil
    }

    /* Each pixel is 4 bits, so shift right by fineX*4 pixels */
    colorIndex := uint16((ppu.BackgroundPixels >> (ppu.FineX * 4)) & 0b1111)
    paletteBase := uint16(0x3f00)

    paletteIndex := ppu.VideoMemory[paletteBase + colorIndex] & 0x3f

    /*
    _, _, coarseY, coarseX := ppu.DeconstructVideoAddress()
    if (coarseX >= 5 || coarseX <= 8) && coarseY == 3 {
        log.Printf("Pixel at scanline %v cycle %v color %v palette index %v", ppu.Scanline, ppu.ScanlineCycle, colorIndex, paletteIndex)
    }
    */

    return ppu.Palette[paletteIndex]
}

func (ppu *PPUState) getSpritePixel(x int, y int, sprites []Sprite) ([]uint8, byte, bool) {

    if !ppu.IsSpriteEnabled() {
        return nil, 0, false
    }

    patternTable := ppu.GetSpritePatternTableBase()
    spriteSize := ppu.GetSpriteSize()

        // offset := 1
    switch spriteSize {
    case SpriteSize8x8:
        maxSprites := len(sprites)
        for spriteIndex := 0; spriteIndex < maxSprites; spriteIndex += 1 {
            sprite := &sprites[spriteIndex]
            tileIndex := sprite.tile
            if x >= int(sprite.x) && x < int(sprite.x) + 8 && y >= int(sprite.y) && y < int(sprite.y) + 8 {
                tileAddress := patternTable + uint16(tileIndex) * 16
                leftBytes := ppu.VideoMemory[tileAddress:tileAddress+8]
                rightBytes := ppu.VideoMemory[tileAddress+8:tileAddress+16]
                palette_base := 0x3f10 + uint16(sprite.palette) * 4

                // for y := 0; y < 8; y++ {
                    // for x := 0; x < 8; x++ {
                use_y := y - int(sprite.y)
                use_x := x - int(sprite.x)

                if sprite.flip_vertical {
                    use_y = 7 - use_y
                }

                if sprite.flip_horizontal {
                    use_x = 7 - use_x
                }

                low := (leftBytes[use_y] >> (7-use_x)) & 0x1
                high := ((rightBytes[use_y] >> (7-use_x)) & 0x1) << 1

                /*
                var low byte
                var high byte
                if sprite.flip_horizontal {
                    low = (leftBytes[use_y] >> (use_x)) & 0x1
                    high = ((rightBytes[use_y] >> (use_x)) & 0x1) << 1
                } else {
                    low = (leftBytes[use_y] >> (7-use_x)) & 0x1
                    high = ((rightBytes[use_y] >> (7-use_x)) & 0x1) << 1
                }
                */

                        colorIndex := high | low

                        /* Skip non-opaque pixels */
                        if colorIndex == 0 {
                            continue
                        }

                        palette_color := ppu.VideoMemory[palette_base + uint16(colorIndex)]

                        /* failsafe in case we are reading bogus memory somehow */
                        if int(palette_color) >= len(ppu.Palette) {
                            return nil, 0, false
                        }

                        return ppu.Palette[palette_color], sprite.priority, sprite.sprite0

                        /*
                        var final_x int
                        if flipHorizontal {
                            final_x = spriteX + (7-x)
                        } else {
                            final_x = spriteX + x
                        }
                        var final_y int
                        if flipVertical {
                            final_y = spriteY + (7-y)
                        } else {
                            final_y = spriteY + y
                        }

                        screen.DrawPoint(int32(final_x), int32(final_y), palette[palette_color])
                        */
                    // }
                // }
            }
        }
    case SpriteSize8x16:
        maxSprites := len(sprites)
        for spriteIndex := 0; spriteIndex < maxSprites; spriteIndex += 1 {
            sprite := &sprites[spriteIndex]
            tileIndex := sprite.tile
            if x >= int(sprite.x) && x < int(sprite.x) + 8 && y >= int(sprite.y) && y < int(sprite.y) + 16 {
                tileAddress := (uint16(tileIndex & 0x1) << 12) | (uint16(tileIndex >> 1) * 32)
                y_base := sprite.y
                if sprite.flip_vertical {
                    if y < int(sprite.y) + 8 {
                        tileAddress += 16
                    } else {
                        y_base += 8
                    }
                } else {
                    if y >= int(sprite.y) + 8 {
                        y_base += 8
                        tileAddress += 16
                    }
                }
                leftBytes := ppu.VideoMemory[tileAddress:tileAddress+8]
                rightBytes := ppu.VideoMemory[tileAddress+8:tileAddress+16]
                palette_base := 0x3f10 + uint16(sprite.palette) * 4

                use_y := y - int(y_base)
                use_x := x - int(sprite.x)

                if sprite.flip_vertical {
                    use_y = 7 - use_y
                }

                if sprite.flip_horizontal {
                    use_x = 7 - use_x
                }

                low := (leftBytes[use_y] >> (7-use_x)) & 0x1
                high := ((rightBytes[use_y] >> (7-use_x)) & 0x1) << 1

                /*
                var low byte
                var high byte
                if sprite.flip_horizontal {
                    low = (leftBytes[use_y] >> (use_x)) & 0x1
                    high = ((rightBytes[use_y] >> (use_x)) & 0x1) << 1
                } else {
                    low = (leftBytes[use_y] >> (7-use_x)) & 0x1
                    high = ((rightBytes[use_y] >> (7-use_x)) & 0x1) << 1
                }
                */

                        colorIndex := high | low

                        /* Skip non-opaque pixels */
                        if colorIndex == 0 {
                            continue
                        }

                        palette_color := ppu.VideoMemory[palette_base + uint16(colorIndex)]

                        /* failsafe in case we are reading bogus memory somehow */
                        if int(palette_color) >= len(ppu.Palette) {
                            return nil, 0, false
                        }

                        return ppu.Palette[palette_color], sprite.priority, sprite.sprite0

            }
        }

            /* even tiles come from bank 0x0000, and odd tiles come from 0x1000 */
            /*
            tileAddress := (uint16(tileIndex & 0x1) << 12) | (uint16(tileIndex >> 1) * 32)

            topY := int(sprite.y)
            bottomY := int(sprite.y + 8)

            if sprite.flip_vertical {
                topY = int(sprite.y + 8)
                bottomY = int(sprite.y)
            }
            */

            /* top tile */
            // ppu.renderSpriteTile(tileAddress, sprite.palette, palette, sprite.flip_horizontal, sprite.flip_vertical, int(sprite.x), topY + offset, screen)
            /* bottom tile */
            // ppu.renderSpriteTile(tileAddress+16, sprite.palette, palette, sprite.flip_horizontal, sprite.flip_vertical, int(sprite.x), bottomY + offset, screen)
    }

    return nil, 0, false
}

/* Returns true for a sprite 0 hit */
func (ppu *PPUState) RenderPixel(scanLine int, cycle int, sprites []Sprite, screen *VirtualScreen) bool {
    background := ppu.getBackgroundPixel()
    sprite, spritePriority, sprite0 := ppu.getSpritePixel(cycle, scanLine, sprites)

    if sprite != nil && background != nil {
        if spritePriority == 0 {
            screen.DrawPoint(int32(cycle), int32(scanLine), sprite)
        } else {
            screen.DrawPoint(int32(cycle), int32(scanLine), background)
        }

        return sprite0
    } else if sprite != nil {
        screen.DrawPoint(int32(cycle), int32(scanLine), sprite)
    } else if background != nil {
        screen.DrawPoint(int32(cycle), int32(scanLine), background)
    } else {
        background := ppu.VideoMemory[0x3f00]
        if int(background) >= len(ppu.Palette){
            screen.DrawPoint(int32(cycle), int32(scanLine), ppu.Palette[0])
        } else {
            screen.DrawPoint(int32(cycle), int32(scanLine), ppu.Palette[background])
        }
    }

    return false
}

/* draw lines to cover a grid of 8x8 pixel, which maps directly to
 * background tiles. 4x4 tiles (32x32 pixels) maps to a byte in the
 * attribute table.
 */
func drawOverlay(screen VirtualScreen, size int, color []uint8){
    for y := 0; y < 240; y += size {
        for x := 0; x < 256; x++ {
            screen.DrawPoint(int32(x), int32(y), color)
        }
    }

    for x := 0; x < 256; x += size {
        for y := 0; y < 240; y ++ {
            screen.DrawPoint(int32(x), int32(y), color)
        }
    }
}

/* Bump the y scroll, wrapping the nametable if necessary */
func (ppu *PPUState) IncrementVerticalPosition(){
    fineY, nametable, coarseY, coarseX := ppu.DeconstructVideoAddress()

    if fineY < 7 {
        fineY += 1
    } else {
        fineY = 0
        if coarseY == 29 {
            coarseY = 0

            x_nametable := nametable & 0b1
            y_nametable := nametable >> 0b1
            y_nametable = (y_nametable + 1) & 0b1
            nametable = (y_nametable << 1) | x_nametable
        } else if coarseY == 31 {
            coarseY = 0
        } else {
            coarseY += 1
        }
    }

    if ppu.Debug > 0 {
        log.Printf("PPU: increment vertical. finey %v nametable %v coarseY %v coarseX %v = 0x%x", fineY, nametable, coarseY, coarseX, ppu.ConstructVideoAddress(fineY, nametable, coarseY, coarseX))
    }

    ppu.VideoAddress = ppu.ConstructVideoAddress(fineY, nametable, coarseY, coarseX)
}

func (ppu *PPUState) ResetHorizontalPosition(){
    /* v: ....F.. ...EDCBA = t: ....F.. ...EDCBA
     * keep the X nametable bit (F) and coarse X (EDCBA)
     */
    mask := uint16(0b100_00011111)
    ppu.VideoAddress = (ppu.VideoAddress & (^mask)) | (ppu.TemporaryVideoAddress & mask)

    if ppu.Debug > 0 {
        fineY, nametable, coarseY, coarseX := deconstructVideoAddress(ppu.VideoAddress)
        log.Printf("PPU: reset horizontal fineY %v nametable %v coarseY %v coarseX %v", fineY, nametable, coarseY, coarseX)
    }
}

func deconstructVideoAddress(address uint16) (byte, byte, byte, byte){
    fineY := (address >> 12) & uint16(0b111)
    nametable := (address >> 10) & uint16(0b11)
    coarseY := (address >> 5) & uint16(0b11111)
    coarseX := address & uint16(0b11111)

    return byte(fineY), byte(nametable), byte(coarseY), byte(coarseX)
}

/* Returns fineY, nametable, coarseY, coarseX */
func (ppu *PPUState) DeconstructVideoAddress() (byte, byte, byte, byte) {
    return deconstructVideoAddress(ppu.VideoAddress)
}

func (ppu *PPUState) ConstructVideoAddress(fineY byte, nametable byte, coarseY byte, coarseX byte) uint16 {
    return (uint16(fineY) << 12) | (uint16(nametable) << 10) | (uint16(coarseY) << 5) | uint16(coarseX)
}

func (ppu *PPUState) IncrementHorizontalAddress(){
    fineY, nametable, coarseY, coarseX := ppu.DeconstructVideoAddress()

    if coarseX == 31 {
        coarseX = 0
        x_nametable := nametable & 0b1
        y_nametable := nametable >> 0b1
        x_nametable = (x_nametable + 1) & 0b1
        nametable = (y_nametable << 1) | x_nametable
    } else {
        coarseX += 1
    }

    ppu.VideoAddress = ppu.ConstructVideoAddress(fineY, nametable, coarseY, coarseX)
}

func isNametable1(address uint16) bool {
    return address >= 0x2000 && address < 0x2400
}

func isNametable2(address uint16) bool {
    return address >= 0x2400 && address < 0x2800
}

func isNametable3(address uint16) bool {
    return address >= 0x2800 && address < 0x2c00
}

func isNametable4(address uint16) bool {
    return address >= 0x2c00 && address < 0x3000
}

func (ppu *PPUState) nametableMirrorAddress(address uint16) uint16 {
    /*
    * Nametable1 should appear at 0x2000 and 0x2800 in vertical mirroring
     * and 0x2000 and 0x2400 in horizontal mirroring.
     * Nametable2 should appear at 0x2400 and 0x2c00 in vertical mirroring
     * and 0x2800 and 0x2c00 in horizontal mirroring.
     */

    if ppu.HorizontalNametableMirror {
        if isNametable1(address){
            return address & 0xfff
        }

        if isNametable2(address){
            return (address - 0x400) & 0xfff
        }

        if isNametable3(address){
            return (address - 0x400) & 0xfff
        }

        if isNametable4(address){
            return (address - 0x800) & 0xfff
        }
    } else if ppu.VerticalNametableMirror {
        if isNametable1(address){
            return address & 0xfff
        }

        if isNametable2(address){
            return address & 0xfff
        }

        if isNametable3(address){
            return (address - 0x800) & 0xfff
        }

        if isNametable4(address){
            return (address - 0x800) & 0xfff
        }
    }

    return 0x0
}

func (ppu *PPUState) LoadNametableMemory(address uint16) byte {
    return ppu.NametableMemory[ppu.nametableMirrorAddress(address)]
}

func (ppu *PPUState) StoreNametableMemory(address uint16, value byte) {
    ppu.NametableMemory[ppu.nametableMirrorAddress(address)] = value
}

func (ppu *PPUState) LoadBackgroundTile() {
    fineY, nametable, coarseY, coarseX := ppu.DeconstructVideoAddress()

    /* add the nametable, coarseY and coarseX to the tile address. mirroring is handled
     * in MirrorAddress()
     */
    tileAddress := 0x2000 | (ppu.VideoAddress & 0xfff)
    /* attributeAddress = 0x23c0 + nametable + high 3-bits of coarseY + high 3-bits of coarseX */
    attributeAddress := 0x23c0 | (ppu.VideoAddress & 0xc00) | ((ppu.VideoAddress >> 4) & 0x38) | ((ppu.VideoAddress >> 2) & 0x7)

    patternTable := ppu.GetBackgroundPatternTableBase()

    _ = coarseY
    _ = coarseX
    _ = nametable

    // tileIndex := ppu.VideoMemory[tileAddress]
    tileIndex := ppu.LoadNametableMemory(tileAddress)
    patternTileAddress := patternTable + uint16(tileIndex) * 16
    /* Within the tile, pull out row at an offset of fineY */
    left := ppu.VideoMemory[patternTileAddress + uint16(fineY)]
    right := ppu.VideoMemory[patternTileAddress + 8 + uint16(fineY)]

    // log.Printf("Left 0x%x right 0x%x", leftAddr, rightAddr)

    pattern_x := (coarseX/2) & 0x1
    pattern_y := (coarseY/2) & 0x1

    /* x to x+4
     * top left = x:x+1, y:y+1
     * top right = x+2:x+3, y:y+1
     * bottom left = x:x+1, y+2:y+3
     * bottom right = x+2:x+3, y+2:y+3
     */
    shifter := pattern_x * 2 + pattern_y * 4
    patternAttributeValue := ppu.LoadNametableMemory(attributeAddress)
    color_set := (patternAttributeValue >> shifter) & 0x3
    /* the actual palette to use */
    palette_base := uint16(color_set) * 4

    // if coarseX == 0 && coarseY == 20 && ppu.Debug > 0 {
    if ppu.Debug > 0 {
        log.Printf("Scanline %v cycle %v X %v Y %v fineX %v fineY %v nametable %v Tile address 0x%x tile 0x%x pattern address 0x%x attribute 0x%x Video address 0x%x Attribute value 0x%x color set %v", ppu.Scanline, ppu.ScanlineCycle, coarseX, coarseY, ppu.FineX, fineY, nametable, tileAddress, tileIndex, patternTileAddress, attributeAddress, ppu.VideoAddress, patternAttributeValue, color_set)
    }

    var rawPixel uint16
    var pixel uint32
    for i := 0; i < 8; i++ {
        low := (left >> i) & 1
        high := (right >> i) & 1
        colorIndex := (high << 1) | low

        rawPixel = (rawPixel << 2) | uint16(colorIndex)

        palette_color := palette_base + uint16(colorIndex)
        pixel = (pixel << 4) | uint32(palette_color)
    }

    //if coarseX <= 5 && coarseY == 3 {
        // ram := ppu.VideoMemory[patternTileAddress:patternTileAddress+16]
        // log.Printf("X %v Y %v pixel 0x%x raw pixel 0x%x pattern data %v", coarseX, coarseY, pixel, rawPixel, ram)
    // }

    /* Generate a new pixel, so include it in the register that holds the pixels */
    ppu.RawBackgroundPixels = (ppu.RawBackgroundPixels & 0xffff) | (uint32(rawPixel) << 16)
    /* Set the upper 32 bits to be the new pixel, keep the lower 32 bits */
    ppu.BackgroundPixels = (ppu.BackgroundPixels & 0xffffffff) | (uint64(pixel) << 32)

    ppu.Shifts = 8
    ppu.IncrementHorizontalAddress()
}

/* Load the first two tiles for the scanline */
func (ppu *PPUState) PreloadTiles() {
    ppu.LoadBackgroundTile()
    ppu.BackgroundPixels = ppu.BackgroundPixels >> 32
    ppu.RawBackgroundPixels = ppu.RawBackgroundPixels >> 16
    ppu.LoadBackgroundTile()
}

func (ppu *PPUState) Run(cycles uint64, screen VirtualScreen) (bool, bool) {
    /* http://wiki.nesdev.com/w/index.php/PPU_rendering */
    oldNMI := ppu.IsVerticalBlankFlagSet() && ppu.GetNMIOutput()
    didDraw := false
    for cycle := uint64(0); cycle < cycles; cycle++ {
        if ppu.IsBackgroundEnabled() || ppu.IsSpriteEnabled() {
            if ppu.Scanline < 240 && ppu.ScanlineCycle <= 256 {
                sprite0 := ppu.RenderPixel(ppu.Scanline, ppu.ScanlineCycle, ppu.CurrentSprites, &screen)

                /* Shift one pixel out of the background pixel buffer */
                ppu.BackgroundPixels = ppu.BackgroundPixels >> 4
                ppu.RawBackgroundPixels = ppu.RawBackgroundPixels >> 2

                /* 8 pixels have been shifted out of the shifting register
                 * so load 8 new pixels from the next background tile
                 */
                ppu.Shifts -= 1
                if ppu.Shifts == 0 {
                    ppu.LoadBackgroundTile()
                }

                /* FIXME: not sure if sprite0 should be set multiple times per frame or only once
                 */
                if !ppu.HasSetSprite0 && sprite0 {
                    ppu.HasSetSprite0 = true
                    // log.Printf("Sprite zero hit at scanline %v cycle %v", ppu.Scanline, ppu.ScanlineCycle)
                    ppu.SetSpriteZeroHit(true)
                }
            }
        }

        if (ppu.IsBackgroundEnabled() || ppu.IsSpriteEnabled()) && ppu.Scanline < 240 {
            if ppu.ScanlineCycle == 256 {
                ppu.IncrementVerticalPosition()
            }

            if ppu.ScanlineCycle == 257 {
                ppu.ResetHorizontalPosition()

                ppu.PreloadTiles()
            }
        }

        /* Finished drawing the scene */
        if ppu.Scanline == 240 && ppu.ScanlineCycle == 0 {
            /*
            drawOverlay(screen, 8, []uint8{255, 255, 255})
            drawOverlay(screen, 32, []uint8{255, 0, 0})
            */
            didDraw = true
            // log.Printf("Render complete")
        }

        if ppu.Scanline == 241 && ppu.ScanlineCycle == 1 {
            ppu.SetVerticalBlankFlag(true)
        }

        if ppu.Scanline == 260 && ppu.ScanlineCycle == 0 {
            ppu.SetVerticalBlankFlag(false)
        }

        ppu.ScanlineCycle += 1
        if ppu.ScanlineCycle > 340 {
            ppu.ScanlineCycle = 0
            ppu.Scanline += 1

            /* Load the sprites for the next scanline */
            if ppu.IsSpriteEnabled() && ppu.Scanline < 240 {
                sprites := ppu.GetSprites()
                ppu.CurrentSprites = nil
                count := 0
                size := 8
                if ppu.GetSpriteSize() == SpriteSize8x16 {
                    size = 16
                }
                for i := 0; i < len(sprites); i++ {
                    sprite := &sprites[i]
                    if ppu.Scanline >= int(sprite.y) && ppu.Scanline < int(sprite.y) + size {
                        ppu.CurrentSprites = append(ppu.CurrentSprites, *sprite)
                        count += 1
                        /* 8 sprite limit per scanline */
                        if count > 7 {
                            break
                        }
                    }
                }
            }

            if ppu.Scanline == 261 {
                ppu.SetSpriteZeroHit(false)
            }

            /* Prerender line */
            if ppu.Scanline == 262 {
                ppu.Scanline = 0

                ppu.HasSetSprite0 = false

                if ppu.IsBackgroundEnabled() || ppu.IsSpriteEnabled() {
                    ppu.VideoAddress = ppu.TemporaryVideoAddress

                    if ppu.Debug > 0 {
                        fineY, nametable, coarseY, coarseX := ppu.DeconstructVideoAddress()
                        log.Printf("Draw coarse x %v coarse y %v fine x %v fine y %v nametable %v horizontal %v vertical %v. Video address 0x%x. Temporary video address 0x%x Background color 0x%x", coarseX, coarseY, ppu.FineX, fineY, nametable, ppu.HorizontalNametableMirror, ppu.VerticalNametableMirror, ppu.VideoAddress, ppu.TemporaryVideoAddress, ppu.VideoMemory[0x3f00])
                    }

                    ppu.PreloadTiles()
                }
            }
        }
    }

    /* Only set NMI to true if the bit 7 of PPUCTRL is set */
    nmi := ppu.IsVerticalBlankFlagSet() && ppu.GetNMIOutput()
    return nmi && (oldNMI == false), didDraw
}

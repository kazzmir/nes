package common

import (
    "bytes"
    "encoding/binary"
    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
)

type RenderFunction func(*sdl.Renderer) error

type WindowSize struct {
    X int
    Y int
}

type ProgramActions interface {
}

type ProgramToggleSound struct {
}

type ProgramQuit struct {
}

type ProgramPauseEmulator struct {
}

type ProgramUnpauseEmulator struct {
}

type ProgramQueryAudioState struct {
    Response chan bool
}

type ProgramLoadRom struct {
    Path string
}

type PixelFormat uint32

/* determine endianness of the host by comparing the least-significant byte of a 32-bit number
 * versus a little endian byte array
 * if the first byte in the byte array is the same as the lowest byte of the 32-bit number
 * then the host is little endian
 */
func FindPixelFormat() PixelFormat {
    red := uint32(32)
    green := uint32(128)
    blue := uint32(64)
    alpha := uint32(96)
    color := (red << 24) | (green << 16) | (blue << 8) | alpha

    var buffer bytes.Buffer
    binary.Write(&buffer, binary.LittleEndian, color)

    if buffer.Bytes()[0] == uint8(alpha) {
        return sdl.PIXELFORMAT_ABGR8888
    }

    return sdl.PIXELFORMAT_RGBA8888
}

func WriteFont(font *ttf.Font, renderer *sdl.Renderer, x int, y int, message string, color sdl.Color) error {
    surface, err := font.RenderUTF8Blended(message, color)
    if err != nil {
        return err
    }

    defer surface.Free()

    texture, err := renderer.CreateTextureFromSurface(surface)
    if err != nil {
        return err
    }
    defer texture.Destroy()

    surfaceBounds := surface.Bounds()

    return CopyTexture(texture, renderer, surfaceBounds.Max.X, surfaceBounds.Max.Y, x, y)
}

func CopyTexture(texture *sdl.Texture, renderer *sdl.Renderer, width int, height int, x int, y int) error {
    sourceRect := sdl.Rect{X: 0, Y: 0, W: int32(width), H: int32(height)}
    destRect := sourceRect
    destRect.X = int32(x)
    destRect.Y = int32(y)

    return renderer.Copy(texture, &sourceRect, &destRect)
}


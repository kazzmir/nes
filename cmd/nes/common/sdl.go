package common

import (
    "bytes"
    "encoding/binary"
    "github.com/veandco/go-sdl2/sdl"
)

type RenderFunction func(*sdl.Renderer) error

type WindowSize struct {
    X int
    Y int
}

type ProgramActions int
const (
    ProgramToggleSound = iota
    ProgramQuit
    ProgramPauseEmulator
    ProgramUnpauseEmulator
)

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

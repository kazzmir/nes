package common

import (
    "bytes"
    "sort"
    "sync"
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

func TextWidth(font *ttf.Font, text string) int {
    /* FIXME: this feels a bit inefficient, maybe find a better way that doesn't require fully rendering the text */
    surface, err := font.RenderUTF8Solid(text, sdl.Color{R: 255, G: 255, B: 255, A: 255})
    if err != nil {
        return 0
    }

    defer surface.Free()
    return int(surface.W)
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

type RenderLayerList []RenderLayer

func (list RenderLayerList) Len() int {
    return len(list)
}

func (list RenderLayerList) Swap(a int, b int){
    list[a], list[b] = list[b], list[a]
}

func (list RenderLayerList) Less(a int, b int) bool {
    return list[a].ZIndex() < list[b].ZIndex()
}

type RenderInfo struct {
    Renderer *sdl.Renderer
    Font *ttf.Font
    SmallFont *ttf.Font
    Window *sdl.Window
}

type RenderLayer interface {
    Render(RenderInfo) error
    ZIndex() int // order of the layer
}

type RenderManager struct {
    Layers RenderLayerList
    /* FIXME: maybe use the actor-style message passing loop instead of a lock */
    Lock sync.Mutex
}

func (manager *RenderManager) Replace(index int, layer RenderLayer){
    manager.RemoveByIndex(index)
    manager.AddLayer(layer)
}

func (manager *RenderManager) RemoveByIndex(index int){
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    var out []RenderLayer

    for _, layer := range manager.Layers {
        if layer.ZIndex() != index {
            out = append(out, layer)
        }
    }

    manager.Layers = out

}

func (manager *RenderManager) AddLayer(layer RenderLayer){
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    manager.Layers = append(manager.Layers, layer)
    sort.Sort(manager.Layers)
}

func (manager *RenderManager) RemoveLayer(remove RenderLayer){
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    var out []RenderLayer

    for _, layer := range manager.Layers {
        if layer != remove {
            out = append(out, layer)
        }
    }

    manager.Layers = out
}

func Reverse[T any](in []T){
    max := len(in)
    for i := 0; i < max/2; i++ {
        j := max-i-1
        in[i], in[j] = in[j], in[i]
    }
}

func CopyArray[T any](in []T) []T {
    x := make([]T, len(in))
    copy(x, in)
    return x
}

func (manager *RenderManager) RenderAll(info RenderInfo) error {
    manager.Lock.Lock()
    layers := CopyArray(manager.Layers)
    manager.Lock.Unlock()

    for _, layer := range layers {
        err := layer.Render(info)
        if err != nil {
            return err
        }
    }

    return nil
}

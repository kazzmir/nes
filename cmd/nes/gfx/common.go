package gfx

import (
    "bytes"
    "sort"
    "sync"
    "math"
    "encoding/binary"
    "errors"
    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
    "github.com/veandco/go-sdl2/gfx"
)

type RenderFunction func(*sdl.Renderer) error

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

/* page 303 of computer graphics and geometric modeling: implementation & algorithms (vol 1)
 * https://isidore.co/calibre#book_id=5588&panel=book_details
 */
func rgb2hsv(color sdl.Color) (float32, float32, float32) {
    r := float64(color.R) / 255.0
    g := float64(color.G) / 255.0
    b := float64(color.B) / 255.0

    epsilon := 0.001

    max := math.Max(math.Max(r, g), b)
    min := math.Min(math.Min(r, g), b)

    v := max
    s := float64(0)
    h := float64(0)
    if max > epsilon {
        s = (max - min) / max
    }

    if s > epsilon {
        d := max - min
        if math.Abs(r - max) < epsilon {
            h = (g - b) / d
        } else if math.Abs(g - max) < epsilon {
            h = 2 + (b - r) / d
        } else {
            h = 4 + (r - g) / d
        }

        h = 60 * h
        if h < 0 {
            h += 360
        }
    }

    return float32(h), float32(s), float32(v)
}

/* input: h: 0-360, s: 0-1, v: 0-1
 * output: r: 0-1, b: 0-1, g: 0-1
 */
func hsv2rgb(h float32, s float32, v float32) (float32, float32, float32) {

    epsilon := 0.001

    h = float32(math.Abs(float64(h)))

    if math.Abs(float64(h) - 360) < epsilon {
        h = 0
    } else {
        h /= 60
    }

    fract := h - float32(math.Floor(float64(h)))

    p := v * (1.0 - s)
    q := v * (1.0 - s * fract)
    t := v * (1.0 - s * (1.0 - fract))

    if (0.0 <= h && h < 1.0){
        return v, t, p
    } else if 1.0 <= h && h < 2.0 {
        return q, v, p
    } else if 2.0 <= h && h < 3.0 {
        return p, v, t
    } else if 3.0 <= h && h < 4.0 {
        return p, q, v
    } else if 4.0 <= h && h < 5.0 {
        return t, p, v
    } else if 5.0 <= h && h < 6.0 {
        return v, p, q
    } else {
        return 0, 0, 0
    }
}

func interpolate(v1 float32, v2 float32, period float32, clock uint64) float32 {
    if v1 > v2 {
        v1, v2 = v2, v1
    }

    distance := v2 - v1

    p := math.Sin(float64((clock % uint64(period))) * (180.0 / float64(period)) * math.Pi / 180) * float64(distance)

    return v1 + float32(p)
}

func clamp(v float32, low float32, high float32) float32 {
    if v < low {
        v = low
    }
    if v > high {
        v = high
    }
    return v
}

/* smoothly interpolate between two colors. clock should be a monotonically increasing value.
 * speed governs how fast the change will take place where speed=2 will quickly bounce back and
 * forth between the two colors and speed>2 will interpolate at a slower pace.
 * speed is a period such that after 'speed' clocks the color will return to the start color.
 * thus, at half a period the color will be the end color.
 */
func Glow(start sdl.Color, end sdl.Color, speed float32, clock uint64) sdl.Color {
    startH, startS, startV := rgb2hsv(start)
    endH, endS, endV := rgb2hsv(end)

    h := interpolate(startH, endH, speed, clock)
    s := interpolate(startS, endS, speed, clock)
    v := interpolate(startV, endV, speed, clock)

    r, g, b := hsv2rgb(h, s, v)
    return sdl.Color{R: uint8(clamp(r*255, 0, 255)), G: uint8(clamp(g*255, 0, 255)), B: uint8(clamp(b*255, 0, 255)), A: 255}
}

func DrawEquilateralTriange(renderer *sdl.Renderer, x int, y int, size float64, angle float64, color sdl.Color) error {
    x1 := float64(x) + math.Cos(angle * math.Pi / 180) * size
    y1 := float64(y) - math.Sin(angle * math.Pi / 180) * size

    x2 := float64(x) + math.Cos((angle - 90) * math.Pi / 180) * size
    y2 := float64(y) - math.Sin((angle - 90) * math.Pi / 180) * size

    x3 := float64(x) + math.Cos((angle + 90) * math.Pi / 180) * size
    y3 := float64(y) - math.Sin((angle + 90) * math.Pi / 180) * size

    if !gfx.FilledTrigonColor(renderer, int32(x1), int32(y1), int32(x2), int32(y2), int32(x3), int32(y3), color) {
        return errors.New("Unable to render triangle")
    } else {
        return nil
    }
}

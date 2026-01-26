package gfx

import (
    _ "log"
    _ "bytes"
    "sort"
    "sync"
    "math"
    _ "slices"
    _ "image"
    "image/color"
    _ "cmp"
    _ "encoding/binary"
    // "errors"
    "github.com/hajimehoshi/ebiten/v2"
)

type RenderFunction func(*ebiten.Image) error

type PixelFormat uint32

/* determine endianness of the host by comparing the least-significant byte of a 32-bit number
 * versus a little endian byte array
 * if the first byte in the byte array is the same as the lowest byte of the 32-bit number
 * then the host is little endian
 */
 /*
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
*/

/*
func TextWidth(font *ttf.Font, text string) int {
    / * FIXME: this feels a bit inefficient, maybe find a better way that doesn't require fully rendering the text * /
    surface, err := font.RenderUTF8Solid(text, sdl.Color{R: 255, G: 255, B: 255, A: 255})
    if err != nil {
        log.Printf("Unable to render font text '%v': %v", text, err)
        return 0
    }

    defer surface.Free()
    return int(surface.W)
}
*/

/*
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
*/

/*
func CopyTexture(texture *sdl.Texture, renderer *sdl.Renderer, width int, height int, x int, y int) error {
    sourceRect := sdl.Rect{X: 0, Y: 0, W: int32(width), H: int32(height)}
    destRect := sourceRect
    destRect.X = int32(x)
    destRect.Y = int32(y)

    return renderer.Copy(texture, &sourceRect, &destRect)
}
*/

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
    Screen *ebiten.Image
    /*
    Renderer *sdl.Renderer
    Font *ttf.Font
    SmallFont *ttf.Font
    Window *sdl.Window
    */
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
// FIXME: replace with colorconv library
func rgb2hsv(col color.Color) (float32, float32, float32) {
    return 0, 0, 0
    /*
    r := float64(col.R) / 255.0
    g := float64(col.G) / 255.0
    b := float64(col.B) / 255.0

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
    */
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

func interpolate2(v1 float32, v2 float32, radians float64) float32 {
    distance := v2 - v1

    p := math.Sin(radians) * float64(distance)

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

/* smoothly interpolate from start to end given a maximum of steps. if step > steps, then the color
 * will just be the end color.
 */
 /*
func InterpolateColor(start sdl.Color, end sdl.Color, steps int, step int) sdl.Color {
    if step <= 0 {
        return start
    }
    if step >= steps {
        return end
    }

    // sin(step/steps*90*pi/180)
    startH, startS, startV := rgb2hsv(start)
    endH, endS, endV := rgb2hsv(end)

    radians := float64(step) / float64(steps) * 90 * math.Pi / 180

    h := interpolate2(startH, endH, radians)
    s := interpolate2(startS, endS, radians)
    v := interpolate2(startV, endV, radians)

    r, g, b := hsv2rgb(h, s, v)
    return sdl.Color{R: uint8(clamp(r*255, 0, 255)), G: uint8(clamp(g*255, 0, 255)), B: uint8(clamp(b*255, 0, 255)), A: 255}
}
*/

/* smoothly interpolate between two colors. clock should be a monotonically increasing value.
 * speed governs how fast the change will take place where speed=2 will quickly bounce back and
 * forth between the two colors and speed>2 will interpolate at a slower pace.
 * speed is a period such that after 'speed' clocks the color will return to the start color.
 * thus, at half a period the color will be the end color.
 */
func Glow(start color.Color, end color.Color, speed float32, clock uint64) color.Color {
    // FIXME
    /*
    startH, startS, startV := rgb2hsv(start)
    endH, endS, endV := rgb2hsv(end)

    h := interpolate(startH, endH, speed, clock)
    s := interpolate(startS, endS, speed, clock)
    v := interpolate(startV, endV, speed, clock)

    r, g, b := hsv2rgb(h, s, v)
    return sdl.Color{R: uint8(clamp(r*255, 0, 255)), G: uint8(clamp(g*255, 0, 255)), B: uint8(clamp(b*255, 0, 255)), A: 255}
    */
    return start
}

/*
func RasterizeTriangle(x1 int, y1 int, x2 int, y2 int, x3 int, y3 int, color sdl.Color) (*sdl.Surface, error) {
    // normalize points first
    minX := min(x1, x2, x3)
    minY := min(y1, y2, y3)
    x1 -= minX
    x2 -= minX
    x3 -= minX
    y1 -= minY
    y2 -= minY
    y3 -= minY

    width := max(x1, x2, x3) - min(x1, x2, x3)
    height := max(y1, y2, y3) - min(y1, y2, y3)

    surface, err := sdl.CreateRGBSurfaceWithFormat(0, int32(width), int32(height), 8 * 4, sdl.PIXELFORMAT_RGBA8888)
    // surface.FillRect(nil, sdl.Color{R: 0, G: 255, B: 0, A: 255}.Uint32())

    surface.Lock()
    defer surface.Unlock()

    pixels := surface.Pixels()

    points := []image.Point{image.Pt(x1, y1), image.Pt(x2, y2), image.Pt(x3, y3)}
    // sort by x position first
    slices.SortFunc(points, func(i image.Point, j image.Point) int {
        return cmp.Compare(i.X, j.X)
    })

    // index 0 is left most point
    // if index 1.y is above index 0.y then index 2 is counter-clockwise point, otherwise its index 1

    point1 := 0
    point2 := 1
    point3 := 2
    if points[1].Y > points[0].Y {
        // swap order
        point2 = 2
        point3 = 1
    }

    getDeterminant := func(a image.Point, b image.Point, c image.Point) int {
        ab := image.Pt(b.X - a.X, b.Y - a.Y)
        ac := image.Pt(c.X - a.X, c.Y - a.Y)

        return ab.Y * ac.X - ab.X * ac.Y
    }

    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            p := image.Pt(x, y)
            d1 := getDeterminant(points[point1], points[point2], p)
            d2 := getDeterminant(points[point2], points[point3], p)
            d3 := getDeterminant(points[point3], points[point1], p)

            // all on left or all on right
            if (d1 >= 0 && d2 >= 0 && d3 >= 0) || (d1 <= 0 && d2 <= 0 && d3 <= 0) {
                // log.Printf("triangle put pixel at %d, %d\n", x, y)
                offset := (y * width + x) * surface.BytesPerPixel()
                pixels[offset] = color.R
                pixels[offset+1] = color.G
                pixels[offset+2] = color.B
                pixels[offset+3] = color.A
            }
        }
    }

    return surface, err
}
*/

/*
func DrawEquilateralTriange(renderer *sdl.Renderer, x int, y int, size float64, angle float64, color sdl.Color) error {
    x1 := float64(x) + math.Cos(angle * math.Pi / 180) * size
    y1 := float64(y) - math.Sin(angle * math.Pi / 180) * size

    x2 := float64(x) + math.Cos((angle - 90) * math.Pi / 180) * size
    y2 := float64(y) - math.Sin((angle - 90) * math.Pi / 180) * size

    x3 := float64(x) + math.Cos((angle + 90) * math.Pi / 180) * size
    y3 := float64(y) - math.Sin((angle + 90) * math.Pi / 180) * size

    surface, err := RasterizeTriangle(int(x1), int(y1), int(x2), int(y2), int(x3), int(y3), color)
    if err != nil {
        return err
    }
    defer surface.Free()

    texture, err := renderer.CreateTextureFromSurface(surface)
    if err != nil {
        return err
    }
    defer texture.Destroy()

    return CopyTexture(texture, renderer, int(surface.W), int(surface.H), int(min(x1, x2, x3)), int(min(y1, y2, y3)))

    / *

    if !gfx.FilledTrigonColor(renderer, int32(x1), int32(y1), int32(x2), int32(y2), int32(x3), int32(y3), color) {
        return errors.New("Unable to render triangle")
    } else {
        return nil
    }
    * /
}
*/

type Coordinates struct {
    UpperLeftX int
    UpperLeftY int
    Width int
    Height int
}

func (coords Coordinates) X(x int) int {
    return x + coords.UpperLeftX
}

func (coords Coordinates) Y(y int) int {
    return y + coords.UpperLeftY
}

func (coords Coordinates) MaxX() int {
    return coords.Width
}

func (coords Coordinates) MaxY() int {
    return coords.Height
}

type GuiRenderer func(coords Coordinates)

/* draw a box that clips everything drawn inside of it by the bounds. the box has a 1px white border around it */
/*
func Box1(x int, y int, width int, height int, renderer *sdl.Renderer, render GuiRenderer){
    buffer := 1
    renderer.SetClipRect(&sdl.Rect{X: int32(x+buffer), Y: int32(y+buffer), W: int32(width-buffer*2), H: int32(height-buffer*2)})

    render(Coordinates{
        UpperLeftX: x+buffer,
        UpperLeftY: y+buffer,
        Width: width-buffer*2,
        Height: height-buffer*2,
    })

    renderer.SetClipRect(nil)
    renderer.SetDrawColor(255, 255, 255, 255)
    renderer.DrawRect(&sdl.Rect{X: int32(x), Y: int32(y), W: int32(width), H: int32(height)})
}
*/

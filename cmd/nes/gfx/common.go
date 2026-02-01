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

    "github.com/kazzmir/nes/lib/colorconv"

    // "errors"
    "github.com/hajimehoshi/ebiten/v2"
)

type RenderFunction func(*ebiten.Image) error

type PixelFormat uint32

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

func interpolate(v1 float64, v2 float64, period float64, clock uint64) float64 {
    if v1 > v2 {
        v1, v2 = v2, v1
    }

    distance := v2 - v1

    p := math.Sin(float64((clock % uint64(period))) * (180.0 / float64(period)) * math.Pi / 180) * float64(distance)

    return v1 + float64(p)
}

func interpolate2(v1 float64, v2 float64, radians float64) float64 {
    distance := v2 - v1

    p := math.Sin(radians) * distance

    return v1 + p
}

func clamp(v float64, low float64, high float64) float64 {
    return min(max(v, low), high)
}

/* smoothly interpolate from start to end given a maximum of steps. if step > steps, then the color
 * will just be the end color.
 */
func InterpolateColor(start color.Color, end color.Color, steps int, step int) color.Color {
    if step <= 0 {
        return start
    }
    if step >= steps {
        return end
    }

    // sin(step/steps*90*pi/180)
    startH, startS, startV := colorconv.ColorToHSV(start)
    endH, endS, endV := colorconv.ColorToHSV(end)

    radians := float64(step) / float64(steps) * 90 * math.Pi / 180

    h := clamp(interpolate2(startH, endH, radians), 0, 360)
    s := clamp(interpolate2(startS, endS, radians), 0, 1)
    v := clamp(interpolate2(startV, endV, radians), 0, 1)

    out, err := colorconv.HSVToColor(h, s, v)
    if err != nil {
        return end
    }
    return out
}

/* smoothly interpolate between two colors. clock should be a monotonically increasing value.
 * speed governs how fast the change will take place where speed=2 will quickly bounce back and
 * forth between the two colors and speed>2 will interpolate at a slower pace.
 * speed is a period such that after 'speed' clocks the color will return to the start color.
 * thus, at half a period the color will be the end color.
 */
func Glow(start color.Color, end color.Color, speed float64, clock uint64) color.Color {
    startH, startS, startV := colorconv.ColorToHSV(start)
    endH, endS, endV := colorconv.ColorToHSV(end)

    h := interpolate(startH, endH, speed, clock)
    s := interpolate(startS, endS, speed, clock)
    v := interpolate(startV, endV, speed, clock)

    out, err := colorconv.HSVToColor(h, s, v)
    if err != nil {
        return start
    }
    return out
}

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

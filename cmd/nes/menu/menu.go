package menu

import (
    "context"

    "math"
    "math/rand"
    "time"
    "log"

    "github.com/kazzmir/nes/cmd/nes/common"

    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
)

type MenuInput int
const (
    MenuToggle = iota
    MenuNext
    MenuPrevious
    MenuSelect
)

type Menu struct {
    quit context.Context
    cancel context.CancelFunc
    font *ttf.Font
    events chan sdl.Event
    Input chan MenuInput
}

type MenuAction int
const (
    MenuActionQuit = iota
    MenuActionLoadRom
)

type Snow struct {
    color uint8
    x float32
    y float32
    truex float32
    truey float32
    angle float32
    direction int
    speed float32
    fallSpeed float32
}

func MakeSnow(screenWidth int) Snow {
    x := rand.Float32() * float32(screenWidth)
    // y := rand.Float32() * 400
    y := float32(0)
    return Snow{
        color: uint8(rand.Int31n(210) + 40),
        x: x,
        y: y,
        truex: x,
        truey: y,
        angle: rand.Float32() * 180,
        direction: 1,
        speed: rand.Float32() * 4 + 1,
        fallSpeed: rand.Float32() * 2.5 + 0.8,
    }
}

func copySnow(snow []Snow) []Snow {
    out := make([]Snow, len(snow))
    copy(out, snow)
    return out
}

func drawButton(font *ttf.Font, renderer *sdl.Renderer, x int, y int, message string, color sdl.Color) (int, int, error) {
    buttonInside := sdl.Color{R: 64, G: 64, B: 64, A: 255}
    buttonOutline := sdl.Color{R: 32, G: 32, B: 32, A: 255}

    surface, err := font.RenderUTF8Blended(message, color)
    if err != nil {
        return 0, 0, err
    }

    defer surface.Free()

    texture, err := renderer.CreateTextureFromSurface(surface)
    if err != nil {
        return 0, 0, err
    }
    defer texture.Destroy()

    surfaceBounds := surface.Bounds()

    margin := 12

    renderer.SetDrawColor(buttonOutline.R, buttonOutline.G, buttonOutline.B, buttonOutline.A)
    renderer.FillRect(&sdl.Rect{X: int32(x), Y: int32(y), W: int32(surfaceBounds.Max.X + margin), H: int32(surfaceBounds.Max.Y + margin)})

    renderer.SetDrawColor(buttonInside.R, buttonInside.G, buttonInside.B, buttonInside.A)
    renderer.FillRect(&sdl.Rect{X: int32(x+1), Y: int32(y+1), W: int32(surfaceBounds.Max.X + margin - 3), H: int32(surfaceBounds.Max.Y + margin - 3)})

    sourceRect := sdl.Rect{X: 0, Y: 0, W: int32(surfaceBounds.Max.X), H: int32(surfaceBounds.Max.Y)}
    destRect := sourceRect
    destRect.X = int32(x + margin/2)
    destRect.Y = int32(y + margin/2)

    renderer.Copy(texture, &sourceRect, &destRect)
    return surfaceBounds.Max.X, surfaceBounds.Max.Y, nil
}

func writeFont(font *ttf.Font, renderer *sdl.Renderer, x int, y int, message string, color sdl.Color) error {
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

    sourceRect := sdl.Rect{X: 0, Y: 0, W: int32(surfaceBounds.Max.X), H: int32(surfaceBounds.Max.Y)}
    destRect := sourceRect
    destRect.X = int32(x)
    destRect.Y = int32(y)

    renderer.Copy(texture, &sourceRect, &destRect)

    return nil
}

func MakeMenu(font *ttf.Font, mainQuit context.Context, mainCancel context.CancelFunc, renderUpdates chan common.RenderFunction, windowSizeUpdates chan common.WindowSize) Menu {
    quit, cancel := context.WithCancel(mainQuit)
    events := make(chan sdl.Event)
    menuInput := make(chan MenuInput)

    go func(){
        active := false
        snowTicker := time.NewTicker(time.Second / 20)
        defer snowTicker.Stop()

        choices := []MenuAction{MenuActionQuit, MenuActionLoadRom}
        choice := 0

        update := func(choice int, maxWidth int, maxHeight int, snowflakes []Snow){
            snowCopy := copySnow(snowflakes)
            renderUpdates <- func (renderer *sdl.Renderer) error {
                var err error
                yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
                white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
                renderer.SetDrawColor(32, 0, 0, 192)
                renderer.FillRect(nil)

                for _, snow := range snowCopy {
                    c := snow.color
                    renderer.SetDrawColor(c, c, c, 255)
                    renderer.DrawPoint(int32(snow.x), int32(snow.y))
                }

                colors := []sdl.Color{white, white}
                colors[choice] = yellow

                quitWidth, quitHeight, err := drawButton(font, renderer, 50, 50, "Quit", colors[0])
                loadWidth, loadHeight, err := drawButton(font, renderer, 50 + quitWidth + 50, 50, "Load rom", colors[1])

                _ = quitWidth
                _ = quitHeight
                _ = loadWidth
                _ = loadHeight

                // err = writeFont(font, renderer, 50, 50, "Quit", colors[0])
                err = writeFont(font, renderer, maxWidth - 200, maxHeight - font.Height() * 3, "NES Emulator", white)
                err = writeFont(font, renderer, maxWidth - 200, maxHeight - font.Height() * 3 + font.Height() + 3, "Jon Rafkind", white)
                _ = err

                return nil
            }
        }

        var snow []Snow

        wind := rand.Float32() - 0.5
        var windowSize common.WindowSize

        /* Reset the default renderer */
        for {
            select {
                case <-quit.Done():
                    return
                case input := <-menuInput:
                    switch input {
                        case MenuToggle:
                            if active {
                                renderUpdates <- func(renderer *sdl.Renderer) error {
                                    return nil
                                }
                            } else {
                                choice = 0
                                update(choice, windowSize.X, windowSize.Y, snow)
                            }

                            active = ! active
                        case MenuNext:
                            if active {
                                choice = (choice + 1) % len(choices)
                                update(choice, windowSize.X, windowSize.Y, snow)
                            }
                        case MenuPrevious:
                            if active {
                                choice = (choice + 1) % len(choices)
                                update(choice, windowSize.X, windowSize.Y, snow)
                            }
                        case MenuSelect:
                            if active {
                                switch choices[choice] {
                                    case MenuActionQuit:
                                        mainCancel()
                                    case MenuActionLoadRom:
                                        log.Printf("Load a rom")
                                }
                            }
                    }

                case windowSize = <-windowSizeUpdates:
                case <-snowTicker.C:
                    if active {
                        if len(snow) < 300 {
                            snow = append(snow, MakeSnow(windowSize.X))
                        }

                        wind += (rand.Float32() - 0.5) / 4
                        if wind < -1 {
                            wind = -1
                        }
                        if wind > 1 {
                            wind = 1
                        }

                        for i := 0; i < len(snow); i++ {
                            snow[i].truey += snow[i].fallSpeed
                            snow[i].truex += wind
                            snow[i].x = snow[i].truex + float32(math.Cos(float64(snow[i].angle + 180) * math.Pi / 180.0) * 8)
                            // snow[i].y = snow[i].truey + float32(-math.Sin(float64(snow[i].angle + 180) * math.Pi / 180.0) * 8)
                            snow[i].y = snow[i].truey
                            snow[i].angle += float32(snow[i].direction) * snow[i].speed

                            if snow[i].y > float32(windowSize.Y) {
                                snow[i] = MakeSnow(windowSize.X)
                            }

                            if snow[i].angle < 0 {
                                snow[i].angle = 0
                                snow[i].direction = -snow[i].direction
                            }
                            if snow[i].angle >= 180  {
                                snow[i].angle = 180
                                snow[i].direction = -snow[i].direction
                            }
                        }

                        update(choice, windowSize.X, windowSize.Y, snow)
                    }
                case event := <-events:
                    if event.GetType() == sdl.QUIT {
                        cancel()
                    }
            }
        }
    }()

    return Menu{
        quit: quit,
        cancel: cancel,
        font: font,
        events: events,
        Input: menuInput,
    }
}

func (menu *Menu) Close() {
    menu.cancel()
}

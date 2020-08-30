package menu

import (
    "context"

    "os"
    "path/filepath"
    "fmt"
    "math"
    "math/rand"
    "time"
    "log"
    "sync"

    "github.com/kazzmir/nes/cmd/nes/common"
    nes "github.com/kazzmir/nes/lib"

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
    active bool
    quit context.Context
    cancel context.CancelFunc
    font *ttf.Font
    events chan sdl.Event
    Input chan MenuInput
    Lock sync.Mutex
}

type MenuAction int
const (
    MenuActionQuit = iota
    MenuActionLoadRom
    MenuActionSound
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

/* FIXME: good use-case for generics */
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

type MenuState int
const (
    MenuStateTop = iota
    MenuStateLoadRom
)

func chainRenders(functions ...common.RenderFunction) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {
        for _, f := range functions {
            err := f(renderer)
            if err != nil {
                return err
            }
        }

        return nil
    }
}

type NullInput struct {
}

func (buttons *NullInput) Get() nes.ButtonMapping {
    return make(nes.ButtonMapping)
}

/* Find roms and show thumbnails of them, then let the user select one */
func romLoader(mainQuit context.Context) (nes.NESFile, error) {
    /* for each rom call runNES() and pass in EmulatorInfiniteSpeed to let
     * the emulator run as fast as possible. Pass in a maxCycle of whatever
     * correlates to about 4 seconds of runtime. Save the screens produced
     * every 1s, so there should be about 4 screenshots. Then the thumbnail
     * should cycle through all the screenshots.
     * Let the user pick a thumbnail, and when selecting a thumbnail
     * return that nesfile so it can be played normally.
     */

    possibleRoms := make(chan string, 1000)

    loaderQuit, loaderCancel := context.WithCancel(mainQuit)
    _ = loaderCancel

    /* 4 seconds worth of cycles */
    const maxCycles = uint64(4 * nes.CPUSpeed)

    var wait sync.WaitGroup

    /* Have 4 go routines running roms */
    for i := 0; i < 4; i++ {
        go func(){
            wait.Add(1)
            defer wait.Done()

            for rom := range possibleRoms {
                nesFile, err := nes.ParseNesFile(rom, false)
                if err != nil {
                    log.Printf("Unable to parse nes file %v: %v", rom, err)
                    continue
                }

                cpu, err := common.SetupCPU(nesFile, false)
                if err != nil {
                    log.Printf("Unable to setup cpu for %v: %v", rom, err)
                    continue
                }

                quit, cancel := context.WithCancel(loaderQuit)

                cpu.Input = nes.MakeInput(&NullInput{})

                audioOutput := make(chan []float32, 1)
                emulatorActionsInput := make(chan common.EmulatorAction, 5)
                emulatorActionsInput <- common.EmulatorInfinite
                var screenListeners common.ScreenListeners
                const AudioSampleRate float32 = 44100.0

                toDraw := make(chan nes.VirtualScreen, 1)
                bufferReady := make(chan nes.VirtualScreen, 1)

                buffer := nes.MakeVirtualScreen(256, 240)
                bufferReady <- buffer

                saveFrames := make(chan nes.VirtualScreen, 10)
                go func(){
                    count := 0
                    for {
                        select {
                            case <-quit.Done():
                                return
                            case screen := <-toDraw:
                                count += 1
                                if count == 60 {
                                    /* Try to save the frame in the channel */
                                    select {
                                        case saveFrames <- screen.Copy():
                                        default:
                                    }
                                    count = 0
                                }

                                bufferReady <- screen
                        }
                    }
                }()

                log.Printf("Start loading %v", rom)
                err = common.RunNES(&cpu, maxCycles, quit, toDraw, bufferReady, audioOutput, emulatorActionsInput, &screenListeners, AudioSampleRate, 0)
                if err == common.MaxCyclesReached {
                    log.Printf("%v complete", rom)
                }

                cancel()
                close(saveFrames)

                count := 0
                for frame := range saveFrames {
                    _ = frame
                    count += 1
                }

                log.Printf("%v had %v frames", rom, count)
            }
        }()
    }

    err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
        if mainQuit.Err() != nil {
            return fmt.Errorf("quitting")
        }

        if nes.IsNESFile(path){
            // log.Printf("Possible nes file %v", path)
            possibleRoms <- path
        }

        return nil
    })

    close(possibleRoms)

    wait.Wait()

    return nes.NESFile{}, err
}

func (menu *Menu) IsActive() bool {
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    return menu.active
}

func (menu *Menu) ToggleActive(){
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    menu.active = ! menu.active
}

func MakeMenu(font *ttf.Font, mainQuit context.Context, renderUpdates chan common.RenderFunction, windowSizeUpdates <-chan common.WindowSize, programActions chan<- common.ProgramActions) *Menu {
    quit, cancel := context.WithCancel(mainQuit)
    events := make(chan sdl.Event)
    menuInput := make(chan MenuInput)

    menu := &Menu{
        active: false,
        quit: quit,
        cancel: cancel,
        font: font,
        events: events,
        Input: menuInput,
    }

    go func(menu *Menu){
        snowTicker := time.NewTicker(time.Second / 20)
        defer snowTicker.Stop()

        choices := []MenuAction{MenuActionQuit, MenuActionLoadRom, MenuActionSound}
        choice := 0

        var snow []Snow

        wind := rand.Float32() - 0.5
        var windowSize common.WindowSize
        audio := true

        menuState := MenuStateTop

        baseRenderer := func(renderer *sdl.Renderer) error {
            renderer.SetDrawColor(32, 0, 0, 192)
            renderer.FillRect(nil)
            return nil
        }

        makeSnowRenderer := func(snowflakes []Snow) common.RenderFunction {
            snowCopy := copySnow(snowflakes)
            return func(renderer *sdl.Renderer) error {
                for _, snow := range snowCopy {
                    c := snow.color
                    renderer.SetDrawColor(c, c, c, 255)
                    renderer.DrawPoint(int32(snow.x), int32(snow.y))
                }
                return nil
            }
        }

        makeMenuRenderer := func(choice int, maxWidth int, maxHeight int, audioEnabled bool) common.RenderFunction {
            return func(renderer *sdl.Renderer) error {
                var err error
                yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
                white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

                sound := "Sound enabled"
                if !audioEnabled {
                    sound = "Sound disabled"
                }

                buttons := []string{"Quit", "Load ROM", sound}

                x := 50
                y := 50
                for i, button := range buttons {
                    color := white
                    if i == choice {
                        color = yellow
                    }
                    width, height, err := drawButton(font, renderer, x, y, button, color)
                    x += width + 50
                    _ = height
                    _ = err
                }

                // err = writeFont(font, renderer, 50, 50, "Quit", colors[0])
                err = writeFont(font, renderer, maxWidth - 200, maxHeight - font.Height() * 3, "NES Emulator", white)
                err = writeFont(font, renderer, maxWidth - 200, maxHeight - font.Height() * 3 + font.Height() + 3, "Jon Rafkind", white)
                _ = err
                return err
            }
        }

        makeLoadRomRenderer := func() common.RenderFunction {
            return func(renderer *sdl.Renderer) error {
                white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
                writeFont(font, renderer, 1, 1, "Load a rom", white)
                return nil
            }
        }

        menuRenderer := func(renderer *sdl.Renderer) error {
            return nil
        }
        snowRenderer := makeSnowRenderer(nil)

        loadRomQuit, loadRomCancel := context.WithCancel(mainQuit)

        /* Reset the default renderer */
        for {
            select {
                case <-quit.Done():
                    return
                case input := <-menuInput:
                    switch menuState {
                        case MenuStateTop:
                            switch input {
                                case MenuToggle:
                                    if menu.IsActive() {
                                        renderUpdates <- func(renderer *sdl.Renderer) error {
                                            return nil
                                        }
                                        programActions <- common.ProgramUnpauseEmulator
                                    } else {
                                        choice = 0
                                        menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                        renderUpdates <- chainRenders(baseRenderer, menuRenderer, snowRenderer)
                                        programActions <- common.ProgramPauseEmulator
                                    }

                                    menu.ToggleActive()
                                case MenuNext:
                                    if menu.IsActive() {
                                        choice = (choice + 1) % len(choices)
                                        menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                        renderUpdates <- chainRenders(baseRenderer, menuRenderer, snowRenderer)
                                    }
                                case MenuPrevious:
                                    if menu.IsActive() {
                                        choice = (choice - 1 + len(choices)) % len(choices)
                                        menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                        renderUpdates <- chainRenders(baseRenderer, menuRenderer, snowRenderer)
                                    }
                                case MenuSelect:
                                    if menu.IsActive() {
                                        switch choices[choice] {
                                            case MenuActionQuit:
                                                programActions <- common.ProgramQuit
                                            case MenuActionLoadRom:
                                                menuState = MenuStateLoadRom
                                                loadRomQuit, loadRomCancel = context.WithCancel(mainQuit)
                                                go romLoader(loadRomQuit)

                                                menuRenderer = makeLoadRomRenderer()

                                            case MenuActionSound:
                                                programActions <- common.ProgramToggleSound
                                                audio = !audio
                                                menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                        }
                                    }
                            }

                        case MenuStateLoadRom:
                            switch input {
                                case MenuToggle:
                                    loadRomCancel()
                                    menuState = MenuStateTop
                                    menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                            }
                    }

                case windowSize = <-windowSizeUpdates:
                case <-snowTicker.C:
                    if menu.IsActive() {
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

                        snowRenderer = makeSnowRenderer(snow)
                        renderUpdates <- chainRenders(baseRenderer, menuRenderer, snowRenderer)
                    }
                case event := <-events:
                    if event.GetType() == sdl.QUIT {
                        programActions <- common.ProgramQuit
                    }
            }
        }
    }(menu)

    return menu
}

func (menu *Menu) Close() {
    menu.cancel()
}

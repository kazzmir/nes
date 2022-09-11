package debug

import (
    "sync"
    "context"
    "fmt"
    "log"
    "github.com/kazzmir/nes/cmd/nes/gfx"
    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
)

type WindowRequest any

type WindowRequestWindow struct {
    Response chan *sdl.Window
}

type WindowRequestRedraw struct {
}

type WindowRequestText struct {
    Text string
}

type WindowRequestRaise struct {
}

type Line struct {
    Text string
}

type DebugWindow struct {
    opener sync.Once
    Quit context.Context
    Cancel context.CancelFunc
    Requests chan WindowRequest
    IsOpen bool
    Wait sync.WaitGroup
    BigFont *ttf.Font
    SmallFont *ttf.Font
    Line Line
}

func MakeDebugWindow(mainQuit context.Context, bigFont *ttf.Font, smallFont *ttf.Font) *DebugWindow {
    quit, cancel := context.WithCancel(mainQuit)
    return &DebugWindow{
        Quit: quit,
        Cancel: cancel,
        Requests: make(chan WindowRequest, 5),
        IsOpen: false,
        BigFont: bigFont,
        SmallFont: smallFont,
        Line: Line{},
    }
}

func (debug *DebugWindow) doOpen(quit context.Context) error {
    var window *sdl.Window
    var renderer *sdl.Renderer
    var err error

    sdl.Do(func(){
        window, renderer, err = sdl.CreateWindowAndRenderer(600, 600, sdl.WINDOW_SHOWN | sdl.WINDOW_RESIZABLE)

        if window != nil {
            window.SetTitle("Nes Emulator Debugger")
        }
    })

    if err != nil {
        return err
    }

    defer sdl.Do(func(){
        window.Destroy()
    })
    defer sdl.Do(func(){
        renderer.Destroy()
    })

    white := sdl.Color{
        R: 255,
        G: 255,
        B: 255,
        A: 255,
    }

    render := func(width int, height int, renderer *sdl.Renderer){
        renderer.SetDrawColor(0, 0, 0, 0)
        renderer.Clear()

        gfx.WriteFont(debug.BigFont, renderer, 1, 1, "Debugger", white)

        renderer.SetDrawColor(255, 255, 255, 255)
        y := height - debug.SmallFont.Height() - 2
        renderer.DrawLine(0, int32(y), int32(width), int32(y))
        y += 2
        gfx.WriteFont(debug.SmallFont, renderer, 1, y, fmt.Sprintf("> %v", debug.Line.Text), white)

        renderer.Present()
    }
    
    redraw := make(chan bool, 1)
    redraw <- true

    for {
        select {
            case <-quit.Done():
                return nil
            case <-redraw:
                windowWidth, windowHeight := window.GetSize()
                sdl.Do(func(){
                    render(int(windowWidth), int(windowHeight), renderer)
                })
            case request := <-debug.Requests:
                windowRequest, ok := request.(WindowRequestWindow)
                if ok {
                    windowRequest.Response <- window
                }

                _, ok = request.(WindowRequestRaise)
                if ok {
                    sdl.Do(func(){
                        window.Raise()
                    })
                }

                _, ok = request.(WindowRequestRedraw)
                if ok {
                    select {
                        case redraw <- true:
                        default:
                    }
                }

                input, ok := request.(WindowRequestText)
                if ok {
                    debug.Line.Text += input.Text
                    select {
                        case redraw <- true:
                        default:
                    }
                }
        }
    }

    return nil
}

func (debug *DebugWindow) IsWindow(window *sdl.Window) bool {
    if !debug.IsOpen {
        return false
    }

    request := WindowRequestWindow{
        Response: make(chan *sdl.Window),
    }
    debug.Requests <- request
    out := <-request.Response
    return out == window
}

func (debug *DebugWindow) Open(){
    debug.opener.Do(func(){
        debug.Wait.Add(1)
        go func(){
            defer debug.Wait.Done()
            debug.IsOpen = true
            err := debug.doOpen(debug.Quit)
            if err != nil {
                log.Printf("Could not open debug window: %v", err)
            }
            debug.IsOpen = false
        }()
    })

    debug.Requests <- WindowRequestRaise{}
}

func (debug *DebugWindow) Close(mainQuit context.Context) {
    go func(){
        debug.Cancel()
        debug.Wait.Wait()
        debug.opener = sync.Once{}
        quit, cancel := context.WithCancel(mainQuit)
        debug.Quit = quit
        debug.Cancel = cancel
    }()
}

func (debug *DebugWindow) Redraw() {
    select {
        case debug.Requests <- WindowRequestRedraw{}:
        default:
    }
}

func (debug *DebugWindow) HandleText(event sdl.Event){
    switch event.GetType() {
        case sdl.TEXTINPUT:
            input := event.(*sdl.TextInputEvent)
            message := WindowRequestText{
                Text: input.GetText(),
            }

            debug.Requests <- message
    }
}

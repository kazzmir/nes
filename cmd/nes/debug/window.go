package debug

import (
    "sync"
    "context"
    "log"
    "github.com/veandco/go-sdl2/sdl"
)

type WindowRequest any

type WindowRequestWindow struct {
    Response chan *sdl.Window
}

type DebugWindow struct {
    opener sync.Once
    Quit context.Context
    Cancel context.CancelFunc
    Requests chan WindowRequest
    IsOpen bool
    Wait sync.WaitGroup
}

func MakeDebugWindow(mainQuit context.Context) *DebugWindow {
    quit, cancel := context.WithCancel(mainQuit)
    return &DebugWindow{
        Quit: quit,
        Cancel: cancel,
        Requests: make(chan WindowRequest, 2),
        IsOpen: false,
    }
}

func (debug *DebugWindow) doOpen(quit context.Context) error {
    var window *sdl.Window
    var renderer *sdl.Renderer
    var err error

    sdl.Do(func(){
        window, renderer, err = sdl.CreateWindowAndRenderer(500, 500, sdl.WINDOW_SHOWN | sdl.WINDOW_RESIZABLE)

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

    for {
        select {
            case <-quit.Done():
                return nil
            case request := <-debug.Requests:
                windowRequest, ok := request.(WindowRequestWindow)
                if ok {
                    windowRequest.Response <- window
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

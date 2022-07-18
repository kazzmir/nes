package main

import (
    "time"
    "context"
    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/veandco/go-sdl2/sdl"
)

type ConsoleState int
const (
    StateOpening = iota
    StateOpen
    StateClosing
    StateClosed
)

type Message interface {
    ConsoleMessage()
}

type ToggleMessage struct {
}

func (toggle ToggleMessage) ConsoleMessage() {
}

type Console struct {
    RenderManager *common.RenderManager
    State ConsoleState
    Messages chan Message
    ZIndex int
}

func MakeConsole(zindex int, manager *common.RenderManager, quit context.Context, renderNow chan bool) *Console {
    console := Console{
        RenderManager: manager,
        State: StateClosed,
        Messages: make(chan Message, 5),
        ZIndex: zindex,
    }

    go console.Run(quit, renderNow)

    return &console
}

type RenderConsoleLayer struct {
    Index int
    Size int
}

func (layer *RenderConsoleLayer) ZIndex() int {
    return layer.Index
}

func (layer *RenderConsoleLayer) Render(info common.RenderInfo) error {
    renderer := info.Renderer
    renderer.SetDrawColor(255, 0, 0, 200)

    windowWidth, windowHeight := info.Window.GetSize()
    _ = windowHeight

    y := layer.Size * 30

    renderer.FillRect(&sdl.Rect{X: int32(0), Y: int32(0), W: int32(windowWidth), H: int32(y)})
    renderer.SetDrawColor(255, 255, 255, 200)
    renderer.DrawLine(0, int32(y), int32(windowWidth), int32(y))

    return nil
}

func (console *Console) Run(mainQuit context.Context, renderNow chan bool){
    ticker := time.NewTicker(time.Millisecond * 30)
    defer ticker.Stop()
    maxSize := 7

    layer := RenderConsoleLayer{
        Index: console.ZIndex,
        Size: 0,
    }

    defer console.RenderManager.RemoveLayer(&layer)

    for {
        select {
            case <-mainQuit.Done():
                return
            case message := <-console.Messages:
                _, ok := message.(ToggleMessage)
                if ok {
                    switch console.State {
                        case StateOpen:
                            console.State = StateClosing
                        case StateOpening:
                            console.State = StateClosing
                        case StateClosing:
                            console.State = StateOpening
                            console.RenderManager.Replace(console.ZIndex, &layer)
                        case StateClosed:
                            console.State = StateOpening
                            console.RenderManager.Replace(console.ZIndex, &layer)
                    }
                    select {
                        case renderNow <-true:
                        default:
                    }
                }
            case <-ticker.C:
                switch console.State {
                    case StateOpening:
                        if layer.Size < maxSize {
                            layer.Size += 1
                            select {
                                case renderNow <-true:
                                default:
                            }
                        } else {
                            console.State = StateOpen
                        }
                    case StateClosing:
                        if layer.Size > 0 {
                            layer.Size -= 1
                            select {
                                case renderNow <-true:
                                default:
                            }
                        } else {
                            console.RenderManager.RemoveByIndex(console.ZIndex)
                            console.State = StateClosed
                        }
                }
        }
    }
}

func (console *Console) Toggle(){
    select {
        case console.Messages <- ToggleMessage{}:
        default:
    }
}

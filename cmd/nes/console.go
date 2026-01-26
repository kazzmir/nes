package main

import (
    "time"
    "context"
    "log"
    "fmt"
    "strings"
    "strconv"
    "sync"
    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/kazzmir/nes/cmd/nes/gfx"
    "github.com/kazzmir/nes/cmd/nes/debug"
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

type IsActiveMessage struct {
    Response chan bool
}

func (active IsActiveMessage) ConsoleMessage() {
}

type TextInputMessage struct {
    Text string
}

func (input TextInputMessage) ConsoleMessage() {
}

type BackspaceMessage struct {
}

func (backspace BackspaceMessage) ConsoleMessage() {
}

type RemoveWordMessage struct {
}

func (remove RemoveWordMessage) ConsoleMessage() {
}

type ClearLineMessage struct {
}

func (clear ClearLineMessage) ConsoleMessage() {
}

type EnterMessage struct {
}

func (enter EnterMessage) ConsoleMessage() {
}

type Console struct {
    RenderManager *gfx.RenderManager
    State ConsoleState
    Messages chan Message
    ZIndex int
}

func MakeConsole(zindex int, manager *gfx.RenderManager, cancel context.CancelFunc, quit context.Context, emulatorActions chan<- common.EmulatorAction, nesActions chan NesAction, renderNow chan bool) *Console {
    console := Console{
        RenderManager: manager,
        State: StateClosed,
        Messages: make(chan Message, 5),
        ZIndex: zindex,
    }

    go console.run(cancel, quit, emulatorActions, nesActions, renderNow)

    return &console
}

type RenderConsoleLayer struct {
    Index int
    Size int
    Lines []string
    Text string
    Lock sync.Mutex
}

func (layer *RenderConsoleLayer) ZIndex() int {
    return layer.Index
}

func (layer *RenderConsoleLayer) GetText() string {
    layer.Lock.Lock()
    defer layer.Lock.Unlock()

    return layer.Text
}

func (layer *RenderConsoleLayer) AddLine(line string){
    layer.Lock.Lock()
    defer layer.Lock.Unlock()
    layer.Lines = append(layer.Lines, line)
}

func (layer *RenderConsoleLayer) ClearLines(){
    layer.Lock.Lock()
    defer layer.Lock.Unlock()
    layer.Lines = nil
}

func (layer *RenderConsoleLayer) SetText(text string){
    layer.Lock.Lock()
    defer layer.Lock.Unlock()
    layer.Text = text
}

func (layer *RenderConsoleLayer) Render(info gfx.RenderInfo) error {
    /*
    renderer := info.Renderer
    var alpha uint8 = 200
    renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
    renderer.SetDrawColor(255, 0, 0, alpha)

    windowWidth, windowHeight := info.Window.GetSize()
    _ = windowHeight

    y := layer.Size * 22

    renderer.FillRect(&sdl.Rect{X: int32(0), Y: int32(0), W: int32(windowWidth), H: int32(y)})
    renderer.SetDrawColor(255, 255, 255, alpha)
    renderer.DrawLine(0, int32(y), int32(windowWidth), int32(y))

    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
    grey := sdl.Color{R: 200, G: 200, B: 200, A: 255}

    yPos := y - info.SmallFont.Height() - 1

    gfx.WriteFont(info.SmallFont, renderer, 1, yPos, fmt.Sprintf("> %s|", layer.GetText()), white)

    layer.Lock.Lock()
    max := len(layer.Lines)
    if max > 30 {
        max = 30
    }
    lines := gfx.CopyArray(layer.Lines[len(layer.Lines)-max:len(layer.Lines)])
    layer.Lock.Unlock()
    gfx.Reverse(lines)

    / * show all previous lines * /
    for _, line := range lines {
        yPos -= info.SmallFont.Height() - 1
        if yPos < -info.SmallFont.Height() {
            break
        }
        gfx.WriteFont(info.SmallFont, renderer, 1, yPos, line, grey)
    }
    */

    return nil
}

const helpText string = `
help, ?: this help text
exit, quit: quit the program
clear: clear console text
info: show emulator info
reload, restart: reload the current rom
debug: open debug window
`

func (console *Console) GetDebugger(emulatorActions chan<- common.EmulatorAction) debug.Debugger {
    data := make(chan debug.Debugger)
    response := common.EmulatorActionGetDebugger{
        Response: data,
    }
    select {
        case emulatorActions<-response:
            for debugger := range data {
                return debugger
            }
        default:
    }
    return nil
}

func firstString(strings []string) string {
    if len(strings) > 0 {
        return strings[0]
    }
    return ""
}

func (console *Console) run(mainCancel context.CancelFunc, mainQuit context.Context, emulatorActions chan<- common.EmulatorAction, nesActions chan NesAction, renderNow chan bool){
    normalTime := time.Millisecond * 13
    slowTime := time.Hour * 100
    ticker := time.NewTicker(slowTime)
    defer ticker.Stop()
    maxSize := 10

    layer := RenderConsoleLayer{
        Index: console.ZIndex,
        Size: 0,
        Text: "",
    }

    defer console.RenderManager.RemoveLayer(&layer)

    for {
        select {
            case <-mainQuit.Done():
                return
            case message := <-console.Messages:

                switch message.(type) {
                    case ToggleMessage:
                        switch console.State {
                            case StateOpen, StateOpening:
                                console.State = StateClosing
                                ticker.Reset(normalTime)
                                // sdl.Do(sdl.StopTextInput)
                            case StateClosing, StateClosed:
                                // sdl.Do(sdl.StartTextInput)
                                ticker.Reset(normalTime)
                                console.State = StateOpening
                                console.RenderManager.Replace(console.ZIndex, &layer)
                        }
                        select {
                            case renderNow <-true:
                            default:
                        }
                    case TextInputMessage:
                        text := message.(TextInputMessage)
                        newText := layer.GetText() + text.Text
                        if len(newText) > 1024 {
                            newText = newText[0:1024]
                        }
                        layer.SetText(newText)
                        select {
                            case renderNow <-true:
                            default:
                        }
                    case RemoveWordMessage:
                        text := strings.TrimRight(layer.GetText(), " ")
                        last := strings.LastIndex(text, " ")
                        if last == -1 {
                            last = 0
                        } else {
                            last += 1
                        }
                        text = text[0:last]
                        layer.SetText(text)
                        select {
                            case renderNow <-true:
                            default:
                        }
                    case ClearLineMessage:
                        layer.SetText("")
                        select {
                            case renderNow <-true:
                            default:
                        }
                    case BackspaceMessage:
                        text := layer.GetText()
                        if len(text) > 0 {
                            text = text[0:len(text)-1]
                        }
                        layer.SetText(text)
                        select {
                            case renderNow <-true:
                            default:
                        }
                    case EnterMessage:
                        text := layer.GetText()

                        args := strings.Split(text, " ")

                        switch strings.ToLower(strings.TrimSpace(firstString(args))) {
                            case "exit", "quit":
                                mainCancel()
                            case "clear":
                                layer.ClearLines()
                            case "sup":
                                layer.AddLine("nm, u?")
                            case "debug", "debugger":
                                select {
                                    case nesActions <- &NesActionDebugger{}:
                                        layer.AddLine("Opening the debug window..")
                                    default:
                                        layer.AddLine("Error: input dropped. Try again")
                                }
                            case "break":
                                debugger := console.GetDebugger(emulatorActions)
                                if debugger != nil {
                                    if len(args) > 1 {
                                        if strings.ToLower(args[1]) == "list" {
                                            layer.AddLine("Breakpoints")
                                            breakpoints := debugger.GetBreakpoints()
                                            for _, breakpoint := range breakpoints {
                                                layer.AddLine(fmt.Sprintf(" %v: enabled=%v 0x%x", breakpoint.Id, breakpoint.Enabled, breakpoint.PC))
                                            }
                                        } else {
                                            pc, err := strconv.ParseInt(args[1], 0, 32)
                                            if err != nil {
                                                layer.AddLine(fmt.Sprintf("Invalid address '%v': %v", args[1], err))
                                            } else {
                                                breakpoint := debugger.AddPCBreakpoint(uint16(pc))
                                                layer.AddLine(fmt.Sprintf("Breakpoint %v added at 0x%x", breakpoint.Id, breakpoint.PC))
                                            }
                                        }
                                    } else {
                                        breakpoint := debugger.AddCurrentPCBreakpoint()
                                        layer.AddLine(fmt.Sprintf("Breakpoint %v added at 0x%x", breakpoint.Id, breakpoint.PC))
                                    }
                                } else {
                                    layer.AddLine("No debugger available")
                                }
                            case "step":
                                debugger := console.GetDebugger(emulatorActions)
                                if debugger != nil {
                                    debugger.Step(1)
                                    layer.AddLine("Step")
                                } else {
                                    layer.AddLine("No debugger available")
                                }
                            case "delete":
                                if len(args) == 2 {
                                    id, err := strconv.Atoi(args[1])
                                    if err != nil {
                                        layer.AddLine(fmt.Sprintf("Bad breakpoint '%v'", args[1]))
                                    } else {
                                        debugger := console.GetDebugger(emulatorActions)
                                        if debugger != nil {
                                            debugger.RemoveBreakpoint(uint64(id))
                                            layer.AddLine(fmt.Sprintf("Removed breakpoint %v", id))
                                        } else {
                                            layer.AddLine("No debugger available")
                                        }
                                    }
                                } else {
                                    layer.AddLine("Give a breakpoint id to delete")
                                }
                            case "continue":
                                debugger := console.GetDebugger(emulatorActions)
                                if debugger != nil {
                                    debugger.Continue()
                                    layer.AddLine("Continue")
                                } else {
                                    layer.AddLine("No debugger available")
                                }
                            case "help", "?":
                                help := strings.Split(helpText, "\n")
                                for _, line := range help {
                                    if line != "" {
                                        layer.AddLine(line)
                                    }
                                }
                            case "reload", "restart":
                                layer.AddLine("reload")
                                select {
                                    case nesActions <- &NesActionRestart{}:
                                        layer.AddLine("Reloading..")
                                    default:
                                        layer.AddLine("Error: input dropped. Try again")
                                }
                            case "info":
                                layer.AddLine("info")
                                data := make(chan common.EmulatorInfo)
                                response := common.EmulatorActionGetInfo{
                                    Response: data,
                                }
                                select {
                                    case emulatorActions<-response:
                                        ok := false
                                        for info := range data {
                                            ok = true
                                            layer.AddLine("Emulator Info")
                                            layer.AddLine(fmt.Sprintf("Cycles: %v", info.Cycles))
                                            layer.AddLine(fmt.Sprintf("PC: 0x%x", info.Pc))
                                            break
                                        }
                                        if !ok {
                                            layer.AddLine("No ROM loaded")
                                        }
                                    default:
                                }
                            default:
                                if strings.TrimSpace(text) != "" {
                                    layer.AddLine(text)
                                }
                        }

                        layer.SetText("")
                        select {
                            case renderNow <-true:
                            default:
                        }
                    case IsActiveMessage:
                        isActive := message.(IsActiveMessage)
                        isActive.Response <- console.State == StateOpen || console.State == StateOpening
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
                            ticker.Reset(slowTime)
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
                            ticker.Reset(slowTime)
                        }
                }
        }
    }
}

func (console *Console) HandleText(event sdl.Event){
    switch event.GetType() {
        case sdl.TEXTINPUT:
            input := event.(*sdl.TextInputEvent)
            message := TextInputMessage{
                Text: input.GetText(),
            }

            select {
                case console.Messages<- message:
                default:
            }
        case sdl.TEXTEDITING:
            log.Printf("Text editing")
    }
}

/*
func (console *Console) HandleKey(event *sdl.KeyboardEvent, emulatorKeys common.EmulatorKeys){
    switch event.Keysym.Sym {
        case emulatorKeys.Console:
            console.Toggle()
        case sdl.K_BACKSPACE:
            select {
                case console.Messages <- BackspaceMessage{}:
                default:
            }
        case sdl.K_w:
            if (event.Keysym.Mod & sdl.KMOD_LCTRL) == sdl.KMOD_LCTRL {
                select {
                    case console.Messages <- RemoveWordMessage{}:
                    default:
                }
            }
        case sdl.K_u:
            if (event.Keysym.Mod & sdl.KMOD_LCTRL) == sdl.KMOD_LCTRL {
                select {
                    case console.Messages <- ClearLineMessage{}:
                    default:
                }
            }
        case sdl.K_RETURN:
            select {
                case console.Messages <- EnterMessage{}:
                default:
            }
    }
}
*/

func (console *Console) Toggle(){
    log.Printf("toggle console")
    select {
        case console.Messages <- ToggleMessage{}:
        default:
    }
}

func (console *Console) IsActive() bool {
    check := IsActiveMessage{
        Response: make(chan bool),
    }
    console.Messages <- check
    return <-check.Response
}

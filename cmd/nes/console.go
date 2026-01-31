package main

import (
    "context"
    "strings"
    "fmt"
    // "log"
    "sync"
    "image/color"
    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/kazzmir/nes/cmd/nes/debug"

    "github.com/hajimehoshi/ebiten/v2"
    // "github.com/hajimehoshi/ebiten/v2/inpututil"
    "github.com/hajimehoshi/ebiten/v2/vector"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
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
    State ConsoleState
    Lines []string
    Current string
    Size int
    Lock sync.Mutex
}

func MakeConsole(cancel context.CancelFunc, quit context.Context, emulatorActions chan<- common.EmulatorAction, nesActions chan NesAction) *Console {
    console := Console{
        State: StateClosed,
        Size: 0,
    }

    return &console
}

const helpText string = `
help, ?: this help text
exit, quit: quit the program
clear: clear console text
info: show emulator info
reload, restart: reload the current rom
debug: open debug window
`

func (console *Console) AddLine(line string) {
    console.Lock.Lock()
    defer console.Lock.Unlock()
    console.Lines = append(console.Lines, line)
}

func (console *Console) ClearLines() {
    console.Lock.Lock()
    defer console.Lock.Unlock()
    console.Lines = nil
}

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

func (console *Console) Render(screen *ebiten.Image, font text.Face) {
    if console.Size == 0 {
        return
    }

    height := float32(console.Size * 22)

    vector.FillRect(screen, 0, 0, float32(screen.Bounds().Dx()), height, color.NRGBA{R: 255, G: 0, B: 0, A: 200}, false)
    vector.StrokeLine(screen, 0, height, float32(screen.Bounds().Dx()), height, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 200}, false)

    _, fontHeight := text.Measure("A", font, 1)

    yPos := height - float32(fontHeight) - 1
    var textOptions text.DrawOptions
    textOptions.GeoM.Translate(1, float64(yPos))
    text.Draw(screen, "> " + console.Current + "|", font, &textOptions)

    textOptions.ColorScale.ScaleWithColor(color.NRGBA{R: 200, G: 200, B: 200, A: 255})

    console.Lock.Lock()
    defer console.Lock.Unlock()

    for i := len(console.Lines) - 1; i >= 0; i-- {
        line := console.Lines[i]
        textOptions.GeoM.Translate(0, -float64(fontHeight) - 1)
        if yPos >= 0 {
            text.Draw(screen, line, font, &textOptions)
        }
    }
}

func (console *Console) Update(mainCancel context.CancelFunc, emulatorActions chan<- common.EmulatorAction, nesActions chan NesAction, pressedKeys []ebiten.Key, toggleKey ebiten.Key) {
    maxSize := 10

    if console.State == StateOpen || console.State == StateOpening {
        add_chars := true
        control := ebiten.IsKeyPressed(ebiten.KeyControlLeft)
        key_w := false
        key_u := false

        for _, key := range pressedKeys {
            switch key {
                case ebiten.KeyBackspace:
                    if len(console.Current) > 0 {
                        console.Current = console.Current[0:len(console.Current)-1]
                    }
                case ebiten.KeyW:
                    key_w = true
                case ebiten.KeyU:
                    key_u = true
                case toggleKey:
                    console.Toggle()
                case ebiten.KeyEnter:
                    if len(console.Current) > 0 {

                        console.AddLine(console.Current)
                        last := console.Current
                        console.Current = ""

                        parts := strings.Fields(last)
                        switch strings.ToLower(parts[0]) {
                            case "exit", "quit":
                                mainCancel()
                            case "clear":
                                console.ClearLines()
                            case "sup":
                                console.AddLine("nm, u?")
                            case "reload", "restart":
                                select {
                                    case nesActions <- &NesActionRestart{}:
                                        console.AddLine("Reloading..")
                                    default:
                                        console.AddLine("Error: input dropped. Try again")
                                }
                            case "info":
                                data := make(chan common.EmulatorInfo, 1)
                                getInfo := common.EmulatorActionGetInfo{
                                    Response: data,
                                }
                                select {
                                    case emulatorActions<-getInfo:
                                        go func() {
                                            ok := false
                                            for info := range data {
                                                ok = true
                                                console.AddLine("Emulator Info")
                                                console.AddLine(fmt.Sprintf("Cycles: %v", info.Cycles))
                                                console.AddLine(fmt.Sprintf("PC: 0x%x", info.Pc))
                                                break
                                            }
                                            if !ok {
                                                console.AddLine("No ROM loaded")
                                            }
                                        }()
                                    default:
                                }

                                /*
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
                        
                        }

                        */
                    }
                }
            }
        }

        if control && key_w {
            // Remove last word
            text := strings.TrimRight(console.Current, " ")
            last := strings.LastIndex(text, " ")
            if last == -1 {
                last = 0
            } else {
                last += 1
            }
            console.Current = text[0:last]
            add_chars = false
        }

        if control && key_u {
            // Clear line
            console.Current = ""
        }

        if add_chars {
            typed := ebiten.AppendInputChars(nil)
            console.Current += string(typed)
        }
    }

    select {
            /*
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
                case TextInputMessage:
                    text := message.(TextInputMessage)
                    newText := layer.GetText() + text.Text
                    if len(newText) > 1024 {
                        newText = newText[0:1024]
                    }
                    layer.SetText(newText)
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
            */
        default:

    }

    switch console.State {
        case StateOpening:
            if console.Size < maxSize {
                console.Size += 1
            } else {
                console.State = StateOpen
            }
        case StateClosing:
            if console.Size > 0 {
                console.Size -= 1
            }
    }
}

/*
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
*/

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
    switch console.State {
        case StateOpen, StateOpening:
            console.State = StateClosing
        case StateClosing, StateClosed:
            console.State = StateOpening
    }
}

func (console *Console) IsActive() bool {
    return console.State == StateOpen || console.State == StateOpening
}

package debug

import (
    "sync"
    "context"
    "fmt"
    "time"
    "log"
    "strings"
    "strconv"
    nes "github.com/kazzmir/nes/lib"
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

type WindowRequestRaise struct {
}

type WindowRequestBackspace struct {
}

type DebuggerTextBackspace struct {
}

type DebuggerTextAdd struct {
    Text string
}

type DebuggerTextRemoveWord struct {
}

type DebuggerTextClearLine struct {
}

type DebuggerTextEnter struct {
}

type DebuggerTabComplete struct {
}

type Line struct {
    Text string
}

type Instruction struct {
    PC uint16
    Instruction nes.Instruction
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
    Instructions []Instruction
    Lock sync.Mutex
    Debugger Debugger
    Cycle uint64
    Registers Registers
    LastCommand string
}

func MakeDebugWindow(mainQuit context.Context, bigFont *ttf.Font, smallFont *ttf.Font) *DebugWindow {
    quit, cancel := context.WithCancel(mainQuit)
    debug := DebugWindow{
        Quit: quit,
        Cancel: cancel,
        Requests: make(chan WindowRequest, 5),
        IsOpen: false,
        BigFont: bigFont,
        SmallFont: smallFont,
        Line: Line{},
    }

    return &debug
}

func removeLastWord(line string) string {
    text := strings.TrimRight(line, " ")
    last := strings.LastIndex(text, " ")
    if last == -1 {
        last = 0
    } else {
        last += 1
    }
    text = text[0:last]
    return text
}

func (debug *DebugWindow) SetCycle(cycle uint64){
    debug.Lock.Lock()
    debug.Cycle = cycle
    debug.Lock.Unlock()
}

func (debug *DebugWindow) SetRegisters(registers Registers){
    debug.Lock.Lock()
    debug.Registers = registers
    debug.Lock.Unlock()
}

func (debug *DebugWindow) AddInstruction(pc uint16, instruction nes.Instruction){
    debug.Lock.Lock()
    if len(debug.Instructions) > 0 {
        last := debug.Instructions[len(debug.Instructions) - 1]
        if last.PC == pc && last.Instruction.Equals(instruction) {
            // >:E
            debug.Lock.Unlock()
            return
        }
    }
    debug.Instructions = append(debug.Instructions, Instruction{PC: pc, Instruction: instruction})
    if len(debug.Instructions) > 100 {
        debug.Instructions = debug.Instructions[len(debug.Instructions) - 100:len(debug.Instructions)]
    }
    debug.Lock.Unlock()

    select {
        case debug.Requests <- WindowRequestRedraw{}:
        default:
    }
}

func (debug *DebugWindow) SetDebugger(debugger Debugger){
    debug.Debugger = debugger
}

func (debug *DebugWindow) doOpen(quit context.Context, cancel context.CancelFunc) error {
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

    yellow := sdl.Color{
        R: 255, G: 255, B: 0, A: 0,
    }

    red := sdl.Color{
        R: 255,
        G: 0,
        B: 0,
        A: 0,
    }

    render := func(width int, height int, renderer *sdl.Renderer){
        renderer.SetDrawColor(0, 0, 32, 0)
        renderer.Clear()

        y := 1
        gfx.WriteFont(debug.BigFont, renderer, 1, y, "Debugger", white)
        y += debug.BigFont.Height() + 1

        instructionY := y

        consoleHeight := height - debug.SmallFont.Height() - 2

        y = consoleHeight - debug.SmallFont.Height() - 1

        debug.Lock.Lock()
        instructions := gfx.CopyArray(debug.Instructions)
        cycle := debug.Cycle
        registers := debug.Registers
        debug.Lock.Unlock()

        cycleText := fmt.Sprintf("Cycle %v", cycle)

        infoWidth := gfx.TextWidth(debug.SmallFont, cycleText) + 10
        infoX := width - infoWidth - 5
        gfx.Box1(infoX, instructionY, infoWidth, debug.SmallFont.Height() * 8, renderer, func(coords gfx.Coordinates){
            // gfx.WriteFont(debug.SmallFont, renderer, width - gfx.TextWidth(debug.SmallFont, cycleText) - 1, 1, cycleText, white)

            y := coords.Y(1)
            gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, cycleText, white)
            y += debug.SmallFont.Height()
            gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, fmt.Sprintf("A: 0x%X", registers.A), white)
            y += debug.SmallFont.Height()
            gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, fmt.Sprintf("X: 0x%X", registers.X), white)
            y += debug.SmallFont.Height()
            gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, fmt.Sprintf("Y: 0x%X", registers.Y), white)
            y += debug.SmallFont.Height()
            gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, fmt.Sprintf("SP: 0x%X", registers.SP), white)
            y += debug.SmallFont.Height()
            gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, fmt.Sprintf("PC: 0x%X", registers.PC), white)
            y += debug.SmallFont.Height()
            gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, fmt.Sprintf("Status: 0x%X", registers.Status), white)
        })

        breakpoints := debug.Debugger.GetBreakpoints()

        instructionBoxHeight := height - instructionY - 50
        gfx.Box1(4, instructionY, width / 2, instructionBoxHeight, renderer, func(coords gfx.Coordinates){
            y := coords.MaxY()
            for i := len(instructions)-1; i >= 0; i -= 1 {
                color := white

                if instructions[i].PC == registers.PC {
                    color = yellow
                }

                for _, breakpoint := range breakpoints {
                    if breakpoint.PC == instructions[i].PC {
                        color = red
                        break
                    }
                }

                data := fmt.Sprintf("%X: %s", instructions[i].PC, instructions[i].Instruction.String())

                gfx.WriteFont(debug.SmallFont, renderer, coords.X(1), y, data, color)
                y -= debug.SmallFont.Height() + 1
                if y < coords.Y(0) {
                    break
                }
            }
        })

        y += instructionBoxHeight + 1

        renderer.SetDrawColor(255, 255, 255, 255)
        y = consoleHeight
        renderer.DrawLine(0, int32(y), int32(width), int32(y))
        y += 2
        gfx.WriteFont(debug.SmallFont, renderer, 1, y, fmt.Sprintf("> %v|", debug.Line.Text), white)

        renderer.Present()
    }
    
    redraw := make(chan bool, 2)
    redraw <- true
    defer close(redraw)

    /* FIXME: make this atomic */
    debug.IsOpen = true
    defer func(){
        debug.IsOpen = false
    }()

    /* Listen for redraw events */
    go func(){
        for {
            select {
                case <-quit.Done():
                    return
                case <-redraw:
                    windowWidth, windowHeight := window.GetSize()
                    sdl.Do(func(){
                        render(int(windowWidth), int(windowHeight), renderer)
                    })
                    time.Sleep(20 * time.Millisecond)
            }
        }
    }()

    doRedraw := func(){
        select {
            case redraw <- true:
            default:
        }
    }

    handleRequest := func(request any){
        switch request.(type) {
            case WindowRequestWindow:
                windowRequest := request.(WindowRequestWindow)
                windowRequest.Response <- window
            case WindowRequestRaise:
                go func(){
                    sdl.Do(func(){
                        window.Raise()
                    })
                    doRedraw()
                }()
            case WindowRequestRedraw:
                doRedraw()
            case DebuggerTextAdd:
                input := request.(DebuggerTextAdd)
                debug.Line.Text += input.Text
                doRedraw()
            case DebuggerTextBackspace:
                if len(debug.Line.Text) > 0 {
                    debug.Line.Text = debug.Line.Text[0:len(debug.Line.Text)-1]
                }
                doRedraw()
            case DebuggerTextRemoveWord:
                debug.Line.Text = removeLastWord(debug.Line.Text)
                doRedraw()
            case DebuggerTextClearLine:
                debug.Line.Text = ""
                doRedraw()
            case DebuggerTextEnter:
                if strings.TrimSpace(debug.Line.Text) != "" {
                    debug.LastCommand = strings.TrimSpace(debug.Line.Text)
                }

                parts := strings.Fields(debug.LastCommand)

                if len(parts) > 0 {
                    switch parts[0] {
                        case "q", "quit":
                            cancel()
                        case "s", "step":
                            count := 1
                            if len(parts) > 1 {
                                v, err := strconv.Atoi(parts[1])
                                if err == nil {
                                    count = v
                                }
                            }
                            debug.Debugger.Step(count)
                        case "n", "next":
                            debug.Debugger.Next()
                        case "c", "continue":
                            debug.Debugger.Continue()
                    }
                }

                debug.Line.Text = ""

                doRedraw()
            case DebuggerTabComplete:
                completions := []string{"quit", "step", "next", "continue"}
                for _, possible := range completions {
                    if strings.HasPrefix(possible, debug.Line.Text) {
                        debug.Line.Text = possible
                        doRedraw()
                        break
                    }
                }
            default:
                log.Printf("Unhandled debugger message: %+v", request)
        }
    }

    /* Do not make any sdl.Do() calls in this for loop. If sdl.Do is needed then wrap it in
     * another go func, e.g.:
     *  go func(){ sdl.Do(...) }
     */
    for {
        select {
            case <-quit.Done():
                return nil
            case request := <-debug.Requests:
                handleRequest(request)
        }
    }

    // return nil
}

func (debug *DebugWindow) IsWindow(windowId uint32) bool {
    if !debug.IsOpen {
        return false
    }

    request := WindowRequestWindow{
        Response: make(chan *sdl.Window),
    }
    debug.Requests <- request
    out := <-request.Response
    if out != nil {
        id, err := out.GetID()
        if err == nil {
            return id == windowId
        }
    }
    return false
}

func (debug *DebugWindow) Open(mainQuit context.Context){
    debug.opener.Do(func(){
        debug.Wait.Add(1)
        go func(){
            defer debug.Wait.Done()
            err := debug.doOpen(debug.Quit, debug.Cancel)
            if err != nil {
                log.Printf("Could not open debug window: %v", err)
            }

            quit, cancel := context.WithCancel(mainQuit)
            debug.Quit = quit
            debug.Cancel = cancel
            debug.opener = sync.Once{}
        }()
    })

    debug.Requests <- WindowRequestRaise{}
}

func (debug *DebugWindow) Close() {
    debug.Cancel()
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
            message := DebuggerTextAdd{
                Text: input.GetText(),
            }

            debug.Requests <- message
    }
}

func hasLeftControlKey(event *sdl.KeyboardEvent) bool {
    return (event.Keysym.Mod & sdl.KMOD_LCTRL) == sdl.KMOD_LCTRL
}

func (debug *DebugWindow) HandleKey(event sdl.Event){
    if !debug.IsOpen {
        return
    }

    switch event.GetType() {
        case sdl.KEYDOWN:
            key_event := event.(*sdl.KeyboardEvent)
            switch key_event.Keysym.Sym {
                case sdl.K_BACKSPACE:
                    select {
                        case debug.Requests <- DebuggerTextBackspace{}:
                        default:
                    }
                case sdl.K_w:
                    if hasLeftControlKey(key_event) {
                        select {
                            case debug.Requests <- DebuggerTextRemoveWord{}:
                            default:
                        }
                    }
                case sdl.K_u:
                    if hasLeftControlKey(key_event) {
                        select {
                            case debug.Requests <- DebuggerTextClearLine{}:
                            default:
                        }
                    }
                case sdl.K_RETURN:
                    select {
                        case debug.Requests <- DebuggerTextEnter{}:
                        default:
                    }
                case sdl.K_TAB:
                    select {
                        case debug.Requests <- DebuggerTabComplete{}:
                        default:
                    }
            }
        case sdl.KEYUP:
    }
}

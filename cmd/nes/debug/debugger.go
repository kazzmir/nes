package debug

import (
    "log"
    "sync"
    "github.com/kazzmir/nes/cmd/nes/gfx"
    nes "github.com/kazzmir/nes/lib"
)

type DebugCommand interface {
    Name() string
}

type DebugCommandSimple struct {
    name string
}

func (command *DebugCommandSimple) Name() string {
    return command.name
}

type DebugCommandStep struct {
    name string
    count int
}

func (command *DebugCommandStep) Name() string {
    return command.name
}

func makeCommand(name string) DebugCommand {
    return &DebugCommandSimple{name: name}
}

var DebugCommandContinue DebugCommand = makeCommand("continue")
var DebugCommandNext DebugCommand = makeCommand("next")

// break when the cpu's PC is at a specific value
// TODO: add conditional breakpoints, and break upon
// reading/writing specific memory addresses
type Breakpoint struct {
    PC uint16
    Id uint64
    Enabled bool
}

func (breakpoint *Breakpoint) Hit(cpu *nes.CPUState) bool {
    return breakpoint.Enabled && breakpoint.PC == cpu.PC
}

type Debugger interface {
    Handle(*nes.CPUState) bool
    AddPCBreakpoint(uint16) Breakpoint
    AddCurrentPCBreakpoint() Breakpoint
    RemoveBreakpoint(id uint64) bool
    GetBreakpoints() []Breakpoint
    Continue()
    Step(count int)
    Next()
    IsStopped() bool
    Update(*nes.CPUState, nes.InstructionTable)
    Close()
}

type DebuggerMode int

const (
    ModeStopped DebuggerMode = iota
    ModeStepping
    ModeNext
    ModeContinue
)

type DefaultDebugger struct {
    Commands chan DebugCommand
    Mode DebuggerMode
    Breakpoints []Breakpoint
    BreakpointId uint64
    Cpu *nes.CPUState
    Window *DebugWindow
    Lock sync.Mutex
    LastPc uint16
    StepCount int
}

type Registers struct {
    A byte
    X byte
    Y byte
    SP byte
    PC uint16
    Status byte
}

func (debugger *DefaultDebugger) Update(cpu *nes.CPUState, table nes.InstructionTable){
    if debugger.Window != nil && debugger.Window.IsOpen {
        /* Warning: fetch calls cpu.LoadMemory that could in theory impact mapper's
         * if the PC is pointing into mapper memory.
         */
        instruction, err := cpu.Fetch(table)
        if err == nil {
            pc := cpu.PC
            debugger.Window.AddInstruction(pc, instruction)
            debugger.Window.SetCycle(cpu.Cycle)
            debugger.Window.SetRegisters(Registers{
                A: cpu.A,
                X: cpu.X,
                Y: cpu.Y,
                SP: cpu.SP,
                PC: cpu.PC,
                Status: cpu.Status,
            })

            debugger.Window.Redraw()
        }
    }
}

func (debugger *DefaultDebugger) IsStopped() bool {
    return debugger.Mode == ModeStopped || debugger.Mode == ModeStepping
}

func (debugger *DefaultDebugger) ContinueUntilBreak(){
    debugger.Mode = ModeContinue
}

func (debugger *DefaultDebugger) GetBreakpoints() []Breakpoint {
    debugger.Lock.Lock()
    defer debugger.Lock.Unlock()
    return gfx.CopyArray(debugger.Breakpoints)
}

func (debugger *DefaultDebugger) AddPCBreakpoint(pc uint16) Breakpoint {
    var breakpoint Breakpoint
    debugger.WithLock(func(){
        breakpoint = Breakpoint{
            PC: pc,
            Id: debugger.BreakpointId,
            Enabled: true,
        }
        debugger.Breakpoints = append(debugger.Breakpoints, breakpoint)
        debugger.BreakpointId += 1
    })
    return breakpoint
}

func (debugger *DefaultDebugger) AddCurrentPCBreakpoint() Breakpoint {
    return debugger.AddPCBreakpoint(debugger.Cpu.PC)
}

func (debugger *DefaultDebugger) WithLock(fn func()){
    debugger.Lock.Lock()
    defer debugger.Lock.Unlock()
    fn()
}

func (debugger *DefaultDebugger) RemoveBreakpoint(id uint64) bool {
    found := false
    debugger.WithLock(func(){
        var out []Breakpoint
        for _, breakpoint := range debugger.Breakpoints {
            if breakpoint.Id != id {
                out = append(out, breakpoint)
            } else {
                found = true
            }
        }
        debugger.Breakpoints = out
    })
    return found
}

func (debugger *DefaultDebugger) Stop(){
    debugger.Mode = ModeStopped
}

func (debugger *DefaultDebugger) Continue(){
    select {
        case debugger.Commands<-DebugCommandContinue:
        default:
    }
}

func (debugger *DefaultDebugger) Step(count int){
    select {
    case debugger.Commands<-&DebugCommandStep{name: "step", count: count}:
        default:
    }
}

func (debugger *DefaultDebugger) Next(){
    select {
        case debugger.Commands<-DebugCommandNext:
        default:
    }
}

func (debugger *DefaultDebugger) Handle(cpu *nes.CPUState) bool {
    select {
        case command := <-debugger.Commands:
            switch command {
                case DebugCommandContinue:
                    log.Printf("[debug] continue")
                    debugger.ContinueUntilBreak()
                    return true
                case DebugCommandNext:
                    log.Printf("[debug] next")
                    debugger.Mode = ModeNext
                    debugger.LastPc = cpu.PC
                default:
                    switch command.(type) {
                        case *DebugCommandStep:
                            log.Printf("[debug] step")
                            debugger.Mode = ModeStepping
                            step := command.(*DebugCommandStep)
                            debugger.StepCount = step.count
                            return true
                    }
            }
        default:
    }

    if debugger.Mode == ModeNext {
        if cpu.PC == debugger.LastPc {
            return true
        } else {
            debugger.Mode = ModeStopped
            return false
        }
    }

    if debugger.Mode == ModeStepping {
        debugger.StepCount -= 1
        return debugger.StepCount > 0
    }

    if debugger.Mode == ModeStopped {
        return false
    }

    for _, breakpoint := range debugger.Breakpoints {
        if breakpoint.Hit(cpu) {
            debugger.Stop()
            return false
        }
    }

    return true
}

func (debugger *DefaultDebugger) Close(){
    if debugger.Window != nil {
        debugger.Window.SetDebugger(nil)
    }
}

func MakeDebugger(cpu *nes.CPUState, window *DebugWindow) Debugger {
    debugger := &DefaultDebugger{
        Commands: make(chan DebugCommand, 5),
        Mode: ModeContinue,
        BreakpointId: 1,
        Cpu: cpu,
        Window: window,
    }

    window.SetDebugger(debugger)
    return debugger
}

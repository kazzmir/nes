package debug

import (
    "log"
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

func makeCommand(name string) DebugCommand {
    return &DebugCommandSimple{name: name}
}

var DebugCommandStep DebugCommand = makeCommand("step")
var DebugCommandContinue DebugCommand = makeCommand("continue")

// break when the cpu's PC is at a specific value
// TODO: add conditional breakpoints, and break upon
// reading/writing specific memory addresses
type Breakpoint struct {
    PC uint16
    Id uint64
}

func (breakpoint *Breakpoint) Hit(cpu *nes.CPUState) bool {
    return breakpoint.PC == cpu.PC
}

type Debugger interface {
    Handle(*nes.CPUState) bool
    AddPCBreakpoint(uint16) Breakpoint
    AddCurrentPCBreakpoint() Breakpoint
    RemoveBreakpoint(id uint64) bool
    Continue()
    Step()
    IsStopped() bool
}

type DebuggerMode int

const (
    ModeStopped DebuggerMode = iota
    ModeStepping
    ModeContinue
)

type DefaultDebugger struct {
    Commands chan DebugCommand
    Mode DebuggerMode
    Breakpoints []Breakpoint
    BreakpointId uint64
    Cpu *nes.CPUState
}

func (debugger *DefaultDebugger) IsStopped() bool {
    return debugger.Mode == ModeStopped || debugger.Mode == ModeStepping
}

func (debugger *DefaultDebugger) ContinueUntilBreak(){
    debugger.Mode = ModeContinue
}

func (debugger *DefaultDebugger) AddPCBreakpoint(pc uint16) Breakpoint {
    breakpoint := Breakpoint{
        PC: pc,
        Id: debugger.BreakpointId,
    }
    debugger.Breakpoints = append(debugger.Breakpoints, breakpoint)
    debugger.BreakpointId += 1
    return breakpoint
}

func (debugger *DefaultDebugger) AddCurrentPCBreakpoint() Breakpoint {
    return debugger.AddPCBreakpoint(debugger.Cpu.PC)
}

func (debugger *DefaultDebugger) RemoveBreakpoint(id uint64) bool {
    var out []Breakpoint
    found := false
    for _, breakpoint := range debugger.Breakpoints {
        if breakpoint.Id != id {
            out = append(out, breakpoint)
        } else {
            found = true
        }
    }
    debugger.Breakpoints = out
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

func (debugger *DefaultDebugger) Step(){
    select {
        case debugger.Commands<-DebugCommandStep:
        default:
    }
}

func (debugger *DefaultDebugger) Handle(cpu *nes.CPUState) bool {
    select {
        case command := <-debugger.Commands:
            switch command {
                case DebugCommandStep:
                    log.Printf("[debug] step")
                    debugger.Mode = ModeStepping
                    return true
                case DebugCommandContinue:
                    log.Printf("[debug] continue")
                    debugger.ContinueUntilBreak()
                    return true
            }
        default:
    }

    if debugger.Mode == ModeStepping {
        return false
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

func MakeDebugger(cpu *nes.CPUState) Debugger {
    return &DefaultDebugger{
        Commands: make(chan DebugCommand, 5),
        Mode: ModeContinue,
        BreakpointId: 1,
        Cpu: cpu,
    }
}

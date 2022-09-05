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
    Handle(*nes.CPUState)
    AddPCBreakpoint(uint16) Breakpoint
    AddCurrentPCBreakpoint() Breakpoint
    Continue()
    IsStopped() bool
}

type DefaultDebugger struct {
    Commands chan DebugCommand
    Stopped bool
    Breakpoints []Breakpoint
    BreakpointId uint64
    Cpu *nes.CPUState
}

func (debugger *DefaultDebugger) IsStopped() bool {
    return debugger.Stopped
}

func (debugger *DefaultDebugger) ContinueUntilBreak(){
    debugger.Stopped = false
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

func (debugger *DefaultDebugger) RemoveBreakpoint(id uint64){
    var out []Breakpoint
    for _, breakpoint := range debugger.Breakpoints {
        if breakpoint.Id != id {
            out = append(out, breakpoint)
        }
    }
    debugger.Breakpoints = out
}

func (debugger *DefaultDebugger) Stop(){
    debugger.Stopped = true
}

func (debugger *DefaultDebugger) Continue(){
    select {
        case debugger.Commands<-DebugCommandContinue:
        default:
    }
}

func (debugger *DefaultDebugger) Handle(cpu *nes.CPUState){
    select {
        case command := <-debugger.Commands:
            switch command {
                case DebugCommandStep:
                    log.Printf("[debug] step")
                    return
                case DebugCommandContinue:
                    log.Printf("[debug] continue")
                    debugger.ContinueUntilBreak()
                    return
            }
        default:
    }

    for _, breakpoint := range debugger.Breakpoints {
        if breakpoint.Hit(cpu) {
            debugger.Stop()
        }
    }
}

func MakeDebugger(cpu *nes.CPUState) Debugger {
    return &DefaultDebugger{
        Commands: make(chan DebugCommand, 5),
        Stopped: false,
        BreakpointId: 1,
        Cpu: cpu,
    }
}

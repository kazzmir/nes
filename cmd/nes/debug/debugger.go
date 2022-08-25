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
}

type DefaultDebugger struct {
    Commands chan DebugCommand
    Stopped bool
    Breakpoints []Breakpoint
    BreakpointId uint64
}

func (debugger *DefaultDebugger) IsStopped() bool {
    return debugger.Stopped
}

func (debugger *DefaultDebugger) ContinueUntilBreak(){
    debugger.Stopped = false
}

func (debugger *DefaultDebugger) AddPCBreakpoint(pc uint16){
    debugger.Breakpoints = append(debugger.Breakpoints, Breakpoint{
        PC: pc,
        Id: debugger.BreakpointId,
    })
    debugger.BreakpointId += 1
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

func (debugger *DefaultDebugger) Handle(cpu *nes.CPUState){
    if debugger.IsStopped() {
        command := <-debugger.Commands
        if command == DebugCommandStep {
            log.Printf("[debug] step")
            return
        }
        if command == DebugCommandContinue {
            log.Printf("[debug] continue")
            debugger.ContinueUntilBreak()
            return
        }
    }

    for _, breakpoint := range debugger.Breakpoints {
        if breakpoint.Hit(cpu) {
            debugger.Stop()
        }
    }
}

func MakeDebugger() Debugger {
    return &DefaultDebugger{
        Commands: make(chan DebugCommand, 5),
        Stopped: true,
        BreakpointId: 1,
    }
}

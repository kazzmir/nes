package common

import (
    "github.com/veandco/go-sdl2/sdl"
)

type RenderFunction func(*sdl.Renderer) error

type WindowSize struct {
    X int
    Y int
}

type ProgramActions int
const (
    ProgramToggleSound = iota
    ProgramQuit
    ProgramPauseEmulator
    ProgramUnpauseEmulator
)

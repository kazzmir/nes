package main

import (
    nes "github.com/kazzmir/nes/lib"
    "github.com/veandco/go-sdl2/sdl"
    "fmt"
)

type SDLButtons struct {
}

func (buttons *SDLButtons) Get() nes.ButtonMapping {
    mapping := make(nes.ButtonMapping)

    keyboard := sdl.GetKeyboardState()
    mapping[nes.ButtonIndexA] = keyboard[sdl.SCANCODE_A] == 1
    mapping[nes.ButtonIndexB] = keyboard[sdl.SCANCODE_S] == 1
    mapping[nes.ButtonIndexSelect] = keyboard[sdl.SCANCODE_Q] == 1
    mapping[nes.ButtonIndexStart] = keyboard[sdl.SCANCODE_RETURN] == 1
    mapping[nes.ButtonIndexUp] = keyboard[sdl.SCANCODE_UP] == 1
    mapping[nes.ButtonIndexDown] = keyboard[sdl.SCANCODE_DOWN] == 1
    mapping[nes.ButtonIndexLeft] = keyboard[sdl.SCANCODE_LEFT] == 1
    mapping[nes.ButtonIndexRight] = keyboard[sdl.SCANCODE_RIGHT] == 1

    return mapping
}

type SDLJoystickButtons struct {
    joystick *sdl.Joystick
}

type IControlPad SDLJoystickButtons

func MakeIControlPadInput(index int) (IControlPad, error){
    joystick, err := OpenJoystick(index)
    return IControlPad(joystick), err
}

func (icontrolpad *IControlPad) Close(){
    icontrolpad.joystick.Close()
}

func (icontrolpad *IControlPad) Get() nes.ButtonMapping {
    mapping := make(nes.ButtonMapping)

    mapping[nes.ButtonIndexA] = icontrolpad.joystick.Button(12) == 1
    mapping[nes.ButtonIndexB] = icontrolpad.joystick.Button(13) == 1
    mapping[nes.ButtonIndexSelect] = icontrolpad.joystick.Button(8) == 1
    mapping[nes.ButtonIndexStart] = icontrolpad.joystick.Button(9) == 1
    mapping[nes.ButtonIndexUp] =  icontrolpad.joystick.Button(0) == 1
    mapping[nes.ButtonIndexDown] = icontrolpad.joystick.Button(3) == 1
    mapping[nes.ButtonIndexLeft] = icontrolpad.joystick.Button(2) == 1
    mapping[nes.ButtonIndexRight] =  icontrolpad.joystick.Button(1) == 1

    return mapping
}

func OpenJoystick(index int) (SDLJoystickButtons, error){
    joystick := sdl.JoystickOpen(index)
    if joystick == nil {
        return SDLJoystickButtons{}, fmt.Errorf("Could not open joystick %v", index)
    }

    return SDLJoystickButtons{joystick: joystick}, nil
}

func (joystick *SDLJoystickButtons) Close(){
    joystick.joystick.Close()
}

func (joystick *SDLJoystickButtons) Get() nes.ButtonMapping {
    mapping := make(nes.ButtonMapping)
    return mapping

    /*
    mapping[nes.ButtonIndexA] = joystick.joystick.Button(12) == 1
    mapping[nes.ButtonIndexB] = joystick.joystick.Button(13) == 1
    mapping[nes.ButtonIndexSelect] = joystick.joystick.Button(8) == 1
    mapping[nes.ButtonIndexStart] = joystick.joystick.Button(9) == 1
    mapping[nes.ButtonIndexUp] =  joystick.joystick.Button(0) == 1
    mapping[nes.ButtonIndexDown] = joystick.joystick.Button(3) == 1
    mapping[nes.ButtonIndexLeft] = joystick.joystick.Button(2) == 1
    mapping[nes.ButtonIndexRight] =  joystick.joystick.Button(1) == 1

    return mapping
    */
}

type CombineButtons struct {
    Buttons []nes.HostInput
}

func MakeCombineButtons(input1 nes.HostInput, input2 nes.HostInput) CombineButtons {
    return CombineButtons{
        Buttons: []nes.HostInput{input1, input2},
    }
}

func combineMapping(input1 nes.ButtonMapping, input2 nes.ButtonMapping) nes.ButtonMapping {
    out := make(nes.ButtonMapping)
    for _, button := range nes.AllButtons() {
        out[button] = input1[button] || input2[button]
    }

    return out
}

func (combine *CombineButtons) Get() nes.ButtonMapping {
    var mapping nes.ButtonMapping
    for _, input := range combine.Buttons {
        mapping = combineMapping(mapping, input.Get())
    }

    return mapping
}


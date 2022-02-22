package common

import (
    nes "github.com/kazzmir/nes/lib"
    "github.com/veandco/go-sdl2/sdl"
    "sync"
    "strings"
    "log"
    "fmt"

    // "runtime/debug"
)

type JoystickManager struct {
    Joysticks []*SDLJoystickButtons
    Player1 *SDLJoystickButtons
    Player2 *SDLJoystickButtons
    Lock sync.Mutex
}

func NewJoystickManager() *JoystickManager {
    manager := JoystickManager{
    }

    max := sdl.NumJoysticks()
    for i := 0; i < max; i++ {
        input, err := OpenJoystick(i)
        if err != nil {
            log.Printf("Could not open joystick %v: %v\n", i, err)
        }

        manager.Joysticks = append(manager.Joysticks, &input)
    }

    if len(manager.Joysticks) > 0 {
        manager.Player1 = manager.Joysticks[0]
    }

    return &manager
}

func (manager *JoystickManager) CurrentName() string {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    if manager.Player1 != nil {
        return manager.Player1.Name
    }

    return "No joystick found"
}

func (manager *JoystickManager) AddJoystick(index int) error {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    joystick, err := OpenJoystick(index)
    if err != nil {
        return err
    }

    for _, check := range manager.Joysticks {
        if check.joystick.InstanceID() == joystick.joystick.InstanceID() {
            return fmt.Errorf("Joystick id %v is already added", joystick.joystick.InstanceID())
        }
    }

    manager.Joysticks = append(manager.Joysticks, &joystick)
    if manager.Player1 == nil {
        manager.Player1 = &joystick
    }

    return nil
}

func (manager *JoystickManager) RemoveJoystick(id sdl.JoystickID){
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    var out []*SDLJoystickButtons
    for _, joystick := range manager.Joysticks {
        if joystick.joystick.InstanceID() == id {
            joystick.Close()
            if manager.Player1 == joystick {
                manager.Player1 = nil
            }
        } else {
            out = append(out, joystick)
        }
    }

    manager.Joysticks = out
}

func (manager *JoystickManager) HandleEvent(event sdl.Event) EmulatorAction {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    var out EmulatorAction = EmulatorNothing
    for _, joystick := range manager.Joysticks {
        value := joystick.HandleEvent(event)
        if value != EmulatorNothing {
            out = value
        }
    }

    return out
}

func (manager *JoystickManager) Close() {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    for _, joystick := range manager.Joysticks {
        joystick.Close()
    }
}

func (manager *JoystickManager) Get() nes.ButtonMapping {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    mapping := make(nes.ButtonMapping)

    mapping[nes.ButtonIndexA] = false
    mapping[nes.ButtonIndexB] = false
    mapping[nes.ButtonIndexSelect] = false
    mapping[nes.ButtonIndexStart] = false
    mapping[nes.ButtonIndexUp] = false
    mapping[nes.ButtonIndexDown] = false
    mapping[nes.ButtonIndexLeft] = false
    mapping[nes.ButtonIndexRight] = false

    if manager.Player1 != nil {
        return manager.Player1.Get()
    }

    return mapping
}

type SDLKeyboardButtons struct {
}

func (buttons *SDLKeyboardButtons) Get() nes.ButtonMapping {
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

type JoystickInput interface {
}

type JoystickButton struct {
    Button int
}

type JoystickAxis struct {
    Axis int
    Value int
}

type SDLJoystickButtons struct {
    joystick *sdl.Joystick
    Inputs map[nes.Button]JoystickInput // normal nes buttons
    ExtraInputs map[EmulatorAction]JoystickInput // extra emulator-only buttons
    Pressed nes.ButtonMapping
    Lock sync.Mutex
    Name string
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

    /*
    log.Printf("Joystick guid: %v", joystick.GUID())
    log.Printf(string(debug.Stack()))
    */

    return SDLJoystickButtons{
        joystick: joystick,
        Inputs: make(map[nes.Button]JoystickInput),
        ExtraInputs: make(map[EmulatorAction]JoystickInput),
        Pressed: make(nes.ButtonMapping),
        Name: strings.TrimSpace(joystick.Name()),
    }, nil
}

func (joystick *SDLJoystickButtons) HandleEvent(event sdl.Event) EmulatorAction {
    joystick.Lock.Lock()
    defer joystick.Lock.Unlock()

    var emulatorOut EmulatorAction = EmulatorNothing

    rawButton, ok := event.(*sdl.JoyButtonEvent)
    if ok && rawButton.Which == joystick.joystick.InstanceID() {
        for input, button := range joystick.Inputs {
            realButton, ok := button.(*JoystickButton)
            if ok {
                if int(rawButton.Button) == realButton.Button {
                    joystick.Pressed[input] = rawButton.State == sdl.PRESSED
                }
            }
        }

        for extraInput, button := range joystick.ExtraInputs {
            realButton, ok := button.(*JoystickButton)
            if ok {
                if int(rawButton.Button) == realButton.Button {
                    _ = extraInput
                    if rawButton.State == sdl.PRESSED {
                        emulatorOut = EmulatorTurbo
                    } else {
                        emulatorOut = EmulatorNormal
                    }
                }
            }
        }
    }

    rawAxis, ok := event.(*sdl.JoyAxisEvent)
    if ok && rawAxis.Which == joystick.joystick.InstanceID() {
        for input, raw := range joystick.Inputs {
            axis, ok := raw.(*JoystickAxis)
            if ok {
                if axis.Axis == int(rawAxis.Axis) {
                    joystick.Pressed[input] = (axis.Value < 0 && rawAxis.Value < 0) || (axis.Value > 0 && rawAxis.Value > 0)
                }
            }
        }
    }

    return emulatorOut
}

func (joystick *SDLJoystickButtons) Close(){
    joystick.joystick.Close()
}

func (joystick *SDLJoystickButtons) SetButton(button nes.Button, input JoystickInput){
    joystick.Inputs[button] = input
}

func (joystick *SDLJoystickButtons) SetExtraButton(button EmulatorAction, input JoystickInput){
    joystick.ExtraInputs[button] = input
}

func (joystick *SDLJoystickButtons) Get() nes.ButtonMapping {
    joystick.Lock.Lock()
    defer joystick.Lock.Unlock()

    copied := make(nes.ButtonMapping)

    copied[nes.ButtonIndexA] = false
    copied[nes.ButtonIndexB] = false
    copied[nes.ButtonIndexSelect] = false
    copied[nes.ButtonIndexStart] = false
    copied[nes.ButtonIndexUp] = false
    copied[nes.ButtonIndexDown] = false
    copied[nes.ButtonIndexLeft] = false
    copied[nes.ButtonIndexRight] = false

    for k, v := range joystick.Pressed {
        copied[k] = v
    }
    return copied
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


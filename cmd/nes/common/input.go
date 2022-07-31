package common

import (
    nes "github.com/kazzmir/nes/lib"
    "github.com/veandco/go-sdl2/sdl"
    "sync"
    "strings"
    "log"
    "fmt"
    "errors"
    "os"
    "path/filepath"
    "encoding/json"

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

type ConfigJoystickData struct {
    A string
    B string
    Select string
    Start string
    Up string
    Down string
    Left string
    Right string
    Guid string
    Name string
}

type ConfigData struct {
    Version int
    Player1Joystick ConfigJoystickData
}

func GetOrCreateConfigDir() (string, error) {
    configDir, err := os.UserConfigDir()
    if err != nil {
        return "", err
    }
    configPath := filepath.Join(configDir, "jon-nes")
    err = os.MkdirAll(configPath, 0755)
    if err != nil {
        return "", err
    }

    return configPath, nil
}

func (manager *JoystickManager) SaveInput() error {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    configPath, err := GetOrCreateConfigDir()
    if err != nil {
        return err
    }
    config := filepath.Join(configPath, "config.json")

    file, err := os.Create(config)
    if err != nil {
        return err
    }
    defer file.Close()

    if manager.Player1 != nil {
        data := ConfigData{
            Version: 1,
            Player1Joystick: ConfigJoystickData{
                A: manager.Player1.Inputs[nes.ButtonIndexA].Serialize(),
                B: manager.Player1.Inputs[nes.ButtonIndexB].Serialize(),
                Select: manager.Player1.Inputs[nes.ButtonIndexSelect].Serialize(),
                Start: manager.Player1.Inputs[nes.ButtonIndexStart].Serialize(),
                Up: manager.Player1.Inputs[nes.ButtonIndexUp].Serialize(),
                Down: manager.Player1.Inputs[nes.ButtonIndexDown].Serialize(),
                Left: manager.Player1.Inputs[nes.ButtonIndexLeft].Serialize(),
                Right: manager.Player1.Inputs[nes.ButtonIndexRight].Serialize(),
                Guid: sdl.JoystickGetGUIDString(manager.Player1.joystick.GUID()),
                Name: strings.TrimSpace(manager.Player1.joystick.Name()),
            },
        }

        serialized, err := json.Marshal(data)
        if err != nil {
            return err
        }

        file.Write(serialized)
    }

    log.Printf("Saved config to %v", config)

    return nil
}

var JoystickAlreadyAdded = errors.New("Joystick has already been added")

func (manager *JoystickManager) AddJoystick(index int) error {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    joystick, err := OpenJoystick(index)
    if err != nil {
        return err
    }

    for _, check := range manager.Joysticks {
        if check.joystick.InstanceID() == joystick.joystick.InstanceID() {
            return JoystickAlreadyAdded
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

    var out EmulatorAction = MakeEmulatorAction(EmulatorNothing)
    for _, joystick := range manager.Joysticks {
        value := joystick.HandleEvent(event)
        if value.Value() != EmulatorNothing {
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
    Keys EmulatorKeys

    /* true if held down, false if not */
    ButtonA bool
    ButtonB bool
    ButtonTurboA bool
    ButtonTurboB bool
    TurboACounter int
    TurboBCounter int
    ButtonSelect bool
    ButtonStart bool
    ButtonUp bool
    ButtonDown bool
    ButtonLeft bool
    ButtonRight bool
}

func (buttons *SDLKeyboardButtons) Reset(){
    buttons.ButtonA = false
    buttons.ButtonB = false
    buttons.ButtonSelect = false
    buttons.ButtonStart = false
    buttons.ButtonUp = false
    buttons.ButtonDown = false
    buttons.ButtonLeft = false
    buttons.ButtonRight = false
    buttons.ButtonTurboA = false
    buttons.ButtonTurboB = false
    buttons.TurboACounter = 0
    buttons.TurboBCounter = 0
}

func (buttons *SDLKeyboardButtons) Get() nes.ButtonMapping {
    mapping := make(nes.ButtonMapping)

    if buttons.ButtonTurboA {
        buttons.TurboACounter += 1
        if buttons.TurboACounter > 5 {
            buttons.ButtonA = !buttons.ButtonA
            buttons.TurboACounter = 0
        }
    }

    if buttons.ButtonTurboB {
        buttons.TurboBCounter += 1
        if buttons.TurboBCounter > 5 {
            buttons.ButtonB = !buttons.ButtonB
            buttons.TurboBCounter = 0
        }
    }

    mapping[nes.ButtonIndexA] = buttons.ButtonA
    mapping[nes.ButtonIndexB] = buttons.ButtonB
    mapping[nes.ButtonIndexSelect] = buttons.ButtonSelect
    mapping[nes.ButtonIndexStart] = buttons.ButtonStart
    mapping[nes.ButtonIndexUp] = buttons.ButtonUp
    mapping[nes.ButtonIndexDown] = buttons.ButtonDown
    mapping[nes.ButtonIndexLeft] = buttons.ButtonLeft
    mapping[nes.ButtonIndexRight] = buttons.ButtonRight

    return mapping
}

func (buttons *SDLKeyboardButtons) HandleEvent(event *sdl.KeyboardEvent){
    set := false
    switch event.GetType() {
        case sdl.KEYDOWN: set = true
        case sdl.KEYUP: set = false
        default:
            /* what is this? */
            return
    }

    switch event.Keysym.Sym {
        case buttons.Keys.ButtonA: buttons.ButtonA = set
        case buttons.Keys.ButtonB: buttons.ButtonB = set
        case buttons.Keys.ButtonTurboA:
            buttons.ButtonTurboA = set
            /* if the user releases the turbo button the A/B button might be in
             * a pressed state even though the user is not currently pressing it,
             * so ensure that A/B is not pressed if turbo is released
             */
            if !set {
                buttons.ButtonA = false
            }
        case buttons.Keys.ButtonTurboB:
            buttons.ButtonTurboB = set
            if !set {
                buttons.ButtonB = false
            }
        case buttons.Keys.ButtonSelect: buttons.ButtonSelect = set
        case buttons.Keys.ButtonStart: buttons.ButtonStart = set
        case buttons.Keys.ButtonUp: buttons.ButtonUp = set
        case buttons.Keys.ButtonDown: buttons.ButtonDown = set
        case buttons.Keys.ButtonLeft: buttons.ButtonLeft = set
        case buttons.Keys.ButtonRight: buttons.ButtonRight = set
    }
}

type JoystickInput interface {
    /* FIXME: return a mapping suitable for json */
    Serialize() string
}

type JoystickButton struct {
    Button int
}

func (button *JoystickButton) Serialize() string {
    return fmt.Sprintf("%v", button.Button)
}

type JoystickAxis struct {
    Axis int
    Value int
}

func (axis *JoystickAxis) Serialize() string {
    return fmt.Sprintf("axis=%v value=%v", axis.Axis, axis.Value)
}

type SDLJoystickButtons struct {
    joystick *sdl.Joystick
    Inputs map[nes.Button]JoystickInput // normal nes buttons
    ExtraInputs map[EmulatorActionValue]JoystickInput // extra emulator-only buttons
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
        ExtraInputs: make(map[EmulatorActionValue]JoystickInput),
        Pressed: make(nes.ButtonMapping),
        Name: strings.TrimSpace(joystick.Name()),
    }, nil
}

func (joystick *SDLJoystickButtons) HandleEvent(event sdl.Event) EmulatorAction {
    joystick.Lock.Lock()
    defer joystick.Lock.Unlock()

    var emulatorOut EmulatorAction = MakeEmulatorAction(EmulatorNothing)

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
                        emulatorOut = MakeEmulatorAction(EmulatorTurbo)
                    } else {
                        emulatorOut = MakeEmulatorAction(EmulatorNormal)
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

func (joystick *SDLJoystickButtons) SetExtraButton(button EmulatorActionValue, input JoystickInput){
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

type EmulatorKeys struct {
    Turbo sdl.Keycode
    Pause sdl.Keycode
    HardReset sdl.Keycode
    PPUDebug sdl.Keycode
    SlowDown sdl.Keycode
    SpeedUp sdl.Keycode
    Normal sdl.Keycode
    StepFrame sdl.Keycode
    Record sdl.Keycode
    SaveState sdl.Keycode
    LoadState sdl.Keycode
    Console sdl.Keycode

    ButtonA sdl.Keycode
    ButtonB sdl.Keycode
    ButtonTurboA sdl.Keycode
    ButtonTurboB sdl.Keycode
    ButtonSelect sdl.Keycode
    ButtonStart sdl.Keycode
    ButtonUp sdl.Keycode
    ButtonDown sdl.Keycode
    ButtonLeft sdl.Keycode
    ButtonRight sdl.Keycode
}

type EmulatorKey struct {
    Name string
    Code sdl.Keycode
}

func (keys *EmulatorKeys) Update(key string, value sdl.Keycode) {
    switch key {
        case "A": keys.ButtonA = value
        case "B": keys.ButtonB = value
        case "TurboA": keys.ButtonTurboA = value
        case "TurboB": keys.ButtonTurboB = value
        case "Select": keys.ButtonSelect = value
        case "Start": keys.ButtonStart = value
        case "Up": keys.ButtonUp = value
        case "Down": keys.ButtonDown = value
        case "Left": keys.ButtonLeft = value
        case "Right": keys.ButtonRight = value
        case "Turbo": keys.Turbo = value
        case "Pause": keys.Pause = value
        case "HardReset": keys.HardReset = value
        case "PPUDebug": keys.PPUDebug = value
        case "SlowDown": keys.SlowDown = value
        case "SpeedUp": keys.SpeedUp = value
        case "Normal": keys.Normal = value
        case "StepFrame": keys.StepFrame = value
        case "Record": keys.Record = value
        case "SaveState": keys.SaveState = value
        case "LoadState": keys.LoadState = value
        case "Console": keys.Console = value

    }
}

func (keys *EmulatorKeys) UpdateAll(other EmulatorKeys){
    *keys = other
}

func (keys EmulatorKeys) AllKeys() []EmulatorKey {
    return []EmulatorKey{
        EmulatorKey{Name: "A", Code: keys.ButtonA},
        EmulatorKey{Name: "B", Code: keys.ButtonB},
        EmulatorKey{Name: "TurboA", Code: keys.ButtonTurboA},
        EmulatorKey{Name: "TurboB", Code: keys.ButtonTurboB},
        EmulatorKey{Name: "Select", Code: keys.ButtonSelect},
        EmulatorKey{Name: "Start", Code: keys.ButtonStart},
        EmulatorKey{Name: "Up", Code: keys.ButtonUp},
        EmulatorKey{Name: "Down", Code: keys.ButtonDown},
        EmulatorKey{Name: "Left", Code: keys.ButtonLeft},
        EmulatorKey{Name: "Right", Code: keys.ButtonRight},

        EmulatorKey{Name: "Turbo", Code: keys.Turbo},
        EmulatorKey{Name: "Pause", Code: keys.Pause},
        EmulatorKey{Name: "HardReset", Code: keys.HardReset},
        EmulatorKey{Name: "PPUDebug", Code: keys.PPUDebug},
        EmulatorKey{Name: "SlowDown", Code: keys.SlowDown},
        EmulatorKey{Name: "SpeedUp", Code: keys.SpeedUp},
        EmulatorKey{Name: "Normal", Code: keys.Normal},
        EmulatorKey{Name: "StepFrame", Code: keys.StepFrame},
        EmulatorKey{Name: "Record", Code: keys.Record},
        EmulatorKey{Name: "SaveState", Code: keys.SaveState},
        EmulatorKey{Name: "LoadState", Code: keys.LoadState},
        EmulatorKey{Name: "Console", Code: keys.Console},
    }
}

func DefaultEmulatorKeys() EmulatorKeys {
    return EmulatorKeys {
        Turbo: sdl.K_BACKQUOTE,
        Pause: sdl.K_SPACE,
        HardReset: sdl.K_r,
        PPUDebug: sdl.K_p,
        SlowDown: sdl.K_MINUS,
        SpeedUp: sdl.K_EQUALS,
        Normal: sdl.K_0,
        StepFrame: sdl.K_o,
        Record: sdl.K_m,
        SaveState: sdl.K_1,
        LoadState: sdl.K_2,
        Console: sdl.K_TAB,

        ButtonA: sdl.K_a,
        ButtonB: sdl.K_s,
        ButtonTurboA: sdl.K_d,
        ButtonTurboB: sdl.K_f,
        ButtonSelect: sdl.K_q,
        ButtonStart: sdl.K_RETURN,
        ButtonUp:  sdl.K_UP,
        ButtonDown: sdl.K_DOWN,
        ButtonLeft: sdl.K_LEFT,
        ButtonRight: sdl.K_RIGHT,
    }
}

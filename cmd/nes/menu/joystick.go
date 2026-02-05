package menu

import (
    "fmt"
    "log"
    "time"
    "sync"
    "context"
    "slices"
    "image/color"

    nes "github.com/kazzmir/nes/lib"
    "github.com/kazzmir/nes/lib/coroutine"
    "github.com/kazzmir/nes/cmd/nes/gfx"

    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
)

/* Probably this isn't needed, and the JoystickManager can take care of the mapping */
type JoystickButtonMapping struct {
    Inputs map[string]JoystickInputType
    ExtraInputs map[string]JoystickInputType
}

func convertButton(name string) nes.Button {
    switch name {
        case "Up": return nes.ButtonIndexUp
        case "Down": return nes.ButtonIndexDown
        case "Left": return nes.ButtonIndexLeft
        case "Right": return nes.ButtonIndexRight
        case "A": return nes.ButtonIndexA
        case "B": return nes.ButtonIndexB
        case "Select": return nes.ButtonIndexSelect
        case "Start": return nes.ButtonIndexStart
    }

    /* FIXME: error */
    return nes.ButtonIndexA
}

func convertExtraButton(name string) common.EmulatorActionValue {
    switch name {
        case "Fast emulation": return common.EmulatorTurbo
        case "Pause/Unpause Emulator": return common.EmulatorTogglePause
    }

    return common.EmulatorNothing
}

func convertInput(input JoystickInputType) common.JoystickInput {
    button, ok := input.(*JoystickButtonType)
    if ok {
        return &common.JoystickButton{Button: button.Button}
    }

    axis, ok := input.(*JoystickAxisType)
    if ok {
        return &common.JoystickAxis{Axis: axis.Axis, Value: axis.Value}
    }

    return nil
}

func (mapping *JoystickButtonMapping) UpdateJoystick(manager *common.JoystickManager){
    if manager.Player1 != nil {
        for name, input := range mapping.Inputs {
            manager.Player1.SetButton(convertButton(name), convertInput(input))
        }

        for name, input := range mapping.ExtraInputs {
            switch name {
                case "Turbo A":
                    manager.Player1.SetTurboA(convertInput(input))
                case "Turbo B":
                    manager.Player1.SetTurboB(convertInput(input))
                default:
                    manager.Player1.SetExtraButton(convertExtraButton(name), convertInput(input))
            }
        }


        err := manager.SaveInput()
        if err != nil {
            log.Printf("Warning: could not save joystick input: %v", err)
        }
    }
}

func (mapping *JoystickButtonMapping) AddAxisMapping(name string, axis JoystickAxisType){
    if slices.Contains(mapping.ButtonList(), name){
        mapping.Inputs[name] = &axis
    } else if slices.Contains(mapping.ExtraButtonList(), name){
        mapping.ExtraInputs[name] = &axis
    }
}

func (mapping *JoystickButtonMapping) AddButtonMapping(name string, button ebiten.GamepadButton){
    if slices.Contains(mapping.ButtonList(), name){
        mapping.Inputs[name] = &JoystickButtonType{
            Name: name,
            Pressed: false,
            Button: button,
       }
   } else if slices.Contains(mapping.ExtraButtonList(), name){
       mapping.ExtraInputs[name] = &JoystickButtonType{
            Name: name,
            Pressed: false,
            Button: button,
       }
   }
}

func (mapping *JoystickButtonMapping) Unmap(name string){
    delete(mapping.Inputs, name)
}

/*
func handleAxisMap(inputs map[string]JoystickInputType, event *sdl.JoyAxisEvent){
    / * release all axis based on the new event * /
    for _, input := range inputs {
        axis, ok := input.(*JoystickAxisType)
        if ok {
            axis.Pressed = false
        }
    }

    / * press the axis down if value is not zero * /
    if event.Value != 0 {
        for _, input := range inputs {
            axis, ok := input.(*JoystickAxisType)
            if ok && axis.Axis == int(event.Axis) && ((axis.Value < 0 && event.Value < 0) || (axis.Value > 0 && event.Value > 0)){
                axis.Pressed = true
            }
        }
    }
}
*/

/*
func (mapping *JoystickButtonMapping) HandleAxis(event *sdl.JoyAxisEvent){
    handleAxisMap(mapping.Inputs, event)
    handleAxisMap(mapping.ExtraInputs, event)
}
*/

func (mapping *JoystickButtonMapping) Press(rawButton ebiten.GamepadButton){
    for _, input := range mapping.Inputs {
        value, ok := input.(*JoystickButtonType)
        if ok && value.Button == rawButton {
            value.Pressed = true
        }
    }

    for _, input := range mapping.ExtraInputs {
        value, ok := input.(*JoystickButtonType)
        if ok && value.Button == rawButton {
            value.Pressed = true
        }
    }
}

func (mapping *JoystickButtonMapping) Release(rawButton ebiten.GamepadButton){
    for _, input := range mapping.Inputs {
        value, ok := input.(*JoystickButtonType)
        if ok && value.Button == rawButton {
            value.Pressed = false
        }
    }

    for _, input := range mapping.ExtraInputs {
        value, ok := input.(*JoystickButtonType)
        if ok && value.Button == rawButton {
            value.Pressed = false
        }
    }
}

/* returns the sdl joystick button mapped to the given name, or -1
 * if no such mapping exists
 */
func (mapping *JoystickButtonMapping) GetRawCode(name string) ebiten.GamepadButton {
    value, ok := mapping.Inputs[name]
    if ok {
        button, ok := value.(*JoystickButtonType)
        if ok {
            return button.Button
        }
    }

    return -1
}

func (mapping *JoystickButtonMapping) GetRawInput(name string) JoystickInputType {
    value, ok := mapping.Inputs[name]
    if ok {
        return value
    }
    return nil
}

func (mapping *JoystickButtonMapping) GetRawExtraInput(name string) JoystickInputType {
    value, ok := mapping.ExtraInputs[name]
    if ok {
        return value
    }
    return nil
}

func (mapping *JoystickButtonMapping) TotalButtons() int {
    return len(mapping.ButtonList()) + len(mapping.ExtraButtonList())
}

func (mapping *JoystickButtonMapping) GetConfigureButton(button int) string {
    if button < len(mapping.ButtonList()) {
        return mapping.ButtonList()[button]
    }

    button -= len(mapping.ButtonList())
    if button < len(mapping.ExtraButtonList()) {
        return mapping.ExtraButtonList()[button]
    }

    return "?"
}

func (mapping *JoystickButtonMapping) ButtonList() []string {
    /* FIXME: get this dynamically from the underlying Buttons map */
    return []string{"Up", "Down", "Left", "Right", "A", "B", "Select", "Start"}
}

func (mapping *JoystickButtonMapping) ExtraButtonList() []string {
    return []string{"Fast emulation", "Turbo A", "Turbo B", "Pause/Unpause Emulator"}
}

func actionName(action common.EmulatorActionValue) string {
    switch action {
        case common.EmulatorTurbo: return "Fast emulation"
        case common.EmulatorTogglePause: return "Pause/Unpause Emulator"
    }

    return ""
}

func (mapping *JoystickButtonMapping) IsPressed(name string) bool {
    input, ok := mapping.Inputs[name]
    if ok && input.IsPressed(){
        return true
    }

    input, ok = mapping.ExtraInputs[name]
    if ok && input.IsPressed(){
        return true
    }

    return false
}

type JoystickInputType interface {
    IsPressed() bool
    ToString() string
    Update(ebiten.GamepadID)
}

type JoystickButtonType struct {
    Button ebiten.GamepadButton
    Name string
    Pressed bool
}

func (button *JoystickButtonType) Update(gamepad ebiten.GamepadID) {
    button.Pressed = ebiten.IsGamepadButtonPressed(gamepad, button.Button)
}

func (button *JoystickButtonType) IsPressed() bool {
    return button.Pressed
}

func (button *JoystickButtonType) ToString() string {
    return fmt.Sprintf("button %03v", button.Button)
}

type JoystickAxisType struct {
    Axis int
    Value int
    Name string
    Pressed bool
    Zero float64
}

func abs(x float64) float64 {
    if x < 0 {
        return -x
    }

    return x
}

func (axis *JoystickAxisType) Update(gamepad ebiten.GamepadID){
    value := ebiten.GamepadAxisValue(gamepad, axis.Axis)
    if abs(value - float64(axis.Value)) < 0.1 {
        axis.Pressed = true
    } else {
        axis.Pressed = false
    }
}

func (axis *JoystickAxisType) IsPressed() bool {
    return axis.Pressed
}

func (axis *JoystickAxisType) ToString() string {
    return fmt.Sprintf("axis %02v value %v", axis.Axis, axis.Value)
}

type JoystickMenu struct {
    Buttons MenuButtons
    Quit MenuQuitFunc
    // JoystickName string
    // JoystickIndex int
    // Textures map[string]TextureId
    Lock sync.Mutex
    Configuring bool
    Mapping JoystickButtonMapping

    // the button currently being configured, which is an index into the ButtonList()
    PartialButton JoystickInputType
    PartialCounter int
    ConfigureButton int
    ConfigureButtonEnd int
    Released chan int
    ConfigurePrevious context.CancelFunc
    JoystickManager *common.JoystickManager
    AudioManager AudioManager

    ConfigureCoroutine *coroutine.Coroutine
}

const JoystickMaxPartialCounter = 20

func (menu *JoystickMenu) PlayBeep() {
    menu.AudioManager.PlayBeep()
}

func (menu *JoystickMenu) UpdateWindowSize(x int, y int){
    // nothing
}

func (menu *JoystickMenu) FinishConfigure() {
    menu.Configuring = false
    menu.Mapping.UpdateJoystick(menu.JoystickManager)
    err := menu.JoystickManager.SaveInput()
    if err != nil {
        log.Printf("Warning: could not save joystick configuration: %v", err)
    }
}

/*
func (menu *JoystickMenu) RawInput(event sdl.Event){
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    if menu.Configuring {
        / * if its a press then set the current partial key to that press
         * and set a timer for ~1s, if the release comes after 1s then
         * set the button.
         * /
        button, ok := event.(*sdl.JoyButtonEvent)
        if ok {
            // log.Printf("Raw joystick input: %+v", button)
            switch button.Type {
                case sdl.JOYBUTTONDOWN:
                    menu.PartialButton = &JoystickButtonType{Button: int(button.Button)}
                    menu.PartialCounter = 0
                    if menu.ConfigurePrevious != nil {
                        menu.ConfigurePrevious()
                    }

                    quit, cancel := context.WithCancel(context.Background())
                    menu.ConfigurePrevious = cancel

                    go func(pressed JoystickButtonType){
                        ticker := time.NewTicker(1000 / JoystickMaxPartialCounter * time.Millisecond)
                        defer ticker.Stop()
                        ok := false
                        done := false
                        for !done {
                            select {
                            case use := <-menu.Released:
                                if use == pressed.Button {
                                    done = true
                                }
                            case <-quit.Done():
                                return
                            case <-ticker.C:
                                menu.Lock.Lock()
                                if menu.PartialCounter < JoystickMaxPartialCounter {
                                    menu.PartialCounter += 1
                                } else {
                                    ok = true
                                }
                                menu.Lock.Unlock()
                            }
                        }

                        menu.Lock.Lock()
                        defer menu.Lock.Unlock()

                        if ok {
                            // menu.Mapping.Buttons[menu.Mapping.ButtonList()[menu.ConfigureButton]] = pressed
                            menu.Mapping.AddButtonMapping(pressed.Name, pressed.Button)
                            menu.ConfigureButton += 1
                            if menu.ConfigureButton >= menu.ConfigureButtonEnd {
                                menu.FinishConfigure()
                            }
                        } else {
                            menu.PartialButton = nil
                            menu.Mapping.Unmap(pressed.Name)
                        }

                        / * FIXME: channel leak with the timer * /
                        // ticker.Stop()
                        / *
                        if !timer.Stop() {
                            go func(){
                                <-timer.C
                            }()
                        }
                        * /
                    }(JoystickButtonType{
                        Name: menu.Mapping.GetConfigureButton(menu.ConfigureButton),
                        Button: int(button.Button),
                        Pressed: false,
                    })
                case sdl.JOYBUTTONUP:
                    menu.Mapping.Release(int(button.Button))
                    select {
                        case menu.Released <- int(button.Button):
                        default:
                    }
                    menu.PartialButton = nil
            }
        }

        / * if its an axis event then keep track of which axis and value was pressed.
         * as long as the same axis and mostly the same value is pressed then use that
         * pair of values (axis, value) as the button
         * /
        axis, ok := event.(*sdl.JoyAxisEvent)
        if ok {
            log.Printf("Axis event axis=%v value=%v\n", axis.Axis, axis.Value)

            / * when the user lets go of the current axis button a 'release' axis event
             * will be emitted, which is an axis event with value=0. at that point
             * the ConfigurePrevious() cancel method will be invoked, which will cause
             * the most recently pressed axis to configure the button.
             * /
            menu.PartialCounter = 0
            if menu.ConfigurePrevious != nil {
                menu.ConfigurePrevious()
            }

            if axis.Value != 0 {
                quit, cancel := context.WithCancel(context.Background())
                menu.ConfigurePrevious = cancel

                pressed := JoystickAxisType{Axis: int(axis.Axis), Value: int(axis.Value)}

                menu.PartialButton = &pressed

                go func(){
                    ticker := time.NewTicker(1000 / JoystickMaxPartialCounter * time.Millisecond)
                    defer ticker.Stop()
                    ok := false
                    done := false
                    for !done {
                        select {
                        case <-quit.Done():
                            done = true
                        case <-ticker.C:
                            menu.Lock.Lock()
                            if menu.PartialCounter < JoystickMaxPartialCounter {
                                menu.PartialCounter += 1
                            } else {
                                ok = true
                            }
                            menu.Lock.Unlock()
                        }
                    }

                    menu.Lock.Lock()
                    defer menu.Lock.Unlock()

                    / * the axis was held long enough * /
                    if ok {
                        log.Printf("Map button %v to axis %v value %v\n", menu.ConfigureButton, axis.Axis, axis.Value)
                        menu.Mapping.AddAxisMapping(menu.Mapping.GetConfigureButton(menu.ConfigureButton), pressed)
                        menu.ConfigureButton += 1
                        if menu.ConfigureButton >= menu.ConfigureButtonEnd {
                            menu.FinishConfigure()
                        }
                    } else {
                        menu.PartialButton = nil
                    }
                }()
            }
        }

    } else {
        button, ok := event.(*sdl.JoyButtonEvent)
        if ok {
            // log.Printf("Raw joystick input: %+v", button)
            switch button.Type {
                case sdl.JOYBUTTONDOWN:
                    menu.Mapping.Press(int(button.Button))
                case sdl.JOYBUTTONUP:
                    menu.Mapping.Release(int(button.Button))
            }
        }

        axis, ok := event.(*sdl.JoyAxisEvent)
        if ok {
            menu.Mapping.HandleAxis(axis)
        }
    }
}
*/

func (menu *JoystickMenu) DoConfigure(joystick *common.JoystickButtons, yield coroutine.YieldFunc, buttonList []string) {

    totalAxis := ebiten.GamepadAxisCount(joystick.GetGamepadID())

    type AxisInfo struct {
        Time time.Time
        Value float64
    }

    abs := func(a float64) float64 {
        if a < 0 {
            return -a
        }
        return a
    }

    // what values the axis has when centered (untouched)
    // most are 0, but some buttons start at either -1 or 1
    axisZero := make(map[int]float64)

    count := 10

    for range count {
        for axis := range totalAxis {
            value := ebiten.GamepadAxisValue(joystick.GetGamepadID(), axis)
            axisZero[axis] += value
        }

        yield()
    }

    for key, value := range axisZero {
        axisZero[key] = value / float64(count)
    }

    configureButton := func(button string) bool {
        axisValues := make(map[int]AxisInfo)
        var lastTime time.Time
        lastButton := ebiten.GamepadButton(-1)
        log.Printf("Configuring button '%v'", button)
        for {
            pressed := inpututil.AppendJustPressedGamepadButtons(joystick.GetGamepadID(), nil)
            for _, button := range pressed {
                lastButton = button
                lastTime = time.Now()
            }

            released := inpututil.AppendJustReleasedGamepadButtons(joystick.GetGamepadID(), nil)
            for _, button := range released {
                if button == lastButton {
                    lastButton = ebiten.GamepadButton(-1)
                }
            }

            if lastButton != ebiten.GamepadButton(-1) && time.Since(lastTime) > 700 * time.Millisecond {
                menu.Mapping.AddButtonMapping(button, lastButton)
                menu.ConfigureButton += 1
                return true
            }

            for axis := range totalAxis {
                value := ebiten.GamepadAxisValue(joystick.GetGamepadID(), axis)
                if abs(value - axisZero[axis]) > 0.5 {
                    previous, ok := axisValues[axis]
                    if ok {
                        diff := abs(previous.Value - value)

                        if diff < 0.2 {
                            if time.Since(previous.Time) > 700 * time.Millisecond {
                                menu.Mapping.AddAxisMapping(button, JoystickAxisType{
                                    Axis: int(axis),
                                    Value: int(value),
                                    Name: button,
                                    Pressed: false,
                                    Zero: axisZero[axis],
                                })
                                menu.ConfigureButton += 1

                                log.Printf("Configured button '%v' to axis %v value %v", button, axis, value)
                                return true
                            }
                        } else {
                            previous.Time = time.Now()
                            previous.Value = value
                            axisValues[axis] = previous
                        }
                    } else {
                        previous.Time = time.Now()
                        previous.Value = value
                        axisValues[axis] = previous
                    }
                }
            }

            if yield() != nil {
                return false
            }
        }
    }

    for _, button := range buttonList {
        if !configureButton(button) {
            return
        }
    }

    menu.FinishConfigure()
}

func (menu *JoystickMenu) Update(){

    if menu.JoystickManager.Player1 != nil {
        gamepadID := menu.JoystickManager.Player1.GetGamepadID()
        for _, input := range menu.Mapping.Inputs {
            input.Update(gamepadID)
        }

        for _, input := range menu.Mapping.ExtraInputs {
            input.Update(gamepadID)
        }
    }

    if menu.ConfigureCoroutine != nil {
        menu.ConfigureCoroutine.Run()
        if !menu.Configuring {
            menu.ConfigureCoroutine.Stop()
            menu.ConfigureCoroutine = nil
        }
    }
}

func (menu *JoystickMenu) MouseMove(x int, y int) {
    menu.Buttons.MouseMove(x, y)
}

func (menu *JoystickMenu) MouseClick(x int, y int) SubMenu {
    return menu.Buttons.MouseClick(x, y, menu)
}

func (menu *JoystickMenu) Input(input MenuInput) SubMenu {
    switch input {
        case MenuQuit:
            menu.Lock.Lock()
            defer menu.Lock.Unlock()
            menu.Configuring = false

            if menu.ConfigurePrevious != nil {
                menu.ConfigurePrevious()
            }

            return menu.Quit(menu)
        default:
            menu.Lock.Lock()
            ok := !menu.Configuring
            menu.Lock.Unlock()
            if ok {
                return menu.Buttons.Interact(input, menu)
            }

            return menu
    }
}

func (menu *JoystickMenu) MakeRenderer(font text.Face, smallFont text.Face, clock uint64) gfx.RenderFunction {
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    fontWidth, fontHeight := text.Measure("A", font, 1)
    _ = fontWidth

    return func(screen *ebiten.Image) error {
        name := fmt.Sprintf("Joystick: %v", menu.JoystickManager.CurrentName())

        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
        red := color.RGBA{R: 255, G: 0, B: 0, A: 255}

        var textOptions text.DrawOptions
        textOptions.GeoM.Translate(10, 10)
        text.Draw(screen, name, font, &textOptions)

        x := float64(50)
        y := float64(100)
        _, y, err := menu.Buttons.Render(x, y, font, screen, clock)
        if err != nil {
            return err
        }

        y += fontHeight * 2

        if menu.Configuring {
            configureText := "Configuring: hold a button for 1 second to set it"
            textOptions.GeoM.Reset()
            textOptions.GeoM.Translate(x, y)
            text.Draw(screen, configureText, font, &textOptions)
        }

        buttons := menu.Mapping.ButtonList()

        verticalMargin := float64(20)
        x = 80
        y += fontHeight
        // y += font.Height() * 3 + verticalMargin

        drawOffsetYButtons := y

        maxWidth := float64(0)

        /* draw the regular buttons on the left side */

        /* map the button name to its vertical position */
        buttonPositions := make(map[string]float64)

        for i, button := range buttons {
            buttonPositions[button] = y
            color := white

            if menu.Configuring && menu.ConfigureButton == i {
                color = red
            }

            if !menu.Configuring && menu.Mapping.IsPressed(button) {
                color = red
            }

            width, height := drawButton(smallFont, screen, x, y, button, color)
            if width > maxWidth {
                maxWidth = width
            }
            _ = width
            _ = height
            y += height + verticalMargin
        }

        maxWidth2 := maxWidth
        extraInputsStart := 0

        for i, button := range buttons {
            rawButton := menu.Mapping.GetRawInput(button)
            extraInputsStart = i + 1
            _ = extraInputsStart
            mapped := "Unmapped"
            col := white
            if rawButton != nil {
                mapped = fmt.Sprintf("%03v", rawButton.ToString())
            }

            if menu.Configuring && menu.ConfigureButton == i {
                mapped = "?"
                if menu.PartialButton !=  nil{
                    mapped = menu.PartialButton.ToString()
                    /*
                    button, ok := menu.PartialButton.(*JoystickButtonType)
                    if ok {
                        mapped = fmt.Sprintf("button %03v", button.Button)
                    }

                    axis, ok := menu.PartialButton.(*JoystickAxisType)
                    if ok {
                        mapped = fmt.Sprintf("axis %02v value %v", axis.Axis, axis.Value)
                    }
                    */

                    m := uint8(menu.PartialCounter * 255 / JoystickMaxPartialCounter)

                    if menu.PartialCounter == JoystickMaxPartialCounter {
                        col = color.RGBA{R: 255, G: 255, B: 0, A: 255}
                    } else {
                        col = color.RGBA{R: 255, G: m, B: m, A: 255}
                    }
                }
            }

            vx := x + maxWidth + 20
            vy := buttonPositions[button]
            width, height, err := drawConstButton(smallFont, screen, vx, vy, mapped, col)

            if width > maxWidth2 {
                maxWidth2 = width
            }

            _ = height
            if err != nil {
                return err
            }
        }

        /* draw the extra buttons on the right side */
        y = drawOffsetYButtons
        x += maxWidth + maxWidth2 + 20 + 60

        extraButtons := menu.Mapping.ExtraButtonList()
        extraButtonPositions := make(map[string]float64)
        maxWidthExtra := maxWidth
        for i, button := range extraButtons {
            color := white

            if menu.Configuring && menu.ConfigureButton == extraInputsStart + i {
                color = red
            }

            if !menu.Configuring && menu.Mapping.IsPressed(button) {
                color = red
            }

            width, height := drawButton(smallFont, screen, x, y, button, color)
            if width > maxWidthExtra {
                maxWidthExtra = width
            }
            extraButtonPositions[button] = y
            _ = width
            _ = height
            y += height + verticalMargin
        }

        for i, button := range extraButtons {
            rawButton := menu.Mapping.GetRawExtraInput(button)
            mapped := "Unmapped"
            col := white
            if rawButton != nil {
                mapped = fmt.Sprintf("%03v", rawButton.ToString())
            }

            if menu.Configuring && menu.ConfigureButton == extraInputsStart + i {
                mapped = "?"
                if menu.PartialButton !=  nil{
                    mapped = menu.PartialButton.ToString()
                    /*
                    button, ok := menu.PartialButton.(*JoystickButtonType)
                    if ok {
                        mapped = fmt.Sprintf("button %03v", button.Button)
                    }

                    axis, ok := menu.PartialButton.(*JoystickAxisType)
                    if ok {
                        mapped = fmt.Sprintf("axis %02v value %v", axis.Axis, axis.Value)
                    }
                    */

                    m := uint8(menu.PartialCounter * 255 / JoystickMaxPartialCounter)

                    if menu.PartialCounter == JoystickMaxPartialCounter {
                        col = color.RGBA{R: 255, G: 255, B: 0, A: 255}
                    } else {
                        col = color.RGBA{R: 255, G: m, B: m, A: 255}
                    }
                }
            }

            vx := x + maxWidthExtra + 20
            vy := extraButtonPositions[button]
            width, height, err := drawConstButton(smallFont, screen, vx, vy, mapped, col)

            _ = width
            _ = height
            if err != nil {
                return err
            }
        }

        return nil
    }
}

func (joystickMenu *JoystickMenu) UpdateMapping() {
    joystickMenu.Lock.Lock()
    defer joystickMenu.Lock.Unlock()

    joystickMenu.Mapping = JoystickButtonMapping{
        Inputs: make(map[string]JoystickInputType),
        ExtraInputs: make(map[string]JoystickInputType),
    }

    joystick := joystickMenu.JoystickManager.Player1
    if joystick != nil {
        for button, input := range joystick.Inputs {
            name := nes.ButtonName(button)

            realButton, ok := input.(*common.JoystickButton)
            if ok {
                joystickMenu.Mapping.AddButtonMapping(name, realButton.Button)
            }

            realAxis, ok := input.(*common.JoystickAxis)
            if ok {
                joystickMenu.Mapping.AddAxisMapping(name, JoystickAxisType{
                    Axis: realAxis.Axis,
                    Value: realAxis.Value,
                    Name: name,
                    Pressed: false,
                })
            }
        }

        turboA, ok := joystick.TurboA.(*common.JoystickButton)
        if ok {
            joystickMenu.Mapping.AddButtonMapping("Turbo A", turboA.Button)
        }

        turboA_axis, ok := joystick.TurboA.(*common.JoystickAxis)
        if ok {
            joystickMenu.Mapping.AddAxisMapping("Turbo A", JoystickAxisType{
                Axis: turboA_axis.Axis,
                Value: turboA_axis.Value,
                Name: "Turbo A",
                Pressed: false,
            })
        }

        turboB, ok := joystick.TurboB.(*common.JoystickButton)
        if ok {
            joystickMenu.Mapping.AddButtonMapping("Turbo B", turboB.Button)
        }

        turboB_axis, ok := joystick.TurboB.(*common.JoystickAxis)
        if ok {
            joystickMenu.Mapping.AddAxisMapping("Turbo B", JoystickAxisType{
                Axis: turboB_axis.Axis,
                Value: turboB_axis.Value,
                Name: "Turbo B",
                Pressed: false,
            })
        }

        for action, input := range joystick.ExtraInputs {
            name := actionName(action)
            realButton, ok := input.(*common.JoystickButton)
            if ok {
                joystickMenu.Mapping.AddButtonMapping(name, realButton.Button)
            }

            realAxis, ok := input.(*common.JoystickAxis)
            if ok {
                joystickMenu.Mapping.AddAxisMapping(name, JoystickAxisType{
                    Axis: realAxis.Axis,
                    Value: realAxis.Value,
                    Name: name,
                    Pressed: false,
                })
            }
        }
    }
}

func MakeJoystickMenu(parent SubMenu, joystickStateChanges <-chan JoystickState, joystickManager *common.JoystickManager, audioManager AudioManager) SubMenu {
    menu := &JoystickMenu{
        Quit: func(current SubMenu) SubMenu {
            return parent
        },
        // JoystickName: "No joystick found",
        // Textures: make(map[string]TextureId),
        // JoystickIndex: -1,
        AudioManager: audioManager,
        Mapping: JoystickButtonMapping{
            Inputs: make(map[string]JoystickInputType),
            ExtraInputs: make(map[string]JoystickInputType),
        },
        Released: make(chan int, 4),
        ConfigurePrevious: nil,
        JoystickManager: joystickManager,
    }

    menu.UpdateMapping()

    go func(){
        for stateChange := range joystickStateChanges {
            _ = stateChange
            // log.Printf("Joystick state change: %v", stateChange)

            /*
            add, ok := stateChange.(*JoystickStateAdd)
            if ok {
                // log.Printf("Add joystick")
                // menu.Lock.Lock()
                err := joystickManager.AddJoystick(add.Index)
                if err != nil && err != common.JoystickAlreadyAdded {
                    log.Printf("Warning: could not add joystick %v: %v\n", add.InstanceId, err)
                }

                / *
                menu.JoystickName = add.Name
                menu.JoystickIndex = add.Index
                log.Printf("Set joystick to '%v' index %v", add.Name, add.Index)
                * /
                // menu.Lock.Unlock()
            }
            */

            /*
            remove, ok := stateChange.(*JoystickStateRemove)
            if ok {
                // log.Printf("Remove joystick")
                _ = remove
                // menu.Lock.Lock()
                joystickManager.RemoveJoystick(remove.InstanceId)
                / *
                menu.JoystickName = "No joystick found"
                menu.JoystickIndex = -1
                * /
                // menu.Lock.Unlock()
            }
            */
        }
    }()

    menu.Buttons.Add(&SubMenuButton{Name: "Back", Func: func() SubMenu{ return parent } })

    menu.Buttons.Add(&MenuNextLine{})
    menu.Buttons.Add(&SubMenuButton{Name: "Previous Joystick", Func: func() SubMenu {
        joystickManager.PreviousJoystick()
        menu.UpdateMapping()
        return menu
    }})
    menu.Buttons.Add(&SubMenuButton{Name: "Next Joystick", Func: func() SubMenu {
        joystickManager.NextJoystick()
        menu.UpdateMapping()
        return menu
    }})
    menu.Buttons.Add(&MenuNextLine{})
    menu.Buttons.Add(&MenuLabel{Label: "Configure", Color: color.RGBA{R: 255, G: 0, B: 0, A: 255}})
    menu.Buttons.Add(&MenuNextLine{})

    menu.Buttons.Add(&SubMenuButton{Name: "All Buttons", Func: func() SubMenu {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        if joystickManager.Player1 != nil {

            menu.ConfigureButton = 0
            menu.ConfigureButtonEnd = menu.Mapping.TotalButtons()
            menu.Configuring = true
            menu.Mapping.Inputs = make(map[string]JoystickInputType)
            menu.Mapping.ExtraInputs = make(map[string]JoystickInputType)

            joystick := joystickManager.Player1
            menu.ConfigureCoroutine = coroutine.MakeCoroutine(func(yield coroutine.YieldFunc) error {
                menu.DoConfigure(joystick, yield, append(menu.Mapping.ButtonList(), menu.Mapping.ExtraButtonList()...))
                return nil
            })
        }

        return menu
    }})

    menu.Buttons.Add(&SubMenuButton{Name: "Main Buttons", Func: func() SubMenu {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        if joystickManager.Player1 != nil {
            menu.Configuring = true
            menu.ConfigureButton = 0
            menu.ConfigureButtonEnd = len(menu.Mapping.ButtonList())
            menu.Mapping.Inputs = make(map[string]JoystickInputType)

            joystick := joystickManager.Player1
            menu.ConfigureCoroutine = coroutine.MakeCoroutine(func(yield coroutine.YieldFunc) error {
                menu.DoConfigure(joystick, yield, menu.Mapping.ButtonList())
                return nil
            })
        }
        return menu
    }})

    menu.Buttons.Add(&SubMenuButton{Name: "Extra Buttons", Func: func() SubMenu {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        if joystickManager.Player1 != nil {
            menu.Configuring = true
            menu.ConfigureButton = len(menu.Mapping.ButtonList())
            menu.ConfigureButtonEnd = menu.Mapping.TotalButtons()
            menu.Mapping.ExtraInputs = make(map[string]JoystickInputType)

            joystick := joystickManager.Player1
            menu.ConfigureCoroutine = coroutine.MakeCoroutine(func(yield coroutine.YieldFunc) error {
                menu.DoConfigure(joystick, yield, menu.Mapping.ExtraButtonList())
                return nil
            })
        }

        return menu
    }})

    return menu
}

package menu

import (
    "context"

    // "io"
    // "io/fs"
    // "runtime"
    "time"
    "os"
    "fmt"
    "math"
    "math/rand/v2"
    "bytes"
    "log"
    "sync"
    "strings"
    "text/template"
    "path/filepath"

    "image"
    "image/png"
    "image/color"

    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/kazzmir/nes/cmd/nes/gfx"
    // "github.com/kazzmir/nes/data"
    nes "github.com/kazzmir/nes/lib"
    "github.com/kazzmir/nes/lib/coroutine"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
    "github.com/hajimehoshi/ebiten/v2/vector"
)

type MenuInput int
const (
    MenuToggle = iota
    MenuNext
    MenuPrevious
    MenuUp
    MenuDown
    MenuSelect
    MenuQuit // usually when ESC is input
)

type Menu struct {
    active bool
    quit context.Context
    cancel context.CancelFunc
    font text.Face
    Input chan MenuInput
    Lock sync.Mutex
    AudioManager AudioManager
}

type MenuAction int
const (
    MenuActionQuit = iota
    MenuActionLoadRom
    MenuActionSound
    MenuActionJoystick
)

type Snow struct {
    color uint8
    x float32
    y float32
    truex float32
    truey float32
    angle float32
    direction int
    speed float32
    fallSpeed float32
}

func MakeSnow(screenWidth int) Snow {
    trueWidth := float32(screenWidth) * 1.2

    x := rand.Float32() * trueWidth - trueWidth * 0.1
    // y := rand.Float32() * 400
    y := float32(0)
    return Snow{
        color: uint8(rand.N(210) + 40),
        x: x,
        y: y,
        truex: x,
        truey: y,
        angle: rand.Float32() * 180,
        direction: 1,
        speed: rand.Float32() * 3 + 0.5,
        fallSpeed: rand.Float32() * 1.3 + 0.2,
    }
}

type ProgramActions interface {
    LoadRom(name string, file common.MakeFile)
    SetSoundEnabled(enabled bool)
    IsSoundEnabled() bool
}

type AudioManager interface {
    PlayBeep()
}

/* an interactable button */
func drawButton(font text.Face, out *ebiten.Image, x float64, y float64, message string, col color.Color) (float64, float64, error) {
    buttonInside := color.RGBA{R: 64, G: 64, B: 64, A: 255}
    buttonOutline := color.RGBA{R: 32, G: 32, B: 32, A: 255}

    margin := 12

    width, height := text.Measure(message, font, 1)

    vector.FillRect(out, float32(x), float32(y), float32(width) + float32(margin), float32(height) + float32(margin), buttonOutline, true)
    vector.FillRect(out, float32(x + 1), float32(y + 1), float32(width) + float32(margin) - 3, float32(height) + float32(margin) - 3, buttonInside, true)

    var options text.DrawOptions
    options.GeoM.Translate(x + float64(margin) / 2, y + float64(margin) / 2)
    options.ColorScale.ScaleWithColor(col)
    text.Draw(out, message, font, &options)

    return width, height, nil
}

func drawFixedWidthButton(font text.Face, out *ebiten.Image, width float64, x float64, y float64, message string, col color.Color) (float64, float64, error) {
    buttonInside := color.RGBA{R: 64, G: 64, B: 64, A: 255}
    buttonOutline := color.RGBA{R: 32, G: 32, B: 32, A: 255}

    _, height := text.Measure(message, font, 1)

    margin := 12.0

    vector.FillRect(out, float32(x), float32(y), float32(width) + float32(margin), float32(height) + float32(margin), buttonOutline, true)
    vector.FillRect(out, float32(x + 1), float32(y + 1), float32(width) + float32(margin) - 3, float32(height) + float32(margin) - 3, buttonInside, true)

    var textOptions text.DrawOptions
    textOptions.GeoM.Translate(x + float64(margin) / 2, y + float64(margin) / 2)
    textOptions.ColorScale.ScaleWithColor(col)
    text.Draw(out, message, font, &textOptions)

    return width + margin, height, nil
}

/* a button that cannot be interacted with */
func drawConstButton(font text.Face, out *ebiten.Image, x float64, y float64, message string, col color.Color) (float64, float64, error) {
    buttonInside := color.RGBA{R: 0x55, G: 0x55, B: 0x40, A: 255}
    buttonOutline := color.RGBA{R: 32, G: 32, B: 32, A: 255}

    margin := 12

    width, height := text.Measure(message, font, 1)

    vector.FillRect(out, float32(x), float32(y), float32(width) + float32(margin), float32(height) + float32(margin), buttonOutline, true)
    vector.FillRect(out, float32(x + 1), float32(y + 1), float32(width) + float32(margin) - 3, float32(height) + float32(margin) - 3, buttonInside, true)

    var textOptions text.DrawOptions
    textOptions.GeoM.Translate(x + float64(margin) / 2, y + float64(margin) / 2)
    textOptions.ColorScale.ScaleWithColor(col)
    text.Draw(out, message, font, &textOptions)

    return width, height, nil
}

type MenuState int
const (
    MenuStateTop = iota
    MenuStateLoadRom
)

func chainRenders(functions ...gfx.RenderFunction) gfx.RenderFunction {
    return func(screen *ebiten.Image) error {
        for _, f := range functions {
            err := f(screen)
            if err != nil {
                return err
            }
        }

        return nil
    }
}

type NullInput struct {
}

func (buttons *NullInput) Get() nes.ButtonMapping {
    return make(nes.ButtonMapping)
}


func (menu *Menu) IsActive() bool {
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    return menu.active
}

func (menu *Menu) ToggleActive(){
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    menu.active = ! menu.active
}

func doRender(raw_pixels []byte, out *ebiten.Image) error {
    out.WritePixels(raw_pixels)
    return nil
}

func loadPng(path string) (image.Image, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    return png.Decode(file)
}

func MakeMenu(mainQuit context.Context, font text.Face, audioManager AudioManager) Menu {
    quit, cancel := context.WithCancel(mainQuit)
    menuInput := make(chan MenuInput, 5)

    return Menu{
        active: false,
        quit: quit,
        cancel: cancel,
        font: font,
        Input: menuInput,
        AudioManager: audioManager,
    }
}

type MenuItem interface {
    Text() string
    /* returns next x,y coordinate where rendering can occur, and a possible error */
    Render(text.Face, *ebiten.Image, float64, float64, bool, uint64) (float64, float64, error)
}

type MenuSpace struct {
    Space int
}

func (space *MenuSpace) Text() string {
    return ""
}

func (space *MenuSpace) Render(font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    return x + float64(space.Space), y, nil
}

type MenuNextLine struct {
}

func (line *MenuNextLine) Text() string {
    return "\n"
}

func (line *MenuNextLine) Render(font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    /* Force the renderer to go to the next line */
    return 999999999, 0, nil
}

type MenuLabel struct {
    Label string
    Color color.Color
}

func (label *MenuLabel) Text() string {
    return label.Label
}

func (label *MenuLabel) Render(font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    return drawButton(font, out, x, y, label.Text(), label.Color)
}

type Button interface {
    MenuItem
    /* invoked when the user presses enter while selecting this button */
    Interact(SubMenu) SubMenu
}

type StaticButtonFunc func(button *StaticButton)

/* A button that does not change state */
type StaticButton struct {
    Name string
    Func StaticButtonFunc
    Lock sync.Mutex
}

func (button *StaticButton) Text() string {
    button.Lock.Lock()
    defer button.Lock.Unlock()
    return button.Name
}

func (button *StaticButton) Update(text string){
    button.Lock.Lock()
    defer button.Lock.Unlock()
    button.Name = text
}

func (button *StaticButton) Interact(menu SubMenu) SubMenu {
    if button.Func != nil {
        button.Func(button)
    }

    return menu
}

func (button *StaticButton) Render(font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    return _doRenderButton(button, font, out, x, y, selected, clock)
}

type ToggleButtonFunc func(bool)

type ToggleButton struct {
    State1 string
    State2 string
    state bool
    Func ToggleButtonFunc
}

func (toggle *ToggleButton) Text() string {
    switch toggle.state {
        case true: return toggle.State1
        case false: return toggle.State2
    }
    return ""
}

func (toggle *ToggleButton) Interact(menu SubMenu) SubMenu {
    toggle.state = !toggle.state
    toggle.Func(toggle.state)
    return menu
}

func (button *ToggleButton) Render(font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    return _doRenderButton(button, font, out, x, y, selected, clock)
}

type SubMenuFunc func() SubMenu

type SubMenuButton struct {
    Name string
    Func SubMenuFunc
}

func _doRenderButton(button Button, font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}
    red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
    white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

    var use color.Color = white
    if selected {
        use = gfx.Glow(red, yellow, 40, clock)
    }

    return drawButton(font, out, x, y, button.Text(), use)
}

type StaticFixedButtonFunc func(*StaticFixedWidthButton)

/* A button that renders its components in a fixed width */
type StaticFixedWidthButton struct {
    Width int
    Parts []string
    Func StaticFixedButtonFunc
    Lock sync.Mutex
}

func (button *StaticFixedWidthButton) Text() string {
    button.Lock.Lock()
    defer button.Lock.Unlock()
    out := ""
    /* FIXME: this doesn't really take into account the width */
    for _, part := range button.Parts {
        out += part
    }
    return out
}

func (button *StaticFixedWidthButton) Update(parts... string){
    button.Lock.Lock()
    button.Parts = parts
    button.Lock.Unlock()
}

func (button *StaticFixedWidthButton) Interact(menu SubMenu) SubMenu {
    if button.Func != nil {
        button.Func(button)
    }

    return menu
}

func (button *StaticFixedWidthButton) Render(font text.Face, screen *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}
    red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
    white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

    var col color.Color = white
    if selected {
        col = gfx.Glow(red, yellow, 40, clock)
    }

    /*
    button.Lock.Lock()
    parts := gfx.CopyArray(button.Parts)
    button.Lock.Unlock()
    */

    totalLength := 0.0
    for _, part := range button.Parts {
        width, _ := text.Measure(part, font, 1)
        totalLength += width
    }

    space, _ := text.Measure(" ", font, 1)

    left := float64(button.Width) - totalLength
    var out string
    if left > 0 {
        spaces := left / space

        if len(button.Parts) > 0 {
            out = button.Parts[0]
            if len(button.Parts) > 1 {
                out += strings.Repeat(" ", int(spaces))
                for _, part := range button.Parts[1:] {
                    out += part
                }
            }
        }
    } else {
        for _, part := range button.Parts {
            out += part
        }
    }

    width, height, err := drawFixedWidthButton(font, screen, float64(button.Width), x, y, out, col)

    return width, height, err
}

func (button *SubMenuButton) Render(font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    return _doRenderButton(button, font, out, x, y, selected, clock)
}

func (button *SubMenuButton) Text() string {
    return button.Name
}

func (button *SubMenuButton) Interact(menu SubMenu) SubMenu {
    return button.Func()
}

type MenuButtons struct {
    Items []MenuItem
    Selected int
    Lock sync.Mutex
}

func MakeMenuButtons() MenuButtons {
    return MenuButtons{
        Selected: 0,
    }
}

func (buttons *MenuButtons) Previous(){
    for {
        buttons.Selected -= 1
        if buttons.Selected < 0 {
            buttons.Selected = len(buttons.Items) - 1
        }
        _, ok := buttons.Items[buttons.Selected].(Button)
        if ok {
            break
        }
    }
}

func (buttons *MenuButtons) Next(){
    for {
        buttons.Selected = (buttons.Selected + 1) % len(buttons.Items)
        _, ok := buttons.Items[buttons.Selected].(Button)
        if ok {
            break
        }
    }
}

func (buttons *MenuButtons) Select(item MenuItem){
    for i := 0; i < len(buttons.Items); i++ {
        if buttons.Items[i] == item {
            buttons.Selected = i
            return
        }
    }
}

func (buttons *MenuButtons) Add(item MenuItem){
    buttons.Items = append(buttons.Items, item)
}

type SubMenu interface {
    /* Returns the new menu based on what button was pressed */
    Input(input MenuInput) SubMenu
    MakeRenderer(font text.Face, smallFont text.Face, clock uint64) gfx.RenderFunction
    UpdateWindowSize(int, int)
    PlayBeep()
    Update()
}

func (buttons *MenuButtons) Interact(input MenuInput, menu SubMenu) SubMenu {
    buttons.Lock.Lock()
    defer buttons.Lock.Unlock()

    switch input {
    case MenuPrevious, MenuUp:
        buttons.Previous()
        menu.PlayBeep()
    case MenuNext, MenuDown:
        buttons.Next()
        menu.PlayBeep()
    case MenuSelect:
        button, ok := buttons.Items[buttons.Selected].(Button)
        if ok {
            return button.Interact(menu)
        }
    }

    return menu
}

func (buttons *MenuButtons) Render(startX float64, startY float64, font text.Face, renderer *ebiten.Image, clock uint64) (float64, float64, error) {
    buttons.Lock.Lock()
    defer buttons.Lock.Unlock()

    const itemDistance = 50

    x := startX
    y := startY

    maxWidth := float64(renderer.Bounds().Dx())

    for i, item := range buttons.Items {

        width, height := text.Measure(item.Text(), font, 1)

        if x > maxWidth - width {
            x = startX
            y += height + 20
        }

        itemWidth, _, err := item.Render(font, renderer, x, y, i == buttons.Selected, clock)

        // textureId := buttonManager.GetButtonTextureId(textureManager, button.Text(), color)
        // width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, button.Text(), color)
        x += itemWidth + itemDistance
        _ = height
        if err != nil {
            return x, y, err
        }
    }

    return x, y, nil
}

type JoystickState interface {
}

type JoystickStateAdd struct {
    Index int
    // InstanceId sdl.JoystickID
    Name string
}

type JoystickStateRemove struct {
    Index int
    // InstanceId sdl.JoystickID
}

// callback that is invoked when MenuQuit is input
type MenuQuitFunc func(SubMenu) SubMenu

type StaticMenu struct {
    Buttons MenuButtons
    Quit MenuQuitFunc
    ExtraInfo string
    AudioManager AudioManager
}

func (menu *StaticMenu) PlayBeep() {
    menu.AudioManager.PlayBeep()
}

func (menu *StaticMenu) UpdateWindowSize(x int, y int){
    // nothing
}

func (menu *StaticMenu) Update(){
}

func (menu *StaticMenu) Input(input MenuInput) SubMenu {
    switch input {
        case MenuQuit:
            return menu.Quit(menu)
        default:
            return menu.Buttons.Interact(input, menu)
    }
}

func renderLines(screen *ebiten.Image, x float64, y float64, font text.Face, info string) (float64, float64, error) {
    aLength, height := text.Measure("A", font, 1)
    // white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

    for _, line := range strings.Split(info, "\n") {
        parts := strings.Split(line, "\t")
        var options text.DrawOptions
        options.GeoM.Translate(float64(x), float64(y))
        for i, part := range parts {
            options.GeoM.Translate(float64(i) * aLength * 20, 0)
            text.Draw(screen, part, font, &options)
            // gfx.WriteFont(font, renderer, x + i * aLength * 20, y, part, white)
        }
        y += height + 2
    }

    return x, y, nil
}

func (menu *StaticMenu) MakeRenderer(font text.Face, smallFont text.Face, clock uint64) gfx.RenderFunction {
    
    return func(screen *ebiten.Image) error {
        startX := float64(50)
        _, y, err := menu.Buttons.Render(startX, 50, font, screen, clock)
        // FIXME: handle err

        _, height := text.Measure("A", font, 1)

        x := startX
        y += height * 3

        _, _, err = renderLines(screen, x, y, smallFont, menu.ExtraInfo)
        return err
    }
}

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

        /* FIXME: just a test */
        // manager.Player1.SetExtraButton(common.EmulatorTurbo, &common.JoystickButton{Button: 5})

        err := manager.SaveInput()
        if err != nil {
            log.Printf("Warning: could not save joystick input: %v", err)
        }
    }
}

func inList(value string, array []string) bool {
    for _, x := range array {
        if x == value {
            return true
        }
    }

    return false
}

func (mapping *JoystickButtonMapping) AddAxisMapping(name string, axis JoystickAxisType){
    if inList(name, mapping.ButtonList()){
        mapping.Inputs[name] = &axis
    } else if inList(name, mapping.ExtraButtonList()){
        mapping.ExtraInputs[name] = &axis
    }
}

func (mapping *JoystickButtonMapping) AddButtonMapping(name string, button ebiten.GamepadButton){
    if inList(name, mapping.ButtonList()){
        mapping.Inputs[name] = &JoystickButtonType{
            Name: name,
            Pressed: false,
            Button: button,
       }
   } else if inList(name, mapping.ExtraButtonList()){
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
}

type JoystickButtonType struct {
    Button ebiten.GamepadButton
    Name string
    Pressed bool
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

func (menu *JoystickMenu) DoConfigure(joystick *common.JoystickButtons, yield coroutine.YieldFunc, buttonList []string) {

    for _, button := range buttonList {
        var lastTime time.Time
        lastButton := ebiten.GamepadButton(-1)
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
                break
            }

            if yield() != nil {
                return
            }
        }
    }

    menu.FinishConfigure()
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

func (menu *JoystickMenu) Update(){
    if menu.ConfigureCoroutine != nil {
        menu.ConfigureCoroutine.Run()
        if !menu.Configuring {
            menu.ConfigureCoroutine.Stop()
            menu.ConfigureCoroutine = nil
        }
    }
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

            width, height, err := drawButton(smallFont, screen, x, y, button, color)
            if err != nil {
                return err
            }
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

            width, height, err := drawButton(smallFont, screen, x, y, button, color)
            if err != nil {
                return err
            }
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

func forkJoystickInput(channel <-chan JoystickState) (<-chan JoystickState, <-chan JoystickState){
    /* FIXME: pass in the buffer size as an argument? */
    copy1 := make(chan JoystickState, 5)
    copy2 := make(chan JoystickState, 5)


    go func(){
        defer close(copy1)
        defer close(copy2)

        for input := range channel {
            copy1 <- input
            copy2 <- input
        }
    }()

    return copy1, copy2
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

    /* playstation 3 mapping */
    menu.Mapping.AddButtonMapping("Up", 13)
    menu.Mapping.AddButtonMapping("Down", 14)
    menu.Mapping.AddButtonMapping("Left", 15)
    menu.Mapping.AddButtonMapping("Right", 16)
    menu.Mapping.AddButtonMapping("A", 0) // X
    menu.Mapping.AddButtonMapping("B", 3) // square
    menu.Mapping.AddButtonMapping("Select", 8)
    menu.Mapping.AddButtonMapping("Start", 9)

    // copy1, copy2 := forkJoystickInput(joystickStateChanges)

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
        return menu
    }})
    menu.Buttons.Add(&SubMenuButton{Name: "Next Joystick", Func: func() SubMenu {
        joystickManager.NextJoystick()
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
                menu.DoConfigure(joystick, yield, menu.Mapping.ButtonList())
                return nil
            })
        }

        return menu
    }})

    return menu
}

type LoadRomMenu struct {
    Quit context.Context
    LoaderCancel context.CancelFunc
    MenuCancel context.CancelFunc
    Back MenuQuitFunc
    SelectRom func()
    LoaderState *RomLoaderState
    AudioManager AudioManager
}

func (loadRomMenu *LoadRomMenu) PlayBeep() {
    loadRomMenu.AudioManager.PlayBeep()
}

func (loadRomMenu *LoadRomMenu) TextInput(text string){
    text = strings.ReplaceAll(text, "-", "")
    text = strings.ReplaceAll(text, "=", "")
    loadRomMenu.LoaderState.SearchAdd(text)
}

func (loadRomMenu *LoadRomMenu) Update() {
    keys := inpututil.AppendJustPressedKeys(nil)
    for _, key := range keys {
        loadRomMenu.KeyDown(key)
    }

    runes := ebiten.AppendInputChars(nil)
    if len(runes) > 0 {
        loadRomMenu.TextInput(string(runes))
    }
}

func (loadRomMenu *LoadRomMenu) KeyDown(key ebiten.Key){
    switch key {
        case ebiten.KeyBackspace:
            loadRomMenu.LoaderState.SearchBackspace()
        case ebiten.KeyMinus:
            loadRomMenu.LoaderState.ZoomOut()
        case ebiten.KeyEqual:
            loadRomMenu.LoaderState.ZoomIn()
    }
}

func (loadRomMenu *LoadRomMenu) Input(input MenuInput) SubMenu {
    switch input {
        case MenuNext:
            loadRomMenu.LoaderState.NextSelection()
            loadRomMenu.PlayBeep()
            return loadRomMenu
        case MenuPrevious:
            loadRomMenu.LoaderState.PreviousSelection()
            loadRomMenu.PlayBeep()
            return loadRomMenu
        case MenuUp:
            loadRomMenu.LoaderState.PreviousUpSelection()
            loadRomMenu.PlayBeep()
            return loadRomMenu
        case MenuDown:
            loadRomMenu.LoaderState.NextDownSelection()
            loadRomMenu.PlayBeep()
            return loadRomMenu
        case MenuQuit:
            loadRomMenu.LoaderCancel()
            return loadRomMenu.Back(loadRomMenu)
        case MenuSelect:
            info, ok := loadRomMenu.LoaderState.GetSelectedRomInfo()
            if ok {
                var size int64 = 0
                stat, err := os.Stat(info.Path)
                if err == nil {
                    size = stat.Size()
                }

                mapper := -1
                nesFile, err := nes.ParseNesFile(info.Path, false)
                if err == nil {
                    mapper = int(nesFile.Mapper)
                }

                return &LoadRomInfoMenu{
                    RomLoader: loadRomMenu,
                    Mapper: mapper,
                    Info: info,
                    Filesize: size,
                }
            } else {
                return loadRomMenu
            }
        default:
            return loadRomMenu
    }
}

func (loadRomMenu *LoadRomMenu) MakeRenderer(font text.Face, smallFont text.Face, clock uint64) gfx.RenderFunction {
    return func(out *ebiten.Image) error {
        return loadRomMenu.LoaderState.Render(font, smallFont, out)
    }
}

func (loadRomMenu *LoadRomMenu) UpdateWindowSize(x int, y int){
    loadRomMenu.LoaderState.UpdateWindowSize(x, y)
}

/* displays info about a specific rom in the rom loader and gives the user a choice to actually load the rom or not */
type LoadRomInfoMenu struct {
    RomLoader *LoadRomMenu // the previous load rom menu
    Selection int
    Filesize int64
    Mapper int
    Info *RomLoaderInfo
}

const (
    LoadRomInfoSelect = iota
    LoadRomInfoBack
)

func (loader *LoadRomInfoMenu) Update(){
}

func (loader *LoadRomInfoMenu) Input(input MenuInput) SubMenu {
    inputs := 2
    switch input {
        case MenuNext:
            loader.Selection = (loader.Selection + 1) % inputs
            loader.PlayBeep()
            return loader
        case MenuPrevious:
            loader.Selection = (loader.Selection - 1 + inputs) % inputs
            loader.PlayBeep()
            return loader
        case MenuUp:
            loader.Selection = (loader.Selection - 1 + inputs) % inputs
            loader.PlayBeep()
            return loader
        case MenuDown:
            loader.Selection = (loader.Selection + 1) % inputs
            loader.PlayBeep()
            return loader
        case MenuQuit:
            return loader.RomLoader
        case MenuSelect:
            switch loader.Selection {
                case LoadRomInfoSelect:
                    loader.RomLoader.SelectRom()
                    return loader.RomLoader
                case LoadRomInfoBack:
                    return loader.RomLoader
                default:
                    return loader.RomLoader
            }
        default:
            return loader
    }
}

func (loader *LoadRomInfoMenu) GetSelectionColor(use int) color.Color {
    white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
    yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}
    if use == loader.Selection {
        return yellow
    }
    return white
}

/* convert a number into a human readable string, like 2100 => 2kb */
func niceSize(size int64) string {
    last := "b"
    if size > 1024 {
        size /= 1024
        last = "kb"
    }
    if size > 1024 {
        size /= 1024
        last = "mb"
    }
    if size > 1024 {
        size /= 1024
        last = "gb"
    }

    return fmt.Sprintf("%v%v", size, last)
}

func (loader *LoadRomInfoMenu) MakeRenderer(font text.Face, smallFont text.Face, clock uint64) gfx.RenderFunction {
    old := loader.RomLoader.MakeRenderer(font, smallFont, clock)

    fontWidth, fontHeight := text.Measure("A", font, 1)
    _ = fontWidth

    return func(out *ebiten.Image) error {
        // render the rom loader in the background
        err := old(out)
        if err != nil {
            return err
        }

        // margin = 5%
        maxWidth := float32(out.Bounds().Dx())
        maxHeight := float32(out.Bounds().Dy())
        marginX := maxWidth * 5 / 100
        marginY := maxHeight * 5 / 100
        margin := marginY
        if marginX < marginY {
            margin = marginX
        }

        white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

        vector.FillRect(out, margin, margin, maxWidth - margin*2, maxHeight - margin*2, color.NRGBA{A: 240}, false)
        vector.StrokeRect(out, margin, margin, maxWidth - margin*2, maxHeight - margin*2, 1, white, false)

        x := margin + 5
        y := margin + 5

        maxX := maxWidth - margin * 2
        maxY := maxHeight - margin * 2
        _ = maxY

        thumbnail := maxWidth * 50 / 100
        if thumbnail > maxX - x {
            thumbnail = maxX - x
        }

        textY := y
        textX := x
        var textOptions text.DrawOptions
        textOptions.GeoM.Translate(float64(textX), float64(textY))

        text.Draw(out, filepath.Base(loader.Info.Path), font, &textOptions)

        textY += float32(fontHeight + 2)

        textOptions.GeoM.Translate(0, fontHeight + 2)
        text.Draw(out, fmt.Sprintf("File size: %v", niceSize(loader.Filesize)), font, &textOptions)
        textOptions.GeoM.Translate(0, fontHeight + 2)

        if loader.Mapper == -1 {
            text.Draw(out, "Mapper: unknown", font, &textOptions)
        } else {
            text.Draw(out, fmt.Sprintf("Mapper: %v", loader.Mapper), font, &textOptions)
        }

        frame, ok := loader.Info.GetFrame()
        if ok {
            width := frame.Bounds().Dx()
            // height := frame.Bounds().Dy()

            divider := float32(width) / float32(thumbnail)

            // overscanPixels := 0

            var draw ebiten.DrawImageOptions
            draw.GeoM.Scale(float64(1/divider), float64(1/divider))
            draw.GeoM.Translate(float64(maxX - thumbnail - 2), float64(y+10))
            out.DrawImage(frame, &draw)
        }

        yPos := float64(maxY) - fontHeight * 4
        textOptions.GeoM.Reset()
        textOptions.GeoM.Translate(float64(x), yPos)
        textOptions.ColorScale.ScaleWithColor(loader.GetSelectionColor(LoadRomInfoSelect))
        text.Draw(out, "Load rom", font, &textOptions)

        textOptions.GeoM.Translate(0, fontHeight + 2)
        textOptions.ColorScale.Reset()
        textOptions.ColorScale.ScaleWithColor(loader.GetSelectionColor(LoadRomInfoBack))
        text.Draw(out, "Back", font, &textOptions)

        return nil
    }
}

func (loader *LoadRomInfoMenu) PlayBeep() {
    loader.RomLoader.PlayBeep()
}

func (loader *LoadRomInfoMenu) UpdateWindowSize(x int, y int){
    loader.RomLoader.UpdateWindowSize(x, y)
}

func keysInfo(keys *common.EmulatorKeys) string {
    n := ebiten.Key.String

    info := template.New("keys")

    info.Funcs(map[string]any{
        "n": n,
    })
    _, err := info.Parse(`Keys:
A: {{n .ButtonA}}{{"\t"}}Turbo: {{n .Turbo}}
B: {{n .ButtonB}}{{"\t"}}Pause: {{n .Pause}}
Start: {{n .ButtonStart}}{{"\t"}}Hard Reset: {{n .HardReset}}
TurboA: {{n .ButtonTurboA}}{{"\t"}}PPU Debug: {{n .PPUDebug}}
TurboB: {{n .ButtonTurboB}}{{"\t"}}Slow down: {{n .SlowDown}}
Select: {{n .ButtonSelect}}{{"\t"}}Speed up: {{n .SpeedUp}}
Start: {{n .ButtonStart}}{{"\t"}}Normal: {{n .Normal}}
Up: {{n .ButtonUp}}{{"\t"}}Step frame: {{n .StepFrame}}
Down: {{n .ButtonDown}}{{"\t"}}Record: {{n .Record}}
Left: {{n .ButtonLeft}}{{"\t"}}Save state: {{n .SaveState}}
Right: {{n .ButtonRight}}{{"\t"}}Load state: {{n .LoadState}}
{{"\t"}}Console: {{n .Console}}
{{"\t"}}Menu: ESC
`)

    if err != nil {
        log.Printf("Could not parse template: %v", err)
        return ""
    }

    var data bytes.Buffer

    err = info.Execute(&data, keys)
    if err != nil {
        return ""
    }
    return data.String()
}

type ChangeKeyMenu struct {
    MenuQuit context.Context
    Quit MenuQuitFunc
    Buttons MenuButtons
    ExtraInfo string
    // Chooser *ChooseButton
    Choosing bool
    ChoosingKey string
    ChoosingButton *StaticFixedWidthButton
    Current uint64
    /* show a warning if the user is choosing a key that is already in use */
    Warning string

    ChooseDone context.Context
    ChooseCancel context.CancelFunc

    TempChoice ebiten.Key
    LastTime time.Time

    AudioManager AudioManager

    Keys *common.EmulatorKeys
}

func (menu *ChangeKeyMenu) PlayBeep() {
    menu.AudioManager.PlayBeep()
}

func (menu *ChangeKeyMenu) UpdateWindowSize(x int, y int){
}

/*
func (menu *ChangeKeyMenu) RawInput(event sdl.Event){
    if menu.IsChoosing() {
        key, ok := event.(*sdl.KeyboardEvent)
        if ok {
            switch key.GetType() {
                case sdl.KEYDOWN:
                    code := key.Keysym.Sym

                    / * check if the user pressed a new key. if they pressed the same key that is being changed then don't do anything * /
                    if code != menu.TempChoice {

                        menu.ChooseCancel()
                        menu.Lock.Lock()
                        menu.ChooseDone, menu.ChooseCancel = context.WithCancel(menu.MenuQuit)

                        // log.Printf("Change key %v", code)
                        choosingKey := menu.ChoosingKey
                        menu.TempChoice = code
                        menu.Current = 0

                        for _, check := range menu.Keys.AllKeys() {
                            if check.Name != menu.ChoosingKey && code == check.Code {
                                menu.Warning = fmt.Sprintf("%v already in use", check.Name)
                            }
                        }

                        menu.Lock.Unlock()


                        go func(done context.Context){
                            xtime := time.NewTicker(time.Second / 10)
                            defer xtime.Stop()
                            after := time.After(500 * time.Millisecond)
                            for {
                                select {
                                    case <-xtime.C:
                                        menu.Lock.Lock()
                                        menu.Current += 1
                                        menu.Lock.Unlock()
                                    case <-done.Done():
                                        return
                                    case <-after:
                                        menu.Lock.Lock()
                                        menu.TempChoice = 0
                                        menu.Current = 0
                                        menu.ChooseCancel()
                                        menu.Keys.Update(choosingKey, code)
                                        name := sdl.GetKeyName(code)
                                        menu.ChoosingButton.Update(choosingKey, name)
                                        common.SaveEmulatorKeys(*menu.Keys)
                                        menu.Lock.Unlock()

                                        menu.SetChoosing(false, "", nil)
                                        return
                                }
                            }
                        }(menu.ChooseDone)
                    }
                case sdl.KEYUP:
                    menu.ChooseCancel()
                    menu.Lock.Lock()
                    menu.TempChoice = 0
                    menu.Warning = ""
                    menu.Lock.Unlock()
            }
        }
    }
}
*/

func (menu *ChangeKeyMenu) SetChoosing(v bool, key string, button *StaticFixedWidthButton){
    menu.Choosing = v
    menu.ChoosingKey = key
    menu.ChoosingButton = button
    menu.Warning = ""
}

func (menu *ChangeKeyMenu) IsChoosing() bool {
    return menu.Choosing
}

func (menu *ChangeKeyMenu) Update(){
    if menu.IsChoosing() {
        keys := inpututil.AppendJustPressedKeys(nil)
        for _, key := range keys {
            if key != menu.TempChoice {
                menu.TempChoice = key
                menu.LastTime = time.Now()

                for _, check := range menu.Keys.AllKeys() {
                    if check.Name != menu.ChoosingKey && key == check.Code {
                        menu.Warning = fmt.Sprintf("%v already in use", check.Name)
                    }
                }

            }
        }

        keys = inpututil.AppendJustReleasedKeys(nil)
        for _, key := range keys {
            if key == menu.TempChoice {
                menu.TempChoice = ebiten.Key(-1)
                menu.LastTime = time.Time{}
                menu.Warning = ""
            }
        }

        if !menu.LastTime.IsZero() && time.Since(menu.LastTime) >= 700 * time.Millisecond {
            menu.Keys.Update(menu.ChoosingKey, menu.TempChoice)
            menu.ChoosingButton.Update(menu.ChoosingKey, menu.TempChoice.String())
            common.SaveEmulatorKeys(*menu.Keys)
            menu.SetChoosing(false, "", nil)
        }
    }
}

func (menu *ChangeKeyMenu) Input(input MenuInput) SubMenu {
    switch input {
        case MenuQuit:
            if menu.IsChoosing() {
                menu.ChooseCancel()
                menu.SetChoosing(false, "", nil)
                return menu
            }
            return menu.Quit(menu)
        default:
            if menu.IsChoosing() {
                return menu
            }
            return menu.Buttons.Interact(input, menu)
    }
}

func (menu *ChangeKeyMenu) MakeRenderer(font text.Face, smallFont text.Face, clock uint64) gfx.RenderFunction {
    _, fontHeight := text.Measure("A", font, 1)

    wideWidth, _ := text.Measure(strings.Repeat("A", 40), font, 1)

    return func(out *ebiten.Image) error {
        bounds := out.Bounds()
        maxWidth := bounds.Dx()
        maxHeight := bounds.Dy()

        startX := 50
        _, y, err := menu.Buttons.Render(float64(startX), 50, font, out, clock)

        _ = err

        x := startX
        y += fontHeight * 3

        _, _, err = renderLines(out, float64(x), y, smallFont, menu.ExtraInfo)
        // FIXME: handle err

        if menu.IsChoosing() {
            yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}
            red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
            white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

            line := fmt.Sprintf("Press a key to set %v", menu.ChoosingKey)

            midX := float64(maxWidth / 2)
            midY := float64(maxHeight / 2)

            margin := fontHeight * 3
            x1 := midX - wideWidth / 2 - margin
            y1 := midY - fontHeight / 2 - margin
            x2 := midX + fontHeight / 2 + margin
            y2 := midY + fontHeight / 2 + margin

            vector.FillRect(out, float32(x1), float32(y1), float32(x2 - x1), float32(y2 - y1), color.NRGBA{R: 5, G: 5, B: 5, A: 230}, false)
            vector.StrokeRect(out, float32(x1), float32(y1), float32(x2 - x1), float32(y2 - y1), 1, white, false)

            textX := midX - wideWidth / 2
            textY := midY - fontHeight / 2
            var textOptions text.DrawOptions
            textOptions.GeoM.Translate(textX, textY)
            
            text.Draw(out, line, font, &textOptions)
            // gfx.WriteFont(font, renderer, midX - width / 2, midY - height / 2, line, white)

            textY += fontHeight + 2

            tempChoice := menu.TempChoice
            // current := menu.Current
            warning := menu.Warning

            textOptions.GeoM.Translate(0, fontHeight + 2)
            textOptions.ColorScale.ScaleWithColor(gfx.InterpolateColor(red, yellow, 15, int(time.Since(menu.LastTime)/time.Millisecond / (700 / 15))))
            text.Draw(out, tempChoice.String(), font, &textOptions)
            //
            // gfx.WriteFont(font, renderer, textX, textY, sdl.GetKeyName(tempChoice), gfx.Glow(red, yellow, 15, current))

            textY += fontHeight + 2

            if warning != "" {
                textOptions.GeoM.Translate(0, fontHeight + 2)
                textOptions.ColorScale.Reset()
                text.Draw(out, warning, font, &textOptions)
                // gfx.WriteFont(font, renderer, textX, textY, warning, white)
            }
        }

        return nil
    }
}

type ChooseButton struct {
    Enabled bool
    Lock sync.Mutex
    Items []string
    Choice int
}

func (choose *ChooseButton) Text() string {
    choose.Lock.Lock()
    defer choose.Lock.Unlock()
    return choose.Items[choose.Choice]
}

func (choose *ChooseButton) Next() {
    choose.Lock.Lock()
    defer choose.Lock.Unlock()
    choose.Choice = (choose.Choice + 1) % len(choose.Items)
}

func (choose *ChooseButton) Previous() {
    choose.Lock.Lock()
    defer choose.Lock.Unlock()
    choose.Choice -= 1
    if choose.Choice < 0 {
        choose.Choice += len(choose.Items)
    }
}

func (choose *ChooseButton) Interact(menu SubMenu) SubMenu {
    return menu
}

func (choose *ChooseButton) Render(font text.Face, out *ebiten.Image, x float64, y float64, selected bool, clock uint64) (float64, float64, error) {
    // FIXME
    if choose.IsEnabled() {

        size := 10.0
        // gfx.DrawEquilateralTriange(renderer, x-size*2, y + size + font.Height() / 4, float64(size), 180.0, sdl.Color{R: 255, G: 255, B: 255, A: 255})
        width, height, err := _doRenderButton(choose, font, out, x, y, selected, clock)
        x += width
        _ = height
        // gfx.DrawEquilateralTriange(renderer, x+size*2, y + size + font.Height() / 4, float64(size), 0.0, sdl.Color{R: 255, G: 255, B: 255, A: 255})

        _, fontHeight := text.Measure("A", font, 1)

        return x + size*2 + size*2, fontHeight, err
    } else {
        return x, y, nil
    }
}

func (choose *ChooseButton) IsEnabled() bool {
    choose.Lock.Lock()
    defer choose.Lock.Unlock()
    return choose.Enabled
}

func (choose *ChooseButton) SetEnabled(v bool){
    choose.Lock.Lock()
    defer choose.Lock.Unlock()
    choose.Enabled = v
}

func (choose *ChooseButton) Toggle() bool {
    choose.SetEnabled(!choose.IsEnabled())
    return choose.IsEnabled()
}

func (choose *ChooseButton) Disable() {
    choose.SetEnabled(false)
}

func (choose *ChooseButton) Enable() {
    choose.SetEnabled(true)
}

func MakeKeysMenu(menu *Menu, parentMenu SubMenu, update func(common.EmulatorKeys), keys *common.EmulatorKeys) SubMenu {

    /*
    var items []string

    for _, key := range keys.AllKeys() {
        items = append(items, fmt.Sprintf("%v: %v", key.Name, key.Code.String()))
    }
    */

    // chooseButton := &ChooseButton{Items: items}

    chooseDone, chooseCancel := context.WithCancel(menu.quit)

    keyMenu := &ChangeKeyMenu{
        MenuQuit: menu.quit,
        Quit: func(current SubMenu) SubMenu {
            update(*keys)
            return parentMenu
        },
        AudioManager: menu.AudioManager,
        // ExtraInfo: keysInfo(keys),
        // Beep: menu.Beep,
        ChooseDone: chooseDone,
        ChooseCancel: chooseCancel,
        // Chooser: chooseButton,
        Choosing: false,
        Keys: keys,
    }

    back := &SubMenuButton{Name: "Back", Func: func() SubMenu { return parentMenu } }
    keyMenu.Buttons.Add(back)

    changeButtons := make(map[string]*StaticFixedWidthButton)
    for _, key := range keys.AllKeys() {
        name := key.Name
        code := key.Code

        button := &StaticFixedWidthButton{
            Width: 200,
            Parts: []string{name, code.String()},
            // Name: fmt.Sprintf("%v: %v", name, sdl.GetKeyCode(code)),
            Func: func(self *StaticFixedWidthButton){
                keyMenu.SetChoosing(true, name, self)
            },
        }

        changeButtons[name] = button
    }

    defaults := &StaticButton{
        Name: "Reset to defaults",
        Func: func(self *StaticButton){
            keyMenu.Keys.UpdateAll(common.DefaultEmulatorKeys())
            common.SaveEmulatorKeys(*keyMenu.Keys)

            for _, key := range keyMenu.Keys.AllKeys() {
                button := changeButtons[key.Name]
                button.Update(fmt.Sprintf("%v: %v", key.Name, key.Code.String()))
            }
        },
    }

    keyMenu.Buttons.Add(defaults)

    /*
    keyMenu.Buttons.Add(&StaticButton{Name: "Change key", Func: func(){
        if chooseButton.Toggle() {
            keyMenu.Buttons.Select(chooseButton)
        } else {
            keyMenu.Buttons.Select(back)
        }
    }})
    */

    keyMenu.Buttons.Add(&MenuNextLine{})
    keyMenu.Buttons.Add(&MenuLabel{Label: "Select a key to change", Color: color.RGBA{R: 255, G: 255, B: 0, A: 255}})
    keyMenu.Buttons.Add(&MenuNextLine{})

    // keyMenu.Buttons.Add(&MenuSpace{Space: 60})
    // keyMenu.Buttons.Add(chooseButton)

    count := 0
    for _, key := range keys.AllKeys() {
        name := key.Name
        button := changeButtons[name]
        keyMenu.Buttons.Add(button)
        count += 1
        if count % 2 == 0 {
            keyMenu.Buttons.Add(&MenuNextLine{})
        }
    }

    return keyMenu
}

func MakeMainMenu(menu *Menu, mainCancel context.CancelFunc, programActions ProgramActions, joystickStateChanges <-chan JoystickState, joystickManager *common.JoystickManager, keys *common.EmulatorKeys) SubMenu {
    main := &StaticMenu{
        Quit: func(current SubMenu) SubMenu {
            /* quit the entire menu system if the user presses escape at the top level */
            menu.cancel()
            return current
        },
        AudioManager: menu.AudioManager,
    }

    joystickMenu := MakeJoystickMenu(main, joystickStateChanges, joystickManager, menu.AudioManager)

    main.Buttons.Add(&StaticButton{Name: "Quit", Func: func(button *StaticButton){
        mainCancel()
    }})

    main.Buttons.Add(&SubMenuButton{Name: "Load ROM", Func: func() SubMenu {
        loadRomQuit, loadRomCancel := context.WithCancel(menu.quit)

        romLoaderState := MakeRomLoaderState(loadRomQuit, 1, 1)
        go romLoader(loadRomQuit, romLoaderState)

        return &LoadRomMenu{
            Back: func(current SubMenu) SubMenu {
                return main
            },
            SelectRom: func(){
                romName, romFile, ok := romLoaderState.GetSelectedRom()
                if ok {
                    menu.cancel()
                    programActions.LoadRom(romName, romFile)
                }
            },
            Quit: loadRomQuit,
            LoaderCancel: loadRomCancel,
            MenuCancel: menu.cancel,
            LoaderState: romLoaderState,
            AudioManager: menu.AudioManager,
        }
    }})

    main.Buttons.Add(&ToggleButton{
        State1: "Sound enabled",
        State2: "Sound disabled",
        state: programActions.IsSoundEnabled(),
        Func: func(value bool){
            log.Printf("Set sound to %v", value)
            programActions.SetSoundEnabled(value)
        },
    })

    /* FIXME: this callback to update ExtraInfo feels a bit hacky */
    keysMenu := MakeKeysMenu(menu, main, func (newKeys common.EmulatorKeys){
        main.ExtraInfo = keysInfo(&newKeys)
    }, keys)

    main.Buttons.Add(&SubMenuButton{Name: "Keys", Func: func() SubMenu {
        return keysMenu
    }})

    main.Buttons.Add(&SubMenuButton{Name: "Joystick", Func: func() SubMenu { return joystickMenu } })

    main.ExtraInfo = keysInfo(keys)

    return main
}

type MenuRenderLayer struct {
    Renderer func(screen *ebiten.Image) error
    Index int
}

func (layer *MenuRenderLayer) Render(info gfx.RenderInfo) error {
    return layer.Renderer(info.Screen)
}

func (layer *MenuRenderLayer) ZIndex() int {
    return layer.Index
}

type DrawManager interface {
    PushDraw(func(*ebiten.Image), bool)
    PopDraw()
    GetWindowSize() common.WindowSize
}

func (menu *Menu) Run(mainCancel context.CancelFunc, font text.Face, smallFont text.Face, programActions ProgramActions, joystickManager *common.JoystickManager, emulatorKeys *common.EmulatorKeys, yield coroutine.YieldFunc, drawManager DrawManager){
    userInput := make(chan MenuInput, 3)
    defer close(userInput)

    joystickStateChanges := make(chan JoystickState, 3)
    defer close(joystickStateChanges)

    /*
    eventFunction := func(){
        event := sdl.WaitEventTimeout(1)
        if event != nil {
            select {
                case rawEvents <- event:
                default:
                    log.Printf("Warning: dropping raw sdl event\n")
            }

            // log.Printf("Event %+v type %v\n", event)
            switch event.GetType() {
                case sdl.JOYDEVICEADDED:
                    add_event := event.(*sdl.JoyDeviceAddedEvent)
                    joystickStateChanges <- &JoystickStateAdd{
                        Index: int(add_event.Which),
                        InstanceId: add_event.Which,
                        Name: strings.TrimSpace(sdl.JoystickNameForIndex(int(add_event.Which))),
                    }
                case sdl.JOYDEVICEREMOVED:
                    remove_event := event.(*sdl.JoyDeviceRemovedEvent)
                    joystickStateChanges <- &JoystickStateRemove{
                        Index: int(remove_event.Which),
                        InstanceId: remove_event.Which,
                    }
                case sdl.DROPFILE:
                    drop_event := event.(*sdl.DropEvent)
                    switch drop_event.Type {
                        case sdl.DROPFILE:
                            menu.cancel()
                            open := func() (fs.File, error) {
                                return os.Open(drop_event.File)
                            }
                            programActions <- &common.ProgramLoadRom{Name: drop_event.File, File: open}
                        case sdl.DROPBEGIN:
                            log.Printf("drop begin '%v'\n", drop_event.File)
                        case sdl.DROPCOMPLETE:
                            log.Printf("drop complete '%v'\n", drop_event.File)
                        case sdl.DROPTEXT:
                            log.Printf("drop text '%v'\n", drop_event.File)
                    }
            }
        }
    }
    */

    var snow []Snow

    renderSnow := func(out *ebiten.Image) error {
        for _, snow := range snow {
            c := snow.color
            vector.FillCircle(out, snow.x, snow.y, float32(1), color.NRGBA{R: c, G: c, B: c, A: 255}, false)
        }
        return nil
    }

    wind := (rand.Float32() - 0.5) / 2

    updateSnow := func(windowSize common.WindowSize){
        if len(snow) < 300 {
            snow = append(snow, MakeSnow(windowSize.X))
        }

        maxWind := float32(0.8)

        wind += (rand.Float32() - 0.5) / 6
        if wind < -maxWind {
            wind = -maxWind
        }
        if wind > maxWind {
            wind = maxWind
        }

        for i := range snow {
            snow[i].truey += snow[i].fallSpeed
            snow[i].truex += wind
            snow[i].x = snow[i].truex + float32(math.Cos(float64(snow[i].angle + 180) * math.Pi / 180.0) * 8)
            // snow[i].y = snow[i].truey + float32(-math.Sin(float64(snow[i].angle + 180) * math.Pi / 180.0) * 8)
            snow[i].y = snow[i].truey
            snow[i].angle += float32(snow[i].direction) * snow[i].speed

            if snow[i].y > float32(windowSize.Y) {
                snow[i] = MakeSnow(windowSize.X)
            }

            if snow[i].angle < 0 {
                snow[i].angle = 0
                snow[i].direction = -snow[i].direction
            }
            if snow[i].angle >= 180  {
                snow[i].angle = 180
                snow[i].direction = -snow[i].direction
            }

            newColor := int(snow[i].color) + rand.N(11) - 5
            if newColor > 255 {
                newColor = 255
            }
            if newColor < 40 {
                newColor = 40
            }

            snow[i].color = uint8(newColor)
        }
    }

    renderInfo := func(out *ebiten.Image) error {
        // white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

        maxWidth, maxHeight := out.Bounds().Dx(), out.Bounds().Dy()

        var drawOptions text.DrawOptions

        _, height := text.Measure("A", smallFont, 1)

        drawOptions.GeoM.Translate(float64(maxWidth - 130), float64(maxHeight) - height * 3)
        text.Draw(out, "NES Emulator", smallFont, &drawOptions)
        drawOptions.GeoM.Translate(0, float64(height + 3))
        text.Draw(out, "Jon Rafkind", smallFont, &drawOptions)

        return nil
    }

    var clock uint64 = 0

    currentMenu := MakeMainMenu(menu, mainCancel, programActions, joystickStateChanges, joystickManager, emulatorKeys)

    draw := func(screen *ebiten.Image){
        /* Draw a reddish overlay on the screen */
        vector.FillRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), color.NRGBA{R: 32, G: 0, B: 0, A: 210}, false)

        renderSnow(screen)
        renderInfo(screen)

        currentMenu.MakeRenderer(font, smallFont, clock)(screen)
    }

    drawManager.PushDraw(draw, true)
    defer drawManager.PopDraw()

    /* Reset the default renderer */
    for menu.quit.Err() == nil {
        clock += 1

        keys := inpututil.AppendJustPressedKeys(nil)
        for _, key := range keys {
            switch key {
                case ebiten.KeyEscape, ebiten.KeyCapsLock:
                    currentMenu = currentMenu.Input(MenuQuit)
                case ebiten.KeyLeft, ebiten.KeyH:
                    currentMenu = currentMenu.Input(MenuPrevious)
                case ebiten.KeyRight, ebiten.KeyL:
                    currentMenu = currentMenu.Input(MenuNext)
                case ebiten.KeyUp, ebiten.KeyK:
                    currentMenu = currentMenu.Input(MenuUp)
                case ebiten.KeyDown, ebiten.KeyJ:
                    currentMenu = currentMenu.Input(MenuDown)
                case ebiten.KeyEnter:
                    currentMenu = currentMenu.Input(MenuSelect)
            }
        }

        currentMenu.Update()

        windowSize := drawManager.GetWindowSize()
        currentMenu.UpdateWindowSize(windowSize.X, windowSize.Y)

        updateSnow(windowSize)

        if yield() != nil {
            return
        }
    }

    /*
    sdl.Do(func(){
        // log.Printf("Found joysticks: %v\n", sdl.NumJoysticks())
        for i := 0; i < sdl.NumJoysticks(); i++ {
            // guid := sdl.JoystickGetDeviceGUID(i)
            // log.Printf("Joystick %v: %v = %v\n", i, guid, sdl.JoystickNameForIndex(i))

            joystickStateChanges <- &JoystickStateAdd{
                Index: i,
                Name: strings.TrimSpace(sdl.JoystickNameForIndex(i)),
            }
        }
    })
    */
}

func (menu *Menu) Close() {
    menu.cancel()
}

package menu

/*
#include <stdlib.h>
*/
import "C"
import (
    "context"

    "os"
    "fmt"
    "math"
    "math/rand"
    "time"
    "bytes"
    "log"
    "sync"
    "strings"

    "crypto/md5"

    "image"
    "image/png"
    "golang.org/x/image/bmp"

    "github.com/kazzmir/nes/cmd/nes/common"
    nes "github.com/kazzmir/nes/lib"

    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
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
    font *ttf.Font
    Input chan MenuInput
    Lock sync.Mutex
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
    x := rand.Float32() * float32(screenWidth)
    // y := rand.Float32() * 400
    y := float32(0)
    return Snow{
        color: uint8(rand.Int31n(210) + 40),
        x: x,
        y: y,
        truex: x,
        truey: y,
        angle: rand.Float32() * 180,
        direction: 1,
        speed: rand.Float32() * 4 + 1,
        fallSpeed: rand.Float32() * 2.5 + 0.8,
    }
}

/* FIXME: good use-case for generics */
func copySnow(snow []Snow) []Snow {
    out := make([]Snow, len(snow))
    copy(out, snow)
    return out
}

/* We could juse use sdl.Texture.Query() to get the width/height. The downsides
 * of doing that are that it involves an extra cgo call.
 */
type TextureInfo struct {
    Texture *sdl.Texture
    Width int
    Height int
}

type TextureId uint64

type TextureManager struct {
    id TextureId
    Textures map[TextureId]TextureInfo
    Lock sync.Mutex
}

func (manager *TextureManager) NextId() TextureId {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()
    out := manager.id
    manager.id += 1
    return out
}

func MakeTextureManager() *TextureManager {
    return &TextureManager{
        id: 1, // so that clients can test if their texture id is 0, which means invalid
        Textures: make(map[TextureId]TextureInfo),
    }
}

func (manager *TextureManager) Destroy() {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    for _, info := range manager.Textures {
        info.Texture.Destroy()
    }

    manager.Textures = nil
}

var TextureManagerDestroyed = fmt.Errorf("texture manager has been destroyed")

type TextureMaker func() (TextureInfo, error)

func (manager *TextureManager) GetCachedTexture(id TextureId, makeTexture TextureMaker) (TextureInfo, error) {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    if manager.Textures == nil {
        return TextureInfo{}, TextureManagerDestroyed
    }

    info, ok := manager.Textures[id]
    if ok {
        return info, nil
    }

    var err error
    info, err = makeTexture()
    if err != nil {
        return TextureInfo{}, err
    }

    manager.Textures[id] = info

    return info, nil
}

func textWidth(font *ttf.Font, text string) int {
    /* FIXME: this feels a bit inefficient, maybe find a better way that doesn't require fully rendering the text */
    surface, err := font.RenderUTF8Solid(text, sdl.Color{R: 255, G: 255, B: 255, A: 255})
    if err != nil {
        return 0
    }

    defer surface.Free()
    return int(surface.W)
}

func (manager *TextureManager) RenderText(font *ttf.Font, renderer *sdl.Renderer, text string, color sdl.Color, id TextureId) (TextureInfo, error) {
    return manager.GetCachedTexture(id, func() (TextureInfo, error){
        surface, err := font.RenderUTF8Blended(text, color)
        if err != nil {
            return TextureInfo{}, err
        }

        defer surface.Free()

        texture, err := renderer.CreateTextureFromSurface(surface)
        if err != nil {
            return TextureInfo{}, err
        }

        bounds := surface.Bounds()

        info := TextureInfo{
            Texture: texture,
            Width: bounds.Max.X,
            Height: bounds.Max.Y,
        }

        return info, nil
    })
}

func drawButton(font *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager, textureId TextureId, x int, y int, message string, color sdl.Color) (int, int, error) {
    buttonInside := sdl.Color{R: 64, G: 64, B: 64, A: 255}
    buttonOutline := sdl.Color{R: 32, G: 32, B: 32, A: 255}

    info, err := textureManager.RenderText(font, renderer, message, color, textureId)
    if err != nil {
        return 0, 0, err
    }

    margin := 12

    renderer.SetDrawColor(buttonOutline.R, buttonOutline.G, buttonOutline.B, buttonOutline.A)
    renderer.FillRect(&sdl.Rect{X: int32(x), Y: int32(y), W: int32(info.Width + margin), H: int32(info.Height + margin)})

    renderer.SetDrawColor(buttonInside.R, buttonInside.G, buttonInside.B, buttonInside.A)
    renderer.FillRect(&sdl.Rect{X: int32(x+1), Y: int32(y+1), W: int32(info.Width + margin - 3), H: int32(info.Height + margin - 3)})

    err = copyTexture(info.Texture, renderer, info.Width, info.Height, x + margin/2, y + margin/2)

    return info.Width, info.Height, err
}

func copyTexture(texture *sdl.Texture, renderer *sdl.Renderer, width int, height int, x int, y int) error {
    sourceRect := sdl.Rect{X: 0, Y: 0, W: int32(width), H: int32(height)}
    destRect := sourceRect
    destRect.X = int32(x)
    destRect.Y = int32(y)

    return renderer.Copy(texture, &sourceRect, &destRect)
}

func writeFont(font *ttf.Font, renderer *sdl.Renderer, x int, y int, message string, color sdl.Color) error {
    surface, err := font.RenderUTF8Blended(message, color)
    if err != nil {
        return err
    }

    defer surface.Free()

    texture, err := renderer.CreateTextureFromSurface(surface)
    if err != nil {
        return err
    }
    defer texture.Destroy()

    surfaceBounds := surface.Bounds()

    return copyTexture(texture, renderer, surfaceBounds.Max.X, surfaceBounds.Max.Y, x, y)
}

func writeFontCached(font *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager, id TextureId, x int, y int, message string, color sdl.Color) error {
    info, err := textureManager.RenderText(font, renderer, message, color, id)
    if err != nil {
        return err
    }
    return copyTexture(info.Texture, renderer, info.Width, info.Height, x, y)
}

type MenuState int
const (
    MenuStateTop = iota
    MenuStateLoadRom
)

func chainRenders(functions ...common.RenderFunction) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {
        for _, f := range functions {
            err := f(renderer)
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

func doRender(width int, height int, raw_pixels []byte, destX int, destY int, destWidth int, destHeight int, pixelFormat common.PixelFormat, renderer *sdl.Renderer) error {
    pixels := C.CBytes(raw_pixels)
    defer C.free(pixels)

    depth := 8 * 4 // RGBA8888
    pitch := int(width) * int(depth) / 8

    // pixelFormat := sdl.PIXELFORMAT_ABGR8888

    /* pixelFormat should be ABGR8888 on little-endian (x86) and
     * RBGA8888 on big-endian (arm)
     */

    surface, err := sdl.CreateRGBSurfaceWithFormatFrom(pixels, int32(width), int32(height), int32(depth), int32(pitch), uint32(pixelFormat))
    if err != nil {
        return fmt.Errorf("Unable to create surface from pixels: %v", err)
    }
    if surface == nil {
        return fmt.Errorf("Did not create a surface somehow")
    }

    defer surface.Free()

    texture, err := renderer.CreateTextureFromSurface(surface)
    if err != nil {
        return fmt.Errorf("Could not create texture: %v", err)
    }

    defer texture.Destroy()

    // texture_format, access, width, height, err := texture.Query()
    // log.Printf("Texture format=%v access=%v width=%v height=%v err=%v\n", get_pixel_format(texture_format), access, width, height, err)

    destRect := sdl.Rect{X: int32(destX), Y: int32(destY), W: int32(destWidth), H: int32(destHeight)}
    renderer.Copy(texture, nil, &destRect)

    return nil
}



/* FIXME: cache the resulting texture */
func imageToTexture(data image.Image, renderer *sdl.Renderer) (*sdl.Texture, error) {
    /* encode image to bmp to a raw memory stream
     * use sdl.RWFromMem to get an rwops
     * use sdl.LoadBMPRW from rwops to get a surface
     * convert surface to texture
     *
     * could we go directly from an image to a surface and skip the bmp step?
     * probably, but this way is much simpler to implement.
     */

    var memory bytes.Buffer
    err := bmp.Encode(&memory, data)
    if err != nil {
        return nil, err
    }

    rwops, err := sdl.RWFromMem(memory.Bytes())
    if err != nil {
        return nil, err
    }

    surface, err := sdl.LoadBMPRW(rwops, false)
    if err != nil {
        return nil, err
    }
    defer surface.Free()

    return renderer.CreateTextureFromSurface(surface)
}

func loadPng(path string) (image.Image, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    return png.Decode(file)
}

/* Maps a hash of a string and the 32-bit representation of a color to a texture id */
type ButtonManager struct {
    Ids map[uint64]map[uint32]TextureId
    Lock sync.Mutex
}

func MakeButtonManager() ButtonManager {
    return ButtonManager{
        Ids: make(map[uint64]map[uint32]TextureId),
    }
}

/* md5 the string then add up the first 8 bytes to produce a 64-bit value */
func computeStringHash(value string) uint64 {
    hash := md5.Sum([]byte(value))
    var out uint64
    for i := 0; i < 8; i++ {
        out = (out << 8) + uint64(hash[i])
    }

    return out
}

func (manager *ButtonManager) GetButtonTextureId(textureManager *TextureManager, message string, color sdl.Color) TextureId {
    manager.Lock.Lock()
    defer manager.Lock.Unlock()

    stringHash := computeStringHash(message)
    colorValue := (uint32(color.R) << 24) | (uint32(color.G) << 16) | (uint32(color.B) << 8) | uint32(color.A)

    colorMap, ok := manager.Ids[stringHash]
    if !ok {
        colorMap = make(map[uint32]TextureId)
        manager.Ids[stringHash] = colorMap
    }

    id, ok := colorMap[colorValue]
    if ok {
        return id
    }

    id = textureManager.NextId()
    colorMap[colorValue] = id
    return id
}

func MakeMenu(mainQuit context.Context, font *ttf.Font) Menu {
    quit, cancel := context.WithCancel(mainQuit)
    menuInput := make(chan MenuInput, 5)
    return Menu{
        active: false,
        quit: quit,
        cancel: cancel,
        font: font,
        Input: menuInput,
    }
}

type Button interface {
    Text() string
    Interact(SubMenu) SubMenu
}

type StaticButtonFunc func()

/* A button that does not change state */
type StaticButton struct {
    Name string
    Func StaticButtonFunc
}

func (button *StaticButton) Text() string {
    return button.Name
}

func (button *StaticButton) Interact(menu SubMenu) SubMenu {
    if button.Func != nil {
        button.Func()
    }

    return menu
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

type SubMenuFunc func() SubMenu

type SubMenuButton struct {
    Name string
    Func SubMenuFunc
}

func (button *SubMenuButton) Text() string {
    return button.Name
}

func (button *SubMenuButton) Interact(menu SubMenu) SubMenu {
    return button.Func()
}

type MenuButtons struct {
    Buttons []Button
    Selected int
    Lock sync.Mutex
}

func MakeMenuButtons() MenuButtons {
    return MenuButtons{
        Selected: 0,
    }
}

func (buttons *MenuButtons) Previous(){
    buttons.Selected -= 1
    if buttons.Selected < 0 {
        buttons.Selected = len(buttons.Buttons) - 1
    }
}

func (buttons *MenuButtons) Next(){
    buttons.Selected = (buttons.Selected + 1) % len(buttons.Buttons)
}

func (buttons *MenuButtons) Add(button Button){
    buttons.Buttons = append(buttons.Buttons, button)
}

func isAudioEnabled(quit context.Context, programActions chan<- common.ProgramActions) bool {
    response := make(chan bool)
    programActions <- &common.ProgramQueryAudioState{Response: response}
    select {
        case value := <-response:
            return value
        case <-quit.Done():
            return false
    }
}

type SubMenu interface {
    /* Returns the new menu based on what button was pressed */
    Input(input MenuInput) SubMenu
    MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font) common.RenderFunction
    UpdateWindowSize(int, int)
    RawInput(sdl.Event)
}

func (buttons *MenuButtons) Interact(input MenuInput, menu SubMenu) SubMenu {
    buttons.Lock.Lock()
    defer buttons.Lock.Unlock()

    switch input {
    case MenuPrevious:
        buttons.Previous()
    case MenuNext:
        buttons.Next()
    case MenuSelect:
        return buttons.Buttons[buttons.Selected].Interact(menu)
    }

    return menu
}

func (buttons *MenuButtons) Render(startX int, startY int, maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, renderer *sdl.Renderer) error {
    buttons.Lock.Lock()
    buttons.Lock.Unlock()

    yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

    const buttonDistance = 50

    x := startX
    y := startY
    for i, button := range buttons.Buttons {
        color := white
        if i == buttons.Selected {
            color = yellow
        }

        if x > maxWidth - textWidth(font, button.Text()) {
            x = startX
            y += font.Height() + 20
        }

        textureId := buttonManager.GetButtonTextureId(textureManager, button.Text(), color)
        width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, button.Text(), color)
        x += width + buttonDistance
        _ = height
        if err != nil {
            return err
        }
    }

    return nil
}

type JoystickState interface {
}

type JoystickStateAdd struct {
    Index int
    Name string
}

type JoystickStateRemove struct {
    Index int
}

// callback that is invoked when MenuQuit is input
type MenuQuitFunc func(SubMenu) SubMenu

type StaticMenu struct {
    Buttons MenuButtons
    Quit MenuQuitFunc
}

func (menu *StaticMenu) RawInput(event sdl.Event){
}

func (menu *StaticMenu) UpdateWindowSize(x int, y int){
    // nothing
}

func (menu *StaticMenu) Input(input MenuInput) SubMenu {
    switch input {
        case MenuQuit:
            return menu.Quit(menu)
        default:
            return menu.Buttons.Interact(input, menu)
    }
}

func (menu *StaticMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {

        return menu.Buttons.Render(50, 50, maxWidth, maxHeight, buttonManager, textureManager, font, renderer)
    }
}

type JoystickButtonMapping struct {
    Inputs map[string]JoystickInputType
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
    /* FIXME */

    if manager.Player1 != nil {
        for name, input := range mapping.Inputs {
            manager.Player1.SetButton(convertButton(name), convertInput(input))
        }
    }
}

func (mapping *JoystickButtonMapping) AddAxisMapping(name string, axis JoystickAxisType){
    mapping.Inputs[name] = &axis
}

func (mapping *JoystickButtonMapping) AddButtonMapping(name string, code int){
    mapping.Inputs[name] = &JoystickButtonType{
        Name: name,
        Pressed: false,
        Button: code,
   }
}

func (mapping *JoystickButtonMapping) Unmap(name string){
    delete(mapping.Inputs, name)
}

func (mapping *JoystickButtonMapping) HandleAxis(event *sdl.JoyAxisEvent){
    /* release all axis based on the new event */
    for _, input := range mapping.Inputs {
        axis, ok := input.(*JoystickAxisType)
        if ok {
            axis.Pressed = false
        }
    }

    /* press the axis down if value is not zero */
    if event.Value != 0 {
        for _, input := range mapping.Inputs {
            axis, ok := input.(*JoystickAxisType)
            if ok && axis.Axis == int(event.Axis) && ((axis.Value < 0 && event.Value < 0) || (axis.Value > 0 && event.Value > 0)){
                axis.Pressed = true
            }
        }
    }
}

func (mapping *JoystickButtonMapping) Press(rawButton int){
    for _, input := range mapping.Inputs {
        value, ok := input.(*JoystickButtonType)
        if ok && value.Button == rawButton {
            value.Pressed = true
        }
    }
}

func (mapping *JoystickButtonMapping) Release(rawButton int){
    for _, input := range mapping.Inputs {
        value, ok := input.(*JoystickButtonType)
        if ok && value.Button == rawButton {
            value.Pressed = false
        }
    }
}

/* returns the sdl joystick button mapped to the given name, or -1
 * if no such mapping exists
 */
func (mapping *JoystickButtonMapping) GetRawCode(name string) int {
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

func (mapping *JoystickButtonMapping) ButtonList() []string {
    /* FIXME: get this dynamically from the underlying Buttons map */
    return []string{"Up", "Down", "Left", "Right", "A", "B", "Select", "Start"}
}

func (mapping *JoystickButtonMapping) IsPressed(name string) bool {
    input, ok := mapping.Inputs[name]
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
    Button int
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
    JoystickName string
    JoystickIndex int
    Textures map[string]TextureId
    Lock sync.Mutex
    Configuring bool
    Mapping JoystickButtonMapping

    // the button currently being configured, which is an index into the ButtonList()
    PartialButton JoystickInputType
    PartialCounter int
    ConfigureButton int
    Released chan int
    ConfigurePrevious context.CancelFunc
    JoystickManager *common.JoystickManager
}

func (menu *JoystickMenu) UpdateWindowSize(x int, y int){
    // nothing
}

func (menu *JoystickMenu) FinishConfigure() {
    menu.Configuring = false

    menu.Mapping.UpdateJoystick(menu.JoystickManager)
}

func (menu *JoystickMenu) RawInput(event sdl.Event){
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    if menu.Configuring {
        /* if its a press then set the current partial key to that press
         * and set a timer for ~1s, if the release comes after 1s then
         * set the button.
         */
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
                        ticker := time.NewTicker(100 * time.Millisecond)
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
                                if menu.PartialCounter < 10 {
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
                            if menu.ConfigureButton >= len(menu.Mapping.ButtonList()) {
                                menu.FinishConfigure()
                            }
                        } else {
                            menu.PartialButton = nil
                            menu.Mapping.Unmap(pressed.Name)
                        }

                        /* FIXME: channel leak with the timer */
                        // ticker.Stop()
                        /*
                        if !timer.Stop() {
                            go func(){
                                <-timer.C
                            }()
                        }
                        */
                    }(JoystickButtonType{
                        Name: menu.Mapping.ButtonList()[menu.ConfigureButton],
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

        /* if its an axis event then keep track of which axis and value was pressed.
         * as long as the same axis and mostly the same value is pressed then use that
         * pair of values (axis, value) as the button
         */
        axis, ok := event.(*sdl.JoyAxisEvent)
        if ok {
            log.Printf("Axis event axis=%v value=%v\n", axis.Axis, axis.Value)

            /* when the user lets go of the current axis button a 'release' axis event
             * will be emitted, which is an axis event with value=0. at that point
             * the ConfigurePrevious() cancel method will be invoked, which will cause
             * the most recently pressed axis to configure the button.
             */
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
                    ticker := time.NewTicker(100 * time.Millisecond)
                    defer ticker.Stop()
                    ok := false
                    done := false
                    for !done {
                        select {
                        case <-quit.Done():
                            done = true
                        case <-ticker.C:
                            menu.Lock.Lock()
                            if menu.PartialCounter < 10 {
                                menu.PartialCounter += 1
                            } else {
                                ok = true
                            }
                            menu.Lock.Unlock()
                        }
                    }

                    menu.Lock.Lock()
                    defer menu.Lock.Unlock()

                    /* the axis was held long enough */
                    if ok {
                        log.Printf("Map button %v to axis %v value %v\n", menu.ConfigureButton, axis.Axis, axis.Value)
                        menu.Mapping.AddAxisMapping(menu.Mapping.ButtonList()[menu.ConfigureButton], pressed)
                        menu.ConfigureButton += 1
                        if menu.ConfigureButton >= len(menu.Mapping.ButtonList()) {
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

func (menu *JoystickMenu) GetTexture(textureManager *TextureManager, text string) TextureId {
    id, ok := menu.Textures[text]
    if ok {
        return id
    }

    next := textureManager.NextId()
    menu.Textures[text] = next
    return next
}

func (menu *JoystickMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font) common.RenderFunction {
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    text := fmt.Sprintf("Joystick: %v", menu.JoystickName)

    textureId := menu.GetTexture(textureManager, text)

    return func(renderer *sdl.Renderer) error {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
        red := sdl.Color{R: 255, G: 0, B: 0, A: 255}

        info, err := textureManager.RenderText(font, renderer, text, white, textureId)
        if err != nil {
            return err
        }

        x := 10
        y := 10

        err = copyTexture(info.Texture, renderer, info.Width, info.Height, x, y)
        if err != nil {
            return err
        }

        x = 50
        y = 100
        err = menu.Buttons.Render(x, y, maxWidth, maxHeight, buttonManager, textureManager, font, renderer)
        if err != nil {
            return err
        }

        y += font.Height() * 2

        if menu.Configuring {
            configureText := "Configuring: hold a button for 1 second to set it"
            configuringId := buttonManager.GetButtonTextureId(textureManager, configureText, white)
            info2, err := textureManager.RenderText(font, renderer, configureText, white, configuringId)
            if err != nil {
                return err
            }

            err = copyTexture(info2.Texture, renderer, info2.Width, info2.Height, x, y)
            if err != nil {
                return err
            }
        }

        buttons := menu.Mapping.ButtonList()

        verticalMargin := 20
        x = 80
        y += font.Height() * 3 + verticalMargin

        maxWidth := 0

        /* map the button name to its vertical position */
        buttonPositions := make(map[string]int)

        for i, button := range buttons {
            buttonPositions[button] = y
            color := white

            if menu.Configuring && menu.ConfigureButton == i {
                color = red
            }

            if !menu.Configuring && menu.Mapping.IsPressed(button) {
                color = red
            }

            textureId := buttonManager.GetButtonTextureId(textureManager, button, color)
            width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, button, color)
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

        for i, button := range buttons {
            rawButton := menu.Mapping.GetRawInput(button)
            mapped := "Unmapped"
            color := white
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

                    m := uint8(menu.PartialCounter * 255 / 10)

                    if menu.PartialCounter == 10 {
                        color = sdl.Color{R: 255, G: 255, B: 0, A: 255}
                    } else {
                        color = sdl.Color{R: 255, G: m, B: m, A: 255}
                    }
                }
            }

            textureId := buttonManager.GetButtonTextureId(textureManager, mapped, color)
            vx := x + maxWidth + 20
            vy := buttonPositions[button]
            width, height, err := drawButton(font, renderer, textureManager, textureId, vx, vy, mapped, color)
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

func MakeJoystickMenu(parent SubMenu, joystickStateChanges <-chan JoystickState, joystickManager *common.JoystickManager) SubMenu {
    menu := &JoystickMenu{
        Quit: func(current SubMenu) SubMenu {
            return parent
        },
        JoystickName: "No joystick found",
        Textures: make(map[string]TextureId),
        JoystickIndex: -1,
        Mapping: JoystickButtonMapping{
            Inputs: make(map[string]JoystickInputType),
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
            // log.Printf("Joystick state change: %v", stateChange)

            add, ok := stateChange.(*JoystickStateAdd)
            if ok {
                menu.Lock.Lock()
                menu.JoystickName = add.Name
                menu.JoystickIndex = add.Index
                log.Printf("Set joystick to '%v' index %v", add.Name, add.Index)
                menu.Lock.Unlock()
            }

            remove, ok := stateChange.(*JoystickStateRemove)
            if ok {
                _ = remove
                menu.Lock.Lock()
                menu.JoystickName = "No joystick found"
                menu.JoystickIndex = -1
                menu.Lock.Unlock()
            }
        }
    }()

    menu.Buttons.Add(&SubMenuButton{Name: "Back", Func: func() SubMenu{ return parent } })

    menu.Buttons.Add(&SubMenuButton{Name: "Configure", Func: func() SubMenu {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        menu.ConfigureButton = 0
        menu.Configuring = true
        menu.Mapping.Inputs = make(map[string]JoystickInputType)

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
}

func (loadRomMenu *LoadRomMenu) RawInput(event sdl.Event){
}

func (loadRomMenu *LoadRomMenu) Input(input MenuInput) SubMenu {
    switch input {
        case MenuNext:
            loadRomMenu.LoaderState.NextSelection()
            return loadRomMenu
        case MenuPrevious:
            loadRomMenu.LoaderState.PreviousSelection()
            return loadRomMenu
        case MenuUp:
            loadRomMenu.LoaderState.PreviousUpSelection()
            return loadRomMenu
        case MenuDown:
            loadRomMenu.LoaderState.NextDownSelection()
            return loadRomMenu
        case MenuQuit:
            loadRomMenu.LoaderCancel()
            return loadRomMenu.Back(loadRomMenu)
        case MenuSelect:
            loadRomMenu.SelectRom()
            return loadRomMenu
        default:
            return loadRomMenu
    }
}

func (loadRomMenu *LoadRomMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {
        return loadRomMenu.LoaderState.Render(maxWidth, maxHeight, font, smallFont, renderer, textureManager)
    }
}

func (loadRomMenu *LoadRomMenu) UpdateWindowSize(x int, y int){
    loadRomMenu.LoaderState.UpdateWindowSize(x, y)
}

func MakeMainMenu(menu *Menu, mainCancel context.CancelFunc, programActions chan<- common.ProgramActions, joystickStateChanges <-chan JoystickState, joystickManager *common.JoystickManager, textureManager *TextureManager) SubMenu {
    main := &StaticMenu{
        Quit: func(current SubMenu) SubMenu {
            menu.cancel()
            return current
        },
    }

    joystickMenu := MakeJoystickMenu(main, joystickStateChanges, joystickManager)

    main.Buttons.Add(&StaticButton{Name: "Quit", Func: func(){
        mainCancel()
    }})

    main.Buttons.Add(&SubMenuButton{Name: "Load ROM", Func: func() SubMenu {
        loadRomQuit, loadRomCancel := context.WithCancel(menu.quit)

        romLoaderState := MakeRomLoaderState(loadRomQuit, 1, 1, textureManager.NextId())
        go romLoader(loadRomQuit, romLoaderState)

        return &LoadRomMenu{
            Back: func(current SubMenu) SubMenu {
                return main
            },
            SelectRom: func(){
                rom, ok := romLoaderState.GetSelectedRom()
                if ok {
                    menu.cancel()
                    programActions <- &common.ProgramLoadRom{Path: rom}
                }
            },
            Quit: loadRomQuit,
            LoaderCancel: loadRomCancel,
            MenuCancel: menu.cancel,
            LoaderState: romLoaderState,
        }
    }})

    main.Buttons.Add(&ToggleButton{State1: "Sound enabled", State2: "Sound disabled", state: isAudioEnabled(menu.quit, programActions),
                              Func: func(value bool){
                                  log.Printf("Set sound to %v", value)
                                  programActions <- &common.ProgramToggleSound{}
                              },
                })

    main.Buttons.Add(&SubMenuButton{Name: "Joystick", Func: func() SubMenu { return joystickMenu } })

    return main
}

func (menu *Menu) Run(window *sdl.Window, mainCancel context.CancelFunc, font *ttf.Font, smallFont *ttf.Font, programActions chan<- common.ProgramActions, renderNow chan bool, renderFuncUpdate chan common.RenderFunction, joystickManager *common.JoystickManager){

    windowSizeUpdates := make(chan common.WindowSize, 10)

    userInput := make(chan MenuInput, 3)
    defer close(userInput)

    joystickStateChanges := make(chan JoystickState, 3)
    defer close(joystickStateChanges)

    rawEvents := make(chan sdl.Event, 100)

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
                case sdl.QUIT: mainCancel()
                case sdl.JOYDEVICEADDED:
                    add_event := event.(*sdl.JoyDeviceAddedEvent)
                    joystickStateChanges <- &JoystickStateAdd{
                        Index: int(add_event.Which),
                        Name: strings.TrimSpace(sdl.JoystickNameForIndex(int(add_event.Which))),
                    }
                case sdl.JOYDEVICEREMOVED:
                    remove_event := event.(*sdl.JoyDeviceRemovedEvent)
                    joystickStateChanges <- &JoystickStateRemove{
                        Index: int(remove_event.Which),
                    }
                case sdl.WINDOWEVENT:
                    window_event := event.(*sdl.WindowEvent)
                    switch window_event.Event {
                        case sdl.WINDOWEVENT_EXPOSED:
                            select {
                                case renderNow <- true:
                                default:
                            }
                        case sdl.WINDOWEVENT_RESIZED:
                            // log.Printf("Window resized")

                    }

                    width, height := window.GetSize()
                    /* Not great but tolerate not updating the system when the window changes */
                    select {
                        case windowSizeUpdates <- common.WindowSize{X: int(width), Y: int(height)}:
                        default:
                            log.Printf("Warning: dropping a window event")
                    }

                case sdl.KEYDOWN:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    // log.Printf("key down %+v pressed %v escape %v", keyboard_event, keyboard_event.State == sdl.PRESSED, keyboard_event.Keysym.Sym == sdl.K_ESCAPE)
                    quit_pressed := keyboard_event.State == sdl.PRESSED && (keyboard_event.Keysym.Sym == sdl.K_ESCAPE || keyboard_event.Keysym.Sym == sdl.K_CAPSLOCK)

                    if quit_pressed {
                        // menu.cancel()
                        userInput <- MenuQuit
                    }

                    switch keyboard_event.Keysym.Scancode {
                        case sdl.SCANCODE_LEFT, sdl.SCANCODE_H:
                            select {
                                case userInput <- MenuPrevious:
                            }
                        case sdl.SCANCODE_RIGHT, sdl.SCANCODE_L:
                            select {
                                case userInput <- MenuNext:
                            }
                        case sdl.SCANCODE_UP, sdl.SCANCODE_K:
                            select {
                                case userInput <- MenuUp:
                            }
                        case sdl.SCANCODE_DOWN, sdl.SCANCODE_J:
                            select {
                                case userInput <- MenuDown:
                            }
                        case sdl.SCANCODE_RETURN:
                            select {
                                case userInput <- MenuSelect:
                            }
                    }
            }
        }
    }

    /* Logic loop */
    go func(){
        textureManager := MakeTextureManager()
        defer textureManager.Destroy()

        snowTicker := time.NewTicker(time.Second / 20)
        defer snowTicker.Stop()

        var snow []Snow

        /* Draw a reddish overlay on the screen */
        baseRenderer := func(renderer *sdl.Renderer) error {
            err := renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
            _ = err
            renderer.SetDrawColor(32, 0, 0, 192)
            renderer.FillRect(nil)
            return nil
        }

        makeSnowRenderer := func(snowflakes []Snow) common.RenderFunction {
            snowCopy := copySnow(snowflakes)
            return func(renderer *sdl.Renderer) error {
                for _, snow := range snowCopy {
                    c := snow.color
                    renderer.SetDrawColor(c, c, c, 255)
                    renderer.DrawPoint(int32(snow.x), int32(snow.y))
                }
                return nil
            }
        }

        buttonManager := MakeButtonManager()
        nesEmulatorTextureId := textureManager.NextId()
        myNameTextureId := textureManager.NextId()

        var windowSize common.WindowSize

        makeDefaultInfoRenderer := func(maxWidth int, maxHeight int) common.RenderFunction {
            white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
            return func(renderer *sdl.Renderer) error {
                err := writeFontCached(font, renderer, textureManager, nesEmulatorTextureId, maxWidth - 200, maxHeight - font.Height() * 3, "NES Emulator", white)
                err = writeFontCached(font, renderer, textureManager, myNameTextureId, maxWidth - 200, maxHeight - font.Height() * 3 + font.Height() + 3, "Jon Rafkind", white)
                return err
            }
        }

        wind := rand.Float32() - 0.5
        snowRenderer := makeSnowRenderer(nil)

        currentMenu := MakeMainMenu(menu, mainCancel, programActions, joystickStateChanges, joystickManager, textureManager)

        /* Reset the default renderer */
        for {
            updateRender := false
            select {
                case <-menu.quit.Done():
                    return

                case windowSize = <-windowSizeUpdates:
                    currentMenu.UpdateWindowSize(windowSize.X, windowSize.Y)

                case input := <-userInput:
                    currentMenu = currentMenu.Input(input)
                    currentMenu.UpdateWindowSize(windowSize.X, windowSize.Y)
                    /* Its slightly more efficient to tell the renderer to perform a render operation rather than
                     * to set updateRender=true which forces the chain of render functions to be recreated.
                     */
                    select {
                        case renderNow <- true:
                    }

                case event := <-rawEvents:
                    currentMenu.RawInput(event)

                case <-snowTicker.C:
                    /* FIXME: move this code somewhere else to keep the main Run() method small */
                    if len(snow) < 300 {
                        snow = append(snow, MakeSnow(windowSize.X))
                    }

                    wind += (rand.Float32() - 0.5) / 4
                    if wind < -1 {
                        wind = -1
                    }
                    if wind > 1 {
                        wind = 1
                    }

                    for i := 0; i < len(snow); i++ {
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

                        newColor := int(snow[i].color) + rand.Intn(11) - 5
                        if newColor > 255 {
                            newColor = 255
                        }
                        if newColor < 40 {
                            newColor = 40
                        }

                        snow[i].color = uint8(newColor)
                    }

                    snowRenderer = makeSnowRenderer(snow)
                    updateRender = true
            }

            if updateRender {
                /* If there is a graphics update then send it to the renderer */
                select {
                    case renderFuncUpdate <- chainRenders(baseRenderer, snowRenderer,
                                                          makeDefaultInfoRenderer(windowSize.X, windowSize.Y),
                                                          currentMenu.MakeRenderer(windowSize.X, windowSize.Y, &buttonManager, textureManager, font, smallFont)):
                    default:
                }
            }
        }
    }()

    sdl.Do(func(){
        width, height := window.GetSize()
        windowSizeUpdates <- common.WindowSize{
            X: int(width),
            Y: int(height),
        }

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

    // log.Printf("Running the menu")
    for menu.quit.Err() == nil {
        sdl.Do(eventFunction)
    }
    // log.Printf("Menu is done")
}

func (menu *Menu) Close() {
    menu.cancel()
}

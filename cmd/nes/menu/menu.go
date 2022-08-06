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
    "text/template"
    "path/filepath"

    "crypto/md5"

    "image"
    "image/png"
    "golang.org/x/image/bmp"

    "github.com/kazzmir/nes/cmd/nes/common"
    nes "github.com/kazzmir/nes/lib"

    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
    "github.com/veandco/go-sdl2/mix"
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
    Beep *mix.Music
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

/* an interactable button */
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

    err = common.CopyTexture(info.Texture, renderer, info.Width, info.Height, x + margin/2, y + margin/2)

    return info.Width, info.Height, err
}

func drawFixedWidthButton(font *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager, textureId TextureId, width int, x int, y int, message string, color sdl.Color) (int, int, error) {
    buttonInside := sdl.Color{R: 64, G: 64, B: 64, A: 255}
    buttonOutline := sdl.Color{R: 32, G: 32, B: 32, A: 255}

    info, err := textureManager.RenderText(font, renderer, message, color, textureId)
    if err != nil {
        return 0, 0, err
    }

    margin := 12

    renderer.SetDrawColor(buttonOutline.R, buttonOutline.G, buttonOutline.B, buttonOutline.A)
    renderer.FillRect(&sdl.Rect{X: int32(x), Y: int32(y), W: int32(width + margin), H: int32(info.Height + margin)})

    renderer.SetDrawColor(buttonInside.R, buttonInside.G, buttonInside.B, buttonInside.A)
    renderer.FillRect(&sdl.Rect{X: int32(x+1), Y: int32(y+1), W: int32(width + margin - 3), H: int32(info.Height + margin - 3)})

    err = common.CopyTexture(info.Texture, renderer, info.Width, info.Height, x + margin/2, y + margin/2)

    return width + margin, info.Height, err
}

/* a button that cannot be interacted with */
func drawConstButton(font *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager, textureId TextureId, x int, y int, message string, color sdl.Color) (int, int, error) {
    buttonInside := sdl.Color{R: 0x55, G: 0x55, B: 0x40, A: 255}
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

    err = common.CopyTexture(info.Texture, renderer, info.Width, info.Height, x + margin/2, y + margin/2)

    return info.Width, info.Height, err
}


func writeFontCached(font *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager, id TextureId, x int, y int, message string, color sdl.Color) error {
    info, err := textureManager.RenderText(font, renderer, message, color, id)
    if err != nil {
        return err
    }
    return common.CopyTexture(info.Texture, renderer, info.Width, info.Height, x, y)
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
    beep, err := mix.LoadMUS(filepath.Join(filepath.Dir(os.Args[0]), "data/beep.ogg"))
    if err != nil {
        log.Printf("Could not load data/beep.ogg: %v\n", err)
        beep = nil
    }
    return Menu{
        active: false,
        quit: quit,
        cancel: cancel,
        font: font,
        Input: menuInput,
        Beep: beep,
    }
}

type MenuItem interface {
    Text() string
    /* returns next x,y coordinate where rendering can occur, and a possible error */
    Render(*ttf.Font, *sdl.Renderer, *ButtonManager, *TextureManager, int, int, bool, uint64) (int, int, error)
}

type MenuSpace struct {
    Space int
}

func (space *MenuSpace) Text() string {
    return ""
}

func (space *MenuSpace) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    return x + space.Space, y, nil
}

type MenuNextLine struct {
}

func (line *MenuNextLine) Text() string {
    return "\n"
}

func (line *MenuNextLine) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    /* Force the renderer to go to the next line */
    return 999999999, 0, nil
}

type MenuLabel struct {
    Label string
    Color sdl.Color
}

func (label *MenuLabel) Text() string {
    return label.Label
}

func (label *MenuLabel) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    // color := sdl.Color{R: 255, G: 0, B: 0, A: 255}
    textureId := buttonManager.GetButtonTextureId(textureManager, label.Text(), label.Color)
    width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, label.Text(), label.Color)
    return width, height, err
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

func (button *StaticButton) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    return _doRenderButton(button, font, renderer, buttonManager, textureManager, x, y, selected, clock)
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

func (button *ToggleButton) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    return _doRenderButton(button, font, renderer, buttonManager, textureManager, x, y, selected, clock)
}

type SubMenuFunc func() SubMenu

type SubMenuButton struct {
    Name string
    Func SubMenuFunc
}

func _doRenderButton(button Button, font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
    red := sdl.Color{R: 255, G: 0, B: 0, A: 255}
    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

    color := white
    if selected {
        color = common.Glow(red, yellow, 15, clock)
    }

    textureId := buttonManager.GetButtonTextureId(textureManager, button.Text(), color)
    width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, button.Text(), color)

    return width, height, err
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

func (button *StaticFixedWidthButton) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
    red := sdl.Color{R: 255, G: 0, B: 0, A: 255}
    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

    color := white
    if selected {
        color = common.Glow(red, yellow, 15, clock)
    }

    button.Lock.Lock()
    parts := common.CopyArray(button.Parts)
    button.Lock.Unlock()

    totalLength := 0
    for _, part := range parts {
        totalLength += common.TextWidth(font, part)
    }

    space := common.TextWidth(font, " ")

    left := button.Width - totalLength
    var out string
    if left > 0 {
        spaces := left / space

        if len(parts) > 0 {
            out = parts[0]
            if len(parts) > 1 {
                out += strings.Repeat(" ", spaces)
                for _, part := range parts[1:] {
                    out += part
                }
            }
        }
    } else {
        for _, part := range parts {
            out += part
        }
    }

    textureId := buttonManager.GetButtonTextureId(textureManager, out, color)

    width, height, err := drawFixedWidthButton(font, renderer, textureManager, textureId, button.Width, x, y, out, color)

    return width, height, err
}


func (button *SubMenuButton) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    return _doRenderButton(button, font, renderer, buttonManager, textureManager, x, y, selected, clock)
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
    MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font, clock uint64) common.RenderFunction
    UpdateWindowSize(int, int)
    RawInput(sdl.Event)
    PlayBeep()
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

func (buttons *MenuButtons) Render(startX int, startY int, maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, renderer *sdl.Renderer, clock uint64) (int, int, error) {
    buttons.Lock.Lock()
    defer buttons.Lock.Unlock()

    const itemDistance = 50

    x := startX
    y := startY
    for i, item := range buttons.Items {
        if x > maxWidth - common.TextWidth(font, item.Text()) {
            x = startX
            y += font.Height() + 20
        }

        width, height, err := item.Render(font, renderer, buttonManager, textureManager, x, y, i == buttons.Selected, clock)

        // textureId := buttonManager.GetButtonTextureId(textureManager, button.Text(), color)
        // width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, button.Text(), color)
        x += width + itemDistance
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
    InstanceId sdl.JoystickID
    Name string
}

type JoystickStateRemove struct {
    Index int
    InstanceId sdl.JoystickID
}

// callback that is invoked when MenuQuit is input
type MenuQuitFunc func(SubMenu) SubMenu

type StaticMenu struct {
    Buttons MenuButtons
    Quit MenuQuitFunc
    ExtraInfo string
    Beep *mix.Music
}

func (menu *StaticMenu) PlayBeep() {
    if menu.Beep != nil {
        menu.Beep.Play(0)
    }
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

func renderLines(renderer *sdl.Renderer, x int, y int, font *ttf.Font, info string) (int, int, error) {
    aLength := common.TextWidth(font, "A")
    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

    for _, line := range strings.Split(info, "\n") {
        parts := strings.Split(line, "\t")
        for i, part := range parts {
            common.WriteFont(font, renderer, x + i * aLength * 20, y, part, white)
        }
        y += font.Height() + 2
    }

    return x, y, nil
}

func (menu *StaticMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font, clock uint64) common.RenderFunction {
    
    return func(renderer *sdl.Renderer) error {
        startX := 50
        _, y, err := menu.Buttons.Render(startX, 50, maxWidth, maxHeight, buttonManager, textureManager, font, renderer, clock)

        x := startX
        y += font.Height() * 3

        _, _, err = renderLines(renderer, x, y, smallFont, menu.ExtraInfo)
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
    /* FIXME */

    if manager.Player1 != nil {
        for name, input := range mapping.Inputs {
            manager.Player1.SetButton(convertButton(name), convertInput(input))
        }

        /* FIXME: just a test */
        manager.Player1.SetExtraButton(common.EmulatorTurbo, &common.JoystickButton{Button: 5})

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

func (mapping *JoystickButtonMapping) AddButtonMapping(name string, code int){
    if inList(name, mapping.ButtonList()){
        mapping.Inputs[name] = &JoystickButtonType{
            Name: name,
            Pressed: false,
            Button: code,
       }
   } else if inList(name, mapping.ExtraButtonList()){
       mapping.ExtraInputs[name] = &JoystickButtonType{
            Name: name,
            Pressed: false,
            Button: code,
       }
   }
}

func (mapping *JoystickButtonMapping) Unmap(name string){
    delete(mapping.Inputs, name)
}

func handleAxisMap(inputs map[string]JoystickInputType, event *sdl.JoyAxisEvent){
    /* release all axis based on the new event */
    for _, input := range inputs {
        axis, ok := input.(*JoystickAxisType)
        if ok {
            axis.Pressed = false
        }
    }

    /* press the axis down if value is not zero */
    if event.Value != 0 {
        for _, input := range inputs {
            axis, ok := input.(*JoystickAxisType)
            if ok && axis.Axis == int(event.Axis) && ((axis.Value < 0 && event.Value < 0) || (axis.Value > 0 && event.Value > 0)){
                axis.Pressed = true
            }
        }
    }
}

func (mapping *JoystickButtonMapping) HandleAxis(event *sdl.JoyAxisEvent){
    handleAxisMap(mapping.Inputs, event)
    handleAxisMap(mapping.ExtraInputs, event)
}

func (mapping *JoystickButtonMapping) Press(rawButton int){
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

func (mapping *JoystickButtonMapping) Release(rawButton int){
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
    // JoystickName string
    // JoystickIndex int
    Textures map[string]TextureId
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
}

const JoystickMaxPartialCounter = 20

func (menu *JoystickMenu) PlayBeep() {
    /* TODO */
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

                    /* the axis was held long enough */
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

func (menu *JoystickMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font, clock uint64) common.RenderFunction {
    menu.Lock.Lock()
    defer menu.Lock.Unlock()

    text := fmt.Sprintf("Joystick: %v", menu.JoystickManager.CurrentName())

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

        err = common.CopyTexture(info.Texture, renderer, info.Width, info.Height, x, y)
        if err != nil {
            return err
        }

        x = 50
        y = 100
        _, y, err = menu.Buttons.Render(x, y, maxWidth, maxHeight, buttonManager, textureManager, font, renderer, clock)
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

            err = common.CopyTexture(info2.Texture, renderer, info2.Width, info2.Height, x, y)
            if err != nil {
                return err
            }
        }

        buttons := menu.Mapping.ButtonList()

        verticalMargin := 20
        x = 80
        y += font.Height()
        // y += font.Height() * 3 + verticalMargin

        drawOffsetYButtons := y

        maxWidth := 0

        /* draw the regular buttons on the left side */

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
            width, height, err := drawButton(smallFont, renderer, textureManager, textureId, x, y, button, color)
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

                    m := uint8(menu.PartialCounter * 255 / JoystickMaxPartialCounter)

                    if menu.PartialCounter == JoystickMaxPartialCounter {
                        color = sdl.Color{R: 255, G: 255, B: 0, A: 255}
                    } else {
                        color = sdl.Color{R: 255, G: m, B: m, A: 255}
                    }
                }
            }

            textureId := buttonManager.GetButtonTextureId(textureManager, mapped, color)
            vx := x + maxWidth + 20
            vy := buttonPositions[button]
            width, height, err := drawConstButton(smallFont, renderer, textureManager, textureId, vx, vy, mapped, color)

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
        extraButtonPositions := make(map[string]int)
        maxWidthExtra := maxWidth
        for i, button := range extraButtons {
            color := white

            if menu.Configuring && menu.ConfigureButton == extraInputsStart + i {
                color = red
            }

            if !menu.Configuring && menu.Mapping.IsPressed(button) {
                color = red
            }

            textureId := buttonManager.GetButtonTextureId(textureManager, button, color)
            width, height, err := drawButton(smallFont, renderer, textureManager, textureId, x, y, button, color)
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
            color := white
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
                        color = sdl.Color{R: 255, G: 255, B: 0, A: 255}
                    } else {
                        color = sdl.Color{R: 255, G: m, B: m, A: 255}
                    }
                }
            }

            textureId := buttonManager.GetButtonTextureId(textureManager, mapped, color)
            vx := x + maxWidthExtra + 20
            vy := extraButtonPositions[button]
            width, height, err := drawConstButton(smallFont, renderer, textureManager, textureId, vx, vy, mapped, color)

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
        // JoystickName: "No joystick found",
        Textures: make(map[string]TextureId),
        // JoystickIndex: -1,
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
            // log.Printf("Joystick state change: %v", stateChange)

            add, ok := stateChange.(*JoystickStateAdd)
            if ok {
                // log.Printf("Add joystick")
                // menu.Lock.Lock()
                err := joystickManager.AddJoystick(add.Index)
                if err != nil && err != common.JoystickAlreadyAdded {
                    log.Printf("Warning: could not add joystick %v: %v\n", add.InstanceId, err)
                }
                /*
                menu.JoystickName = add.Name
                menu.JoystickIndex = add.Index
                log.Printf("Set joystick to '%v' index %v", add.Name, add.Index)
                */
                // menu.Lock.Unlock()
            }

            remove, ok := stateChange.(*JoystickStateRemove)
            if ok {
                // log.Printf("Remove joystick")
                _ = remove
                // menu.Lock.Lock()
                joystickManager.RemoveJoystick(remove.InstanceId)
                /*
                menu.JoystickName = "No joystick found"
                menu.JoystickIndex = -1
                */
                // menu.Lock.Unlock()
            }
        }
    }()

    menu.Buttons.Add(&SubMenuButton{Name: "Back", Func: func() SubMenu{ return parent } })

    menu.Buttons.Add(&MenuNextLine{})
    menu.Buttons.Add(&MenuLabel{Label: "Configure", Color: sdl.Color{R: 255, G: 0, B: 0, A: 255}})
    menu.Buttons.Add(&MenuNextLine{})

    menu.Buttons.Add(&SubMenuButton{Name: "All Buttons", Func: func() SubMenu {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        menu.ConfigureButton = 0
        menu.ConfigureButtonEnd = menu.Mapping.TotalButtons()
        menu.Configuring = true
        menu.Mapping.Inputs = make(map[string]JoystickInputType)
        menu.Mapping.ExtraInputs = make(map[string]JoystickInputType)

        return menu
    }})

    menu.Buttons.Add(&SubMenuButton{Name: "Main Buttons", Func: func() SubMenu {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        menu.Configuring = true
        menu.ConfigureButton = 0
        menu.ConfigureButtonEnd = len(menu.Mapping.ButtonList())
        menu.Mapping.Inputs = make(map[string]JoystickInputType)
        return menu
    }})

    menu.Buttons.Add(&SubMenuButton{Name: "Extra Buttons", Func: func() SubMenu {
        menu.Lock.Lock()
        defer menu.Lock.Unlock()

        menu.Configuring = true
        menu.ConfigureButton = len(menu.Mapping.ButtonList())
        menu.ConfigureButtonEnd = menu.Mapping.TotalButtons()
        menu.Mapping.ExtraInputs = make(map[string]JoystickInputType)

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
    Beep *mix.Music
}

func (loadRomMenu *LoadRomMenu) PlayBeep() {
    if loadRomMenu.Beep != nil {
        loadRomMenu.Beep.Play(0)
    }
}

func (loadRomMenu *LoadRomMenu) RawInput(event sdl.Event){
    switch event.GetType() {
        case sdl.KEYDOWN:
            keyboard_event := event.(*sdl.KeyboardEvent)
            if keyboard_event.Keysym.Sym == sdl.K_BACKSPACE {
                loadRomMenu.LoaderState.SearchBackspace()
            } else if keyboard_event.Keysym.Sym == sdl.K_SPACE {
                loadRomMenu.LoaderState.SearchAdd(" ")
            } else if keyboard_event.Keysym.Sym == sdl.K_MINUS {
                loadRomMenu.LoaderState.ZoomOut()
            } else if keyboard_event.Keysym.Sym == sdl.K_EQUALS {
                loadRomMenu.LoaderState.ZoomIn()
            } else {
                name := sdl.GetKeyName(keyboard_event.Keysym.Sym)
                if len(name) == 1 {
                    loadRomMenu.LoaderState.SearchAdd(strings.ToLower(name))
                }
            }

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
            /*
            loadRomMenu.SelectRom()
            return loadRomMenu
            */
        default:
            return loadRomMenu
    }
}

func (loadRomMenu *LoadRomMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font, clock uint64) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {
        return loadRomMenu.LoaderState.Render(maxWidth, maxHeight, font, smallFont, renderer, textureManager)
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

func (loader *LoadRomInfoMenu) GetSelectionColor(use int) sdl.Color {
    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
    yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
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

func (loader *LoadRomInfoMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font, clock uint64) common.RenderFunction {
    old := loader.RomLoader.MakeRenderer(maxWidth, maxHeight, buttonManager, textureManager, font, smallFont, clock)

    return func(renderer *sdl.Renderer) error {
        // render the rom loader in the background
        err := old(renderer)
        if err != nil {
            return err
        }

        // render a semi-translucent black square on top of it
        err = renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
        _ = err
        renderer.SetDrawColor(0, 0, 0, 240)

        // margin = 5%
        marginX := maxWidth * 5 / 100
        marginY := maxHeight * 5 / 100
        margin := marginY
        if marginX < marginY {
            margin = marginX
        }

        renderer.FillRect(&sdl.Rect{X: int32(margin), Y: int32(margin), W: int32(maxWidth - margin*2), H: int32(maxHeight - margin*2)})
        renderer.SetDrawColor(255, 255, 255, 255)
        renderer.DrawRect(&sdl.Rect{X: int32(margin), Y: int32(margin), W: int32(maxWidth - margin*2), H: int32(maxHeight - margin*2)})

        x := margin + 5
        y := margin + 5

        maxX := maxWidth - margin * 2
        maxY := maxHeight - margin * 2
        _ = maxY

        thumbnail := maxWidth * 50 / 100
        if thumbnail > maxX - x {
            thumbnail = maxX - x
        }

        white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

        textY := y
        textX := x
        common.WriteFont(font, renderer, textX, textY, fmt.Sprintf("%v", filepath.Base(loader.Info.Path)), white)

        textY += font.Height() + 2

        common.WriteFont(font, renderer, textX, textY, fmt.Sprintf("File size: %v", niceSize(loader.Filesize)), white)
        textY += font.Height() + 2

        if loader.Mapper == -1 {
            common.WriteFont(font, renderer, textX, textY, "Mapper: unknown", white)
        } else {
            common.WriteFont(font, renderer, textX, textY, fmt.Sprintf("Mapper: %v", loader.Mapper), white)
        }
        textY += font.Height() + 2

        frame, ok := loader.Info.GetFrame()
        if ok {
            width := frame.Width
            height := frame.Height

            divider := float32(frame.Width) / float32(thumbnail)

            overscanPixels := 0
            // FIXME: move this allocation into the object so its not repeated every draw frame
            raw_pixels := make([]byte, width*height * 4)
            common.RenderPixelsRGBA(frame, raw_pixels, overscanPixels)
            pixelFormat := common.FindPixelFormat()

            romWidth := int(float32(width) / divider)
            romHeight := int(float32(height) / divider)
            doRender(width, height, raw_pixels, int(maxX - thumbnail - 2), int(y+10), romWidth, romHeight, pixelFormat, renderer)

            renderer.SetDrawColor(255, 0, 0, 128)
            renderer.DrawRect(&sdl.Rect{X: int32(maxX - thumbnail - 2), Y: int32(y+10), W: int32(romWidth), H: int32(romHeight)})

            yPos := maxY - font.Height() * 4
            common.WriteFont(font, renderer, x, yPos, "Load rom", loader.GetSelectionColor(LoadRomInfoSelect))
            yPos += font.Height() + 2
            common.WriteFont(font, renderer, x, yPos, "Back", loader.GetSelectionColor(LoadRomInfoBack))
        }

        return nil

        // return loadRomMenu.LoaderState.Render(maxWidth, maxHeight, font, smallFont, renderer, textureManager)
    }
}

func (loader *LoadRomInfoMenu) PlayBeep() {
    loader.RomLoader.PlayBeep()
}

func (loader *LoadRomInfoMenu) RawInput(event sdl.Event){
}

func (loader *LoadRomInfoMenu) UpdateWindowSize(x int, y int){
    loader.RomLoader.UpdateWindowSize(x, y)
}

func keysInfo(keys *common.EmulatorKeys) string {
    n := sdl.GetKeyName
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
    Beep *mix.Music
    Chooser *ChooseButton
    Choosing bool
    ChoosingKey string
    ChoosingButton *StaticFixedWidthButton
    Current uint64

    ChooseDone context.Context
    ChooseCancel context.CancelFunc

    TempChoice sdl.Keycode

    Keys *common.EmulatorKeys
    Lock sync.Mutex
}

func (menu *ChangeKeyMenu) PlayBeep() {
    if menu.Beep != nil {
        menu.Beep.Play(0)
    }
}

func (menu *ChangeKeyMenu) RawInput(event sdl.Event){
    if menu.IsChoosing() {
        key, ok := event.(*sdl.KeyboardEvent)
        if ok {
            switch key.GetType() {
                case sdl.KEYDOWN:
                    code := key.Keysym.Sym

                    if code != menu.TempChoice {

                        menu.ChooseCancel()
                        menu.Lock.Lock()
                        menu.ChooseDone, menu.ChooseCancel = context.WithCancel(menu.MenuQuit)

                        log.Printf("Change key %v", code)
                        choosingKey := menu.ChoosingKey
                        menu.TempChoice = code
                        menu.Current = 0
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
                    menu.Lock.Unlock()
            }
        }
    }
}

func (menu *ChangeKeyMenu) UpdateWindowSize(x int, y int){
    // nothing
}

func (menu *ChangeKeyMenu) SetChoosing(v bool, key string, button *StaticFixedWidthButton){
    menu.Lock.Lock()
    defer menu.Lock.Unlock()
    menu.Choosing = v
    menu.ChoosingKey = key
    menu.ChoosingButton = button
}

func (menu *ChangeKeyMenu) IsChoosing() bool {
    menu.Lock.Lock()
    defer menu.Lock.Unlock()
    return menu.Choosing
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

func (menu *ChangeKeyMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font, smallFont *ttf.Font, clock uint64) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {
        startX := 50
        _, y, err := menu.Buttons.Render(startX, 50, maxWidth, maxHeight, buttonManager, textureManager, font, renderer, clock)

        _ = err

        x := startX
        y += font.Height() * 3

        _, _, err = renderLines(renderer, x, y, smallFont, menu.ExtraInfo)

        if menu.IsChoosing() {
            yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
            red := sdl.Color{R: 255, G: 0, B: 0, A: 255}
            white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
            renderer.SetDrawColor(5, 5, 5, 230)

            line := "Press a key"
            width := common.TextWidth(font, line)
            height := font.Height()

            midX := maxWidth / 2
            midY := maxHeight / 2

            margin := 60
            x1 := midX - width / 2 - margin
            y1 := midY - height / 2 - margin
            x2 := midX + width / 2 + margin
            y2 := midY + height / 2 + margin

            renderer.FillRect(&sdl.Rect{X: int32(x1), Y: int32(y1), W: int32(x2 - x1), H: int32(y2 - y1)})
            renderer.SetDrawColor(255, 255, 255, 250)
            renderer.DrawRect(&sdl.Rect{X: int32(x1), Y: int32(y1), W: int32(x2 - x1), H: int32(y2 - y1)})

            textX := midX - width / 2
            textY := midY - height / 2
            common.WriteFont(font, renderer, midX - width / 2, midY - height / 2, line, white)

            textY += font.Height() + 2

            menu.Lock.Lock()
            tempChoice := menu.TempChoice
            current := menu.Current
            menu.Lock.Unlock()

            common.WriteFont(font, renderer, textX, textY, sdl.GetKeyName(tempChoice), common.Glow(red, yellow, 15, current))
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


func (choose *ChooseButton) Render(font *ttf.Font, renderer *sdl.Renderer, buttonManager *ButtonManager, textureManager *TextureManager, x int, y int, selected bool, clock uint64) (int, int, error) {
    if choose.IsEnabled() {

        size := 10
        common.DrawEquilateralTriange(renderer, x-size*2, y + size + font.Height() / 4, float64(size), 180.0, sdl.Color{R: 255, G: 255, B: 255, A: 255})
        width, height, err := _doRenderButton(choose, font, renderer, buttonManager, textureManager, x, y, selected, clock)
        x += width
        _ = height
        common.DrawEquilateralTriange(renderer, x+size*2, y + size + font.Height() / 4, float64(size), 0.0, sdl.Color{R: 255, G: 255, B: 255, A: 255})

        return x + size*2 + size*2, font.Height(), err
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

func MakeKeysMenu(menu *Menu, parentMenu SubMenu, keys *common.EmulatorKeys) SubMenu {

    var items []string

    for _, key := range keys.AllKeys() {
        items = append(items, fmt.Sprintf("%v: %v", key.Name, sdl.GetKeyName(key.Code)))
    }

    chooseButton := &ChooseButton{Items: items}

    chooseDone, chooseCancel := context.WithCancel(menu.quit)

    keyMenu := &ChangeKeyMenu{
        MenuQuit: menu.quit,
        Quit: func(current SubMenu) SubMenu {
            return parentMenu
        },
        // ExtraInfo: keysInfo(keys),
        Beep: menu.Beep,
        ChooseDone: chooseDone,
        ChooseCancel: chooseCancel,
        Chooser: chooseButton,
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
            Parts: []string{name, sdl.GetKeyName(code)},
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

            for _, key := range keyMenu.Keys.AllKeys() {
                button := changeButtons[key.Name]
                button.Update(fmt.Sprintf("%v: %v", key.Name, sdl.GetKeyName(key.Code)))
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
    keyMenu.Buttons.Add(&MenuLabel{Label: "Select a key to change", Color: sdl.Color{R: 255, G: 255, B: 0, A: 255}})
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

func MakeMainMenu(menu *Menu, mainCancel context.CancelFunc, programActions chan<- common.ProgramActions, joystickStateChanges <-chan JoystickState, joystickManager *common.JoystickManager, textureManager *TextureManager, keys *common.EmulatorKeys) SubMenu {
    main := &StaticMenu{
        Quit: func(current SubMenu) SubMenu {
            /* quit the entire menu system if the user presses escape at the top level */
            menu.cancel()
            return current
        },
        Beep: menu.Beep,
    }

    joystickMenu := MakeJoystickMenu(main, joystickStateChanges, joystickManager)

    main.Buttons.Add(&StaticButton{Name: "Quit", Func: func(button *StaticButton){
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
            Beep: menu.Beep,
        }
    }})

    main.Buttons.Add(&ToggleButton{State1: "Sound enabled", State2: "Sound disabled", state: isAudioEnabled(menu.quit, programActions),
                              Func: func(value bool){
                                  log.Printf("Set sound to %v", value)
                                  programActions <- &common.ProgramToggleSound{}
                              },
                })

    keysMenu := MakeKeysMenu(menu, main, keys)
    main.Buttons.Add(&SubMenuButton{Name: "Keys", Func: func() SubMenu {
        return keysMenu
    }})

    main.Buttons.Add(&SubMenuButton{Name: "Joystick", Func: func() SubMenu { return joystickMenu } })

    main.ExtraInfo = keysInfo(keys)

    return main
}

type MenuRenderLayer struct {
    Renderer func(renderer *sdl.Renderer) error
    Index int
}

func (layer *MenuRenderLayer) Render(info common.RenderInfo) error {
    return layer.Renderer(info.Renderer)
}

func (layer *MenuRenderLayer) ZIndex() int {
    return layer.Index
}

func (menu *Menu) Run(window *sdl.Window, mainCancel context.CancelFunc, font *ttf.Font, smallFont *ttf.Font, programActions chan<- common.ProgramActions, renderNow chan bool, renderManager *common.RenderManager, joystickManager *common.JoystickManager, emulatorKeys *common.EmulatorKeys){

    menuZIndex := 10

    defer renderManager.RemoveByIndex(menuZIndex)

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
                            programActions <- &common.ProgramLoadRom{Path: drop_event.File}
                        case sdl.DROPBEGIN:
                            log.Printf("drop begin '%v'\n", drop_event.File)
                        case sdl.DROPCOMPLETE:
                            log.Printf("drop complete '%v'\n", drop_event.File)
                        case sdl.DROPTEXT:
                            log.Printf("drop text '%v'\n", drop_event.File)
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

                    /* allow vi input */
                    switch keyboard_event.Keysym.Sym {
                        case sdl.K_LEFT, sdl.K_h:
                            select {
                                case userInput <- MenuPrevious:
                            }
                        case sdl.K_RIGHT, sdl.K_l:
                            select {
                                case userInput <- MenuNext:
                            }
                        case sdl.K_UP, sdl.K_k:
                            select {
                                case userInput <- MenuUp:
                            }
                        case sdl.K_DOWN, sdl.K_j:
                            select {
                                case userInput <- MenuDown:
                            }
                        case sdl.K_RETURN:
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
            renderer.SetDrawColor(32, 0, 0, 210)
            renderer.FillRect(nil)
            return nil
        }

        makeSnowRenderer := func(snowflakes []Snow) common.RenderFunction {
            snowCopy := common.CopyArray(snowflakes)
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
                err := writeFontCached(smallFont, renderer, textureManager, nesEmulatorTextureId, maxWidth - 130, maxHeight - smallFont.Height() * 3, "NES Emulator", white)
                err = writeFontCached(smallFont, renderer, textureManager, myNameTextureId, maxWidth - 130, maxHeight - smallFont.Height() * 3 + font.Height() + 3, "Jon Rafkind", white)
                return err
            }
        }

        wind := rand.Float32() - 0.5
        snowRenderer := makeSnowRenderer(nil)

        currentMenu := MakeMainMenu(menu, mainCancel, programActions, joystickStateChanges, joystickManager, textureManager, emulatorKeys)

        var clock uint64 = 0

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
                    clock += 1

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
                renderManager.Replace(menuZIndex, &MenuRenderLayer{
                    Renderer: chainRenders(baseRenderer, snowRenderer,
                                                          makeDefaultInfoRenderer(windowSize.X, windowSize.Y),
                                                          currentMenu.MakeRenderer(windowSize.X, windowSize.Y, &buttonManager, textureManager, font, smallFont, clock)),
                    Index: menuZIndex,
                })
                select {
                    case renderNow <- true:
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

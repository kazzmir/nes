package menu

/*
#include <stdlib.h>
*/
import "C"
import (
    "context"

    "os"
    "path/filepath"
    "fmt"
    "math"
    "math/rand"
    "time"
    "strings"
    "bytes"
    "log"
    "sync"
    "sort"

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
        id: 0,
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

/* Find roms and show thumbnails of them, then let the user select one */
func romLoader(mainQuit context.Context, romLoaderState *RomLoaderState) (nes.NESFile, error) {
    /* for each rom call runNES() and pass in EmulatorInfiniteSpeed to let
     * the emulator run as fast as possible. Pass in a maxCycle of whatever
     * correlates to about 4 seconds of runtime. Save the screens produced
     * every 1s, so there should be about 4 screenshots. Then the thumbnail
     * should cycle through all the screenshots.
     * Let the user pick a thumbnail, and when selecting a thumbnail
     * return that nesfile so it can be played normally.
     */

    possibleRoms := make(chan string, 1000)

    loaderQuit, loaderCancel := context.WithCancel(mainQuit)
    _ = loaderCancel

    /* 3 seconds worth of cycles */
    const maxCycles = uint64(3 * nes.CPUSpeed)

    var wait sync.WaitGroup
    var generatorWait sync.WaitGroup

    generatorChannel := make(chan func(), 500)

    for i := 0; i < 4; i++ {
        generatorWait.Add(1)
        go func(){
            defer generatorWait.Done()
            for generator := range generatorChannel {
                generator()
            }
        }()
    }

    /* Have 4 go routines running roms */
    for i := 0; i < 4; i++ {
        wait.Add(1)
        go func(baseRomId RomId){
            defer wait.Done()

            nextRomId := baseRomId

            for rom := range possibleRoms {
                nesFile, err := nes.ParseNesFile(rom, false)
                if err != nil {
                    log.Printf("Unable to parse nes file %v: %v", rom, err)
                    continue
                }

                cpu, err := common.SetupCPU(nesFile, false)
                if err != nil {
                    log.Printf("Unable to setup cpu for %v: %v", rom, err)
                    continue
                }

                romId := nextRomId
                nextRomId += 1

                romLoaderState.NewRom <- RomLoaderAdd{
                    Id: romId,
                    Path: rom,
                }

                /* Run the actual frame generation in a separate goroutine */
                generatorChannel <- func(){
                    if loaderQuit.Err() != nil {
                        return
                    }
                    quit, cancel := context.WithCancel(loaderQuit)

                    cpu.Input = nes.MakeInput(&NullInput{})

                    audioOutput := make(chan []float32, 1)
                    emulatorActionsInput := make(chan common.EmulatorAction, 5)
                    emulatorActionsInput <- common.EmulatorInfinite
                    var screenListeners common.ScreenListeners
                    const AudioSampleRate float32 = 44100.0

                    toDraw := make(chan nes.VirtualScreen, 1)
                    bufferReady := make(chan nes.VirtualScreen, 1)

                    buffer := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)
                    bufferReady <- buffer

                    go func(){
                        count := 0
                        for {
                            select {
                                case <-quit.Done():
                                    return
                                case screen := <-toDraw:
                                    count += 1
                                    /* every 60 frames should be 1 second */
                                    if count == 60 {
                                        romLoaderState.AddFrame <- RomLoaderFrame{
                                            Id: romId,
                                            Frame: screen.Copy(),
                                        }
                                        count = 0
                                    }

                                    bufferReady <- screen
                            }
                        }
                    }()

                    log.Printf("Start loading %v", rom)
                    err = common.RunNES(&cpu, maxCycles, quit, toDraw, bufferReady, audioOutput, emulatorActionsInput, &screenListeners, AudioSampleRate, 0)
                    if err == common.MaxCyclesReached {
                        log.Printf("%v complete", rom)
                    }

                    cancel()
                }
            }
        }(RomId(uint64(i) * 1000000))
    }

    err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
        if mainQuit.Err() != nil {
            return fmt.Errorf("quitting")
        }

        if nes.IsNESFile(path){
            // log.Printf("Possible nes file %v", path)
            possibleRoms <- path
        }

        return nil
    })

    close(possibleRoms)
    wait.Wait()
    /* Wait till the writers of the generatorChannel have stopped */

    close(generatorChannel)
    generatorWait.Wait()

    return nes.NESFile{}, err
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

type RomId uint64

type RomLoaderAdd struct {
    Id RomId
    Path string
}

type RomLoaderFrame struct {
    Id RomId
    Frame nes.VirtualScreen
}

type RomLoaderInfo struct {
    Path string
    Frames []nes.VirtualScreen
    ShowFrame int
}

func (info *RomLoaderInfo) GetFrame() (nes.VirtualScreen, bool) {
    if len(info.Frames) > 0 {
        return info.Frames[info.ShowFrame], true
    }

    return nes.VirtualScreen{}, false
}

func (info *RomLoaderInfo) NextFrame() {
    if len(info.Frames) > 0 {
        info.ShowFrame = (info.ShowFrame + 1) % len(info.Frames)
    }
}

type RomLoaderState struct {
    Roms map[RomId]*RomLoaderInfo
    NewRom chan RomLoaderAdd
    AddFrame chan RomLoaderFrame
    Lock sync.Mutex

    Arrow image.Image
    ArrowId TextureId

    /* Keep track of which tile to start with when rendering the rows
     * in the loading screen, and the last tile to render.
     * min <= indexof(selectedrom) <= max
     *
     * These values will change as new roms get added, and as
     * the user moves around the tiles using up/down/left/right
     */
    MinRenderIndex int

    WindowSizeWidth int
    WindowSizeHeight int

    SortedRomIdsAndPaths []RomIdAndPath
    SelectedRomKey string
}

func (loader *RomLoaderState) UpdateWindowSize(width int, height int){
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    loader.WindowSizeWidth = width
    loader.WindowSizeHeight = height
}

func (loader *RomLoaderState) GetSelectedRom() (string, bool) {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if loader.SelectedRomKey != "" {
        index := loader.FindSortedIdIndex(loader.SelectedRomKey)
        return loader.SortedRomIdsAndPaths[index].Path, true
    }

    return "", false
}

func (loader *RomLoaderState) moveSelection(count int){
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if len(loader.SortedRomIdsAndPaths) == 0 {
        return
    }

    currentIndex := loader.FindSortedIdIndex(loader.SelectedRomKey)
    if currentIndex != -1 {
        length := len(loader.SortedRomIdsAndPaths)
        currentIndex = (currentIndex + count + length) % length
        loader.SelectedRomKey = loader.SortedRomIdsAndPaths[currentIndex].SortKey()
    }

    maximumTiles := loader.MaximumTiles()
    tilesPerRow := loader.TilesPerRow(loader.WindowSizeWidth)

    for currentIndex >= loader.MinRenderIndex + maximumTiles {
        loader.MinRenderIndex += tilesPerRow
    }

    for currentIndex < loader.MinRenderIndex {
        loader.MinRenderIndex -= tilesPerRow
    }

    if loader.MinRenderIndex < 0 {
        loader.MinRenderIndex = 0
    }
}

func (loader *RomLoaderState) NextSelection() {
    loader.moveSelection(1)
}

func (loader *RomLoaderState) PreviousUpSelection() {
    loader.moveSelection(-loader.TilesPerRow(loader.WindowSizeWidth))
}

func (loader *RomLoaderState) NextDownSelection() {
    loader.moveSelection(loader.TilesPerRow(loader.WindowSizeWidth))
}

func (loader *RomLoaderState) PreviousSelection() {
    loader.moveSelection(-1)
}

func (loader *RomLoaderState) AdvanceFrames() {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    for _, info := range loader.Roms {
        info.NextFrame()
    }
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

func (loader *RomLoaderState) FindSortedIdIndex(path string) int {
    baseKey := filepath.Base(path)
    /* must hold the loader.Lock before calling this */
    index := sort.Search(len(loader.SortedRomIdsAndPaths), func (check int) bool {
        info := loader.SortedRomIdsAndPaths[check]
        return strings.Compare(baseKey, info.SortKey()) <= 0
    })

    if index == len(loader.SortedRomIdsAndPaths) {
        return -1
    }

    return int(index)
}

func (loader *RomLoaderState) FindRomIdByPath(path string) RomId {
    index := loader.FindSortedIdIndex(path)
    if index == -1 {
        return 0
    }

    return loader.SortedRomIdsAndPaths[index].Id
}

type SortRomIds []RomIdAndPath

func (data SortRomIds) Len() int {
    return len(data)
}

func (data SortRomIds) Swap(left, right int){
    data[left], data[right] = data[right], data[left]
}

func (data SortRomIds) Less(left, right int) bool {
    return strings.Compare(data[left].SortKey(), data[right].SortKey()) == -1
}

type RomIdAndPath struct {
    Id RomId
    Path string
}

func (info *RomIdAndPath) SortKey() string {
    /* construct a string that combines the path and id, but make it
     * a weird so it has little chance to collide with a real filename
     */
    return fmt.Sprintf("%v_#!#-%v", strings.ToLower(filepath.Base(info.Path)), info.Id)
}

/* Must hold the lock before calling this */
func (loader *RomLoaderState) MaximumTiles() int {
    return loader.TilesPerRow(loader.WindowSizeWidth) * loader.TileRows(loader.WindowSizeHeight)
}

type TileLayout struct {
    /* Where the first tile starts */
    XStart int
    YStart int
    /* How much to increase x/y by for each tile */
    XSpace int
    YSpace int
    /* amount to divide the nes screen by to make a thumbnail */
    Thumbnail int
}

func (loader *RomLoaderState) TileLayout() TileLayout {
    return TileLayout{
        XStart: 50,
        YStart: 80,
        XSpace: 20,
        YSpace: 25,
        Thumbnail: 2,
    }
}

func (loader *RomLoaderState) TilesPerRow(maxWidth int) int {
    count := 0
    layout := loader.TileLayout()
    x := layout.XStart
    width := nes.VideoWidth

    for x + width / layout.Thumbnail + 5 < maxWidth {
        x += width / layout.Thumbnail + layout.XSpace
        count += 1
    }

    return count
}

func (loader *RomLoaderState) TileRows(maxHeight int) int {
    layout := loader.TileLayout()

    height := nes.VideoHeight - nes.OverscanPixels * 2

    y := layout.YStart
    count := 0

    yDiff := height / layout.Thumbnail + layout.YSpace
    for y + yDiff < maxHeight {
        count += 1
        y += yDiff
    }

    return count
}

func renderUpArrow(x int, y int, texture *sdl.Texture, renderer *sdl.Renderer){
    _, _, width, height, err := texture.Query()
    if err == nil {
        dest := sdl.Rect{X: int32(x), Y: int32(y), W: width, H: height}
        renderer.Copy(texture, nil, &dest)
    }
}

func renderDownArrow(x int, y int, texture *sdl.Texture, renderer *sdl.Renderer){
    _, _, width, height, err := texture.Query()
    if err == nil {
        dest := sdl.Rect{X: int32(x), Y: int32(y), W: width, H: height}
        renderer.CopyEx(texture, nil, &dest, 0, nil, sdl.FLIP_VERTICAL)
    }
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

func (loader *RomLoaderState) Render(maxWidth int, maxHeight int, font *ttf.Font, smallFont *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager) {
    /* FIXME: this coarse grained lock will slow things down a bit */
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

    writeFont(font, renderer, 1, 1, fmt.Sprintf("Load a rom. Roms found %v", len(loader.SortedRomIdsAndPaths)), white)

    layout := loader.TileLayout()

    overscanPixels := 8
    width := nes.VideoWidth
    height := nes.VideoHeight-nes.OverscanPixels*2
    x := layout.XStart
    y := layout.YStart

    selectedIndex := -1
    selectedId := RomId(0)

    if loader.SelectedRomKey != "" {
        selectedIndex = loader.FindSortedIdIndex(loader.SelectedRomKey)
        selectedId = loader.SortedRomIdsAndPaths[selectedIndex].Id
        writeFont(font, renderer, 100, font.Height() + 3, loader.SortedRomIdsAndPaths[selectedIndex].Path, white)
    }

    err := renderer.SetDrawBlendMode(sdl.BLENDMODE_NONE)
    _ = err

    raw_pixels := make([]byte, width*height * 4)
    pixelFormat := common.FindPixelFormat()

    /* if the rom doesn't have any frames loaded then show a blank thumbnail */
    blankScreen := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)
    blankScreen.ClearToColor(0, 0, 0)

    outlineSize := 3

    start := loader.MinRenderIndex
    if start < 0 {
        start = 0
    }

    end := start + loader.MaximumTiles()
    if end >= len(loader.SortedRomIdsAndPaths) {
        end = len(loader.SortedRomIdsAndPaths) - 1
    }

    arrowInfo, _ := textureManager.GetCachedTexture(loader.ArrowId, func() (TextureInfo, error){
        if loader.Arrow != nil {
            arrowTexture, err := imageToTexture(loader.Arrow, renderer)
            if err != nil {
                return TextureInfo{}, err
            } else {
                _, _, width, height, err := arrowTexture.Query()
                if err != nil {
                    arrowTexture.Destroy()
                    return TextureInfo{}, err
                }
                return TextureInfo{
                    Texture: arrowTexture,
                    Width: int(width),
                    Height: int(height),
                }, nil
            }
        } else {
            return TextureInfo{}, fmt.Errorf("No arrow image")
        }
    })

    if loader.MinRenderIndex != 0 {
        if arrowInfo.Texture != nil {
            renderUpArrow(10, 30, arrowInfo.Texture, renderer)
        }
    }

    if loader.MinRenderIndex + loader.MaximumTiles() < len(loader.SortedRomIdsAndPaths) {
        if arrowInfo.Texture != nil {
            downY := maxHeight - 50
            if downY < 30 {
                downY = 30
            }
            renderDownArrow(10, downY, arrowInfo.Texture, renderer)
        }
    }

    const MaxNameSize = 15

    for _, romIdAndPath := range loader.SortedRomIdsAndPaths[start:end+1] {
        info := loader.Roms[romIdAndPath.Id]
        frame, has := info.GetFrame()
        if !has {
            frame = blankScreen
        }

        /* Highlight the selected rom with a yellow outline */
        if selectedId == romIdAndPath.Id {
            renderer.SetDrawColor(255, 255, 0, 255)
            rect := sdl.Rect{X: int32(x-outlineSize), Y: int32(y-outlineSize), W: int32(width / layout.Thumbnail + outlineSize*2), H: int32(height / layout.Thumbnail + outlineSize*2)}
            renderer.FillRect(&rect)
        }

        /* FIXME: cache these textures with the texture manager */
        common.RenderPixelsRGBA(frame, raw_pixels, overscanPixels)
        doRender(width, height, raw_pixels, x, y, width / layout.Thumbnail, height / layout.Thumbnail, pixelFormat, renderer)

        name := filepath.Base(info.Path)
        if len(name) > MaxNameSize {
            name = fmt.Sprintf("%v..", name[0:MaxNameSize-2])
        }

        writeFont(smallFont, renderer, x, y + height / layout.Thumbnail + 1, name, white)

        x += width / layout.Thumbnail + layout.XSpace
        if x + width / layout.Thumbnail + 5 > maxWidth {
            x = layout.XStart
            y += height / layout.Thumbnail + layout.YSpace

            if y + height / layout.Thumbnail > maxHeight {
                break
            }
        }
    }
}

func (loader *RomLoaderState) AddNewRom(rom RomLoaderAdd) {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    _, ok := loader.Roms[rom.Id]
    if ok {
        log.Printf("Warning: adding a duplicate rom id: %v", rom.Id)
        return
    }

    loader.Roms[rom.Id] = &RomLoaderInfo{
        Path: rom.Path,
        Frames: nil,
    }

    distanceToMin := 0
    // distanceToMax := 0
    if loader.SelectedRomKey != "" {
        selectedIndex := loader.FindSortedIdIndex(loader.SelectedRomKey)
        distanceToMin = selectedIndex - loader.MinRenderIndex
    }

    newRomIdAndPath := RomIdAndPath{Id: rom.Id, Path: rom.Path}
    loader.SortedRomIdsAndPaths = append(loader.SortedRomIdsAndPaths, newRomIdAndPath)

    sort.Sort(SortRomIds(loader.SortedRomIdsAndPaths))

    if loader.SelectedRomKey == "" {
        loader.MinRenderIndex = 0
        loader.SelectedRomKey = newRomIdAndPath.SortKey()
    } else {
        selectedIndex := loader.FindSortedIdIndex(loader.SelectedRomKey)
        loader.MinRenderIndex = selectedIndex - distanceToMin
    }
}

func (loader *RomLoaderState) AddRomFrame(frame RomLoaderFrame) {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    info, ok := loader.Roms[frame.Id]
    if !ok {
        log.Printf("Warning: cannot add frame to non-existent rom id: %v", frame.Id)
        return
    }

    info.Frames = append(info.Frames, frame.Frame)
}

func loadPng(path string) (image.Image, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    return png.Decode(file)
}

func MakeRomLoaderState(quit context.Context, windowWidth int, windowHeight int, arrowId TextureId) *RomLoaderState {
    arrow, err := loadPng("data/arrow.png")
    if err != nil {
        log.Printf("Could not load arrow image: %v", err)
        arrow = nil
    }
    state := RomLoaderState{
        Roms: make(map[RomId]*RomLoaderInfo),
        NewRom: make(chan RomLoaderAdd, 5),
        AddFrame: make(chan RomLoaderFrame, 5),
        MinRenderIndex: 0,
        WindowSizeWidth: windowWidth,
        WindowSizeHeight: windowHeight,
        Arrow: arrow,
        ArrowId: arrowId,
    }

    go func(){
        showFrameTimer := time.NewTicker(time.Second / 2)
        defer showFrameTimer.Stop()
        for {
            select {
                case <-quit.Done():
                    return
                case rom := <-state.NewRom:
                    state.AddNewRom(rom)
                case frame := <-state.AddFrame:
                    state.AddRomFrame(frame)
                case <-showFrameTimer.C:
                    state.AdvanceFrames()
            }
        }
    }()

    return &state
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
    hash := md5.New().Sum([]byte(value))
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

type SubMenuButton struct {
    Name string
    Menu SubMenu
}

func (button *SubMenuButton) Text() string {
    return button.Name
}

func (button *SubMenuButton) Interact(menu SubMenu) SubMenu {
    return button.Menu
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

func (buttons *MenuButtons) Interact(menu SubMenu) SubMenu {
    return buttons.Buttons[buttons.Selected].Interact(menu)
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
    MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font) common.RenderFunction
    SetParentMenu(SubMenu)
}

type MainMenu struct {
    Buttons MenuButtons
}

func (mainMenu *MainMenu) SetParentMenu(menu SubMenu){
    log.Println("warning: cannot set parent menu of the main menu")
}

func (mainMenu *MainMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {
        mainMenu.Buttons.Lock.Lock()
        defer mainMenu.Buttons.Lock.Unlock()

        var err error
        yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
        white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

        x := 50
        y := 50
        for i, button := range mainMenu.Buttons.Buttons {
            color := white
            if i == mainMenu.Buttons.Selected {
                color = yellow
            }
            textureId := buttonManager.GetButtonTextureId(textureManager, button.Text(), color)
            width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, button.Text(), color)
            x += width + 50
            _ = height
            _ = err
        }

        // err = writeFont(font, renderer, 50, 50, "Quit", colors[0])
        _ = err
        return err
    }
}

func (mainMenu *MainMenu) Input(input MenuInput) SubMenu {
    mainMenu.Buttons.Lock.Lock()
    defer mainMenu.Buttons.Lock.Unlock()

    switch input {
    case MenuPrevious:
        mainMenu.Buttons.Previous()
    case MenuNext:
        mainMenu.Buttons.Next()
    case MenuSelect:
        return mainMenu.Buttons.Interact(mainMenu)
    }

    return mainMenu
}

type JoystickMenu struct {
    Parent SubMenu
}

func (joystickMenu *JoystickMenu) Input(input MenuInput) SubMenu {
    return joystickMenu
}

func (joystickMenu *JoystickMenu) MakeRenderer(maxWidth int, maxHeight int, buttonManager *ButtonManager, textureManager *TextureManager, font *ttf.Font) common.RenderFunction {
    return func(renderer *sdl.Renderer) error {
        return nil
    }
}

func (joystickMenu *JoystickMenu) SetParentMenu(menu SubMenu){
    joystickMenu.Parent = menu
}

func MakeJoystickMenu() SubMenu {
    return &JoystickMenu{
    }
}

func MakeMainMenu(menu *Menu, mainCancel context.CancelFunc, programActions chan<- common.ProgramActions) SubMenu {

    joystickMenu := MakeJoystickMenu()

    var buttons MenuButtons

    buttons.Add(&StaticButton{Name: "Quit", Func: func(){
        mainCancel()
    }})

    /* FIXME: launch the load rom interface */
    buttons.Add(&StaticButton{Name: "Load ROM"})

    buttons.Add(&ToggleButton{State1: "Sound enabled", State2: "Sound disabled", state: isAudioEnabled(menu.quit, programActions),
                              Func: func(value bool){
                                  log.Printf("Set sound to %v", value)
                                  programActions <- &common.ProgramToggleSound{}
                              },
                })

    buttons.Add(&SubMenuButton{Name: "Joystick", Menu: joystickMenu})

    main := &MainMenu{
        Buttons: buttons,
    }

    /* FIXME: ugly to put this set here */
    joystickMenu.SetParentMenu(main)

    return main
}

func (menu *Menu) Run(window *sdl.Window, mainCancel context.CancelFunc, font *ttf.Font, programActions chan<- common.ProgramActions, renderNow chan bool, renderFuncUpdate chan common.RenderFunction){

    windowSizeUpdates := make(chan common.WindowSize, 10)

    userInput := make(chan MenuInput, 3)
    defer close(userInput)

    currentMenu := MakeMainMenu(menu, mainCancel, programActions)

    eventFunction := func(){
        event := sdl.WaitEventTimeout(1)
        if event != nil {
            // log.Printf("Event %+v\n", event)
            switch event.GetType() {
                case sdl.QUIT: mainCancel()
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
                        menu.cancel()
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

        /* Reset the default renderer */
        for {
            updateRender := false
            select {
                case <-menu.quit.Done():
                    return

                case windowSize = <-windowSizeUpdates:

                case input := <-userInput:
                    currentMenu = currentMenu.Input(input)
                    select {
                        case renderNow <- true:
                    }

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
                                                          currentMenu.MakeRenderer(windowSize.X, windowSize.Y, &buttonManager, textureManager, font)):
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
    })

    // log.Printf("Running the menu")
    for menu.quit.Err() == nil {
        sdl.Do(eventFunction)
    }
    // log.Printf("Menu is done")
}

func MakeMenu2(font *ttf.Font, smallFont *ttf.Font, mainQuit context.Context, renderUpdates chan common.RenderFunction, windowSizeUpdates <-chan common.WindowSize, programActions chan<- common.ProgramActions) *Menu {
    quit, cancel := context.WithCancel(mainQuit)
    menuInput := make(chan MenuInput, 5)

    menu := &Menu{
        active: false,
        quit: quit,
        cancel: cancel,
        font: font,
        Input: menuInput,
    }

    go func(menu *Menu){
        textureManager := MakeTextureManager()
        defer textureManager.Destroy()

        snowTicker := time.NewTicker(time.Second / 20)
        defer snowTicker.Stop()

        choices := []MenuAction{MenuActionQuit, MenuActionLoadRom, MenuActionSound, MenuActionJoystick}
        choice := 0

        var snow []Snow

        wind := rand.Float32() - 0.5
        var windowSize common.WindowSize
        audio := true

        menuState := MenuStateTop

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

        makeMenuRenderer := func(choice int, maxWidth int, maxHeight int, audioEnabled bool) common.RenderFunction {
            return func(renderer *sdl.Renderer) error {
                var err error
                yellow := sdl.Color{R: 255, G: 255, B: 0, A: 255}
                white := sdl.Color{R: 255, G: 255, B: 255, A: 255}

                sound := "Sound enabled"
                if !audioEnabled {
                    sound = "Sound disabled"
                }

                buttons := []string{"Quit", "Load ROM", sound, "Joystick"}

                x := 50
                y := 50
                for i, button := range buttons {
                    color := white
                    if i == choice {
                        color = yellow
                    }
                    textureId := buttonManager.GetButtonTextureId(textureManager, button, color)
                    width, height, err := drawButton(font, renderer, textureManager, textureId, x, y, button, color)
                    x += width + 50
                    _ = height
                    _ = err
                }

                // err = writeFont(font, renderer, 50, 50, "Quit", colors[0])
                err = writeFontCached(font, renderer, textureManager, nesEmulatorTextureId, maxWidth - 200, maxHeight - font.Height() * 3, "NES Emulator", white)
                err = writeFontCached(font, renderer, textureManager, myNameTextureId, maxWidth - 200, maxHeight - font.Height() * 3 + font.Height() + 3, "Jon Rafkind", white)
                _ = err
                return err
            }
        }

        makeLoadRomRenderer := func(maxWidth int, maxHeight int, romLoadState *RomLoaderState) common.RenderFunction {
            return func(renderer *sdl.Renderer) error {

                romLoadState.Render(maxWidth, maxHeight, font, smallFont, renderer, textureManager)

                return nil
            }
        }

        menuRenderer := func(renderer *sdl.Renderer) error {
            return nil
        }
        snowRenderer := makeSnowRenderer(nil)

        loadRomQuit, loadRomCancel := context.WithCancel(mainQuit)

        var romLoaderState *RomLoaderState

        /* Reset the default renderer */
        for {
            updateRender := false
            select {
                case <-quit.Done():
                    return
                case input := <-menuInput:
                    switch menuState {
                        case MenuStateTop:
                            switch input {
                                case MenuToggle:
                                    if menu.IsActive() {
                                        /* This channel put must succeed */
                                        renderUpdates <- func(renderer *sdl.Renderer) error {
                                            return nil
                                        }
                                        programActions <- &common.ProgramUnpauseEmulator{}
                                    } else {
                                        choice = 0
                                        menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                        updateRender = true
                                        programActions <- &common.ProgramPauseEmulator{}
                                    }

                                    menu.ToggleActive()
                                case MenuNext:
                                    if menu.IsActive() {
                                        choice = (choice + 1) % len(choices)
                                        menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                        updateRender = true
                                    }
                                case MenuPrevious:
                                    if menu.IsActive() {
                                        choice = (choice - 1 + len(choices)) % len(choices)
                                        menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                        updateRender = true
                                    }
                                case MenuSelect:
                                    if menu.IsActive() {
                                        switch choices[choice] {
                                            case MenuActionQuit:
                                                programActions <- &common.ProgramQuit{}
                                            case MenuActionLoadRom:
                                                menuState = MenuStateLoadRom
                                                loadRomQuit, loadRomCancel = context.WithCancel(mainQuit)

                                                romLoaderState = MakeRomLoaderState(loadRomQuit, windowSize.X, windowSize.Y, textureManager.NextId())
                                                go romLoader(loadRomQuit, romLoaderState)

                                                menuRenderer = makeLoadRomRenderer(windowSize.X, windowSize.Y, romLoaderState)
                                                updateRender = true

                                            case MenuActionSound:
                                                programActions <- &common.ProgramToggleSound{}
                                                audio = !audio
                                                menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                                updateRender = true
                                            case MenuActionJoystick:
                                        }
                                    }
                            }

                        case MenuStateLoadRom:
                            switch input {
                                case MenuToggle:
                                    loadRomCancel()
                                    /* remove reference so its state can be gc'd */
                                    romLoaderState = nil
                                    menuState = MenuStateTop
                                    menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                    updateRender = true
                                case MenuNext:
                                    if romLoaderState != nil {
                                        romLoaderState.NextSelection()
                                        menuRenderer = makeLoadRomRenderer(windowSize.X, windowSize.Y, romLoaderState)
                                        updateRender = true
                                    }
                                case MenuPrevious:
                                    if romLoaderState != nil {
                                        romLoaderState.PreviousSelection()
                                        menuRenderer = makeLoadRomRenderer(windowSize.X, windowSize.Y, romLoaderState)
                                        updateRender = true
                                    }
                                case MenuUp:
                                    if romLoaderState != nil {
                                        romLoaderState.PreviousUpSelection()
                                        menuRenderer = makeLoadRomRenderer(windowSize.X, windowSize.Y, romLoaderState)
                                        updateRender = true
                                    }
                                case MenuDown:
                                    if romLoaderState != nil {
                                        romLoaderState.NextDownSelection()
                                        menuRenderer = makeLoadRomRenderer(windowSize.X, windowSize.Y, romLoaderState)
                                        updateRender = true
                                    }
                                case MenuSelect:
                                    loadRomCancel()
                                    /* have the main program load the selected rom */
                                    if romLoaderState != nil {
                                        rom, ok := romLoaderState.GetSelectedRom()
                                        if ok {
                                            menuState = MenuStateTop
                                            /* This could block the current goroutine, so we run it in a
                                             * separate goroutine. This isn't super different from just
                                             * making menuInput have a larger backlog, instead we are
                                             * basically using the heap as a giant unbufferd channel.
                                             */
                                            go func(){
                                                menuInput <- MenuToggle
                                            }()
                                            programActions <- &common.ProgramLoadRom{Path: rom}
                                        }
                                        romLoaderState = nil
                                    }
                            }
                    }

                case windowSize = <-windowSizeUpdates:
                    if romLoaderState != nil {
                        romLoaderState.UpdateWindowSize(windowSize.X, windowSize.Y)
                    }

                    if menu.IsActive() {
                        switch menuState {
                            case MenuStateTop:
                                menuRenderer = makeMenuRenderer(choice, windowSize.X, windowSize.Y, audio)
                                updateRender = true
                            case MenuStateLoadRom:
                                if romLoaderState != nil {
                                    menuRenderer = makeLoadRomRenderer(windowSize.X, windowSize.Y, romLoaderState)
                                    updateRender = true
                                }
                        }
                    }

                case <-snowTicker.C:
                    if menu.IsActive() {
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
                        }

                        snowRenderer = makeSnowRenderer(snow)
                        updateRender = true
                    }
            }

            if updateRender {
                /* If there is a graphics update then send it to the renderer */
                select {
                    case renderUpdates <- chainRenders(baseRenderer, snowRenderer, menuRenderer):
                    default:
                }
            }
        }
    }(menu)

    return menu
}

func (menu *Menu) Close() {
    menu.cancel()
}

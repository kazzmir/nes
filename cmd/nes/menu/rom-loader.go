package menu

/*
#include <stdlib.h>
*/
import "C"
import (
    "sync"
    "image"
    "context"
    "log"
    "path/filepath"
    "os"
    "fmt"
    "sort"
    "time"
    "strings"

    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
    "github.com/kazzmir/nes/cmd/nes/common"
    nes "github.com/kazzmir/nes/lib"
)

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

func (loader *RomLoaderState) Render(maxWidth int, maxHeight int, font *ttf.Font, smallFont *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager) error {
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

    return nil
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
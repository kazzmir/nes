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
    "math"
    "math/rand"
    "fmt"
    "sort"
    "time"
    "io/ioutil"
    "strings"

    imagelib "image"
    "image/png"
    "image/color"

    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
    "github.com/kazzmir/nes/cmd/nes/menu/filterlist"
    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/kazzmir/nes/cmd/nes/gfx"
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
    // FIXME: might need a lock here
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

    RomIdsAndPaths filterlist.List[*RomIdAndPath]
    SelectedRomKey string

    Layout TileLayout
}

type PossibleRom struct {
    Path string
    RomId RomId
}

var uniqueIdentifier string
var computeIdentifier sync.Once

/* compute the sha256 of the program itself */
func getUniqueProgramIdentifier() string {
    computeIdentifier.Do(func (){
        value, err := common.GetSha256(os.Args[0])
        if err != nil {
            log.Printf("Warning: could not compute program hash: %v", err)
            /* couldn't read the program hash for some reason, just compute some random string */
            min := int(math.Pow10(5))
            max := int(math.Pow10(6))
            uniqueIdentifier = fmt.Sprintf("%v-%v", time.Now().UnixNano(), rand.Intn(max-min) + min)
        } else {
            uniqueIdentifier = value
        }
    })

    return uniqueIdentifier
}

func getHome() string {
    home, err := os.UserHomeDir()
    if err != nil {
        return os.TempDir()
    }
    return home
}

func dirExists(path string) bool {
    info, err := os.Stat(path)
    if os.IsNotExist(err) {
        // file does not exists return false
        return false
    }

    // return true if exist and is a directory
    return info.IsDir()
}

/* convert an image to the nes virtual screen representation */
func imageToScreen(image imagelib.Image) nes.VirtualScreen {
    width := image.Bounds().Max.X
    height := image.Bounds().Max.Y

    out := nes.VirtualScreen{
        Width: width,
        Height: height,
        Buffer: make([]uint32, width * height),
    }

    for x := 0; x < width; x++ {
        for y := 0; y < height; y++ {
            c := image.At(x, y)
            red, green, blue, alpha := color.NRGBAModel.Convert(c).RGBA()

            alpha = 255

            /* convert to 8-bit value */
            red = (red * 255 / alpha) & 0xff
            green = (green * 255 / alpha) & 0xff
            blue = (blue * 255 / alpha) & 0xff
            // alpha = 255
            // fmt.Printf("%v,%v: r=%v g=%v b=%v a=%v\n", x, y, red, green, blue, alpha)

            value := (red << 24) | (green << 16) | (blue << 8) | (alpha << 0)
            out.Buffer[x+y*width] = value
        }
    }

    return out
}

func getCachedPath(sha string) string {
    return filepath.Join(getHome(), ".cache", "jon-nes", sha)
}

/* returns true if this function was able to load the thumbnails from files, otherwise false */
func getCachedThumbnails(loaderQuit context.Context, romId RomId, path string, addFrame chan<- RomLoaderFrame) bool {
    /* look in ~/.cache/jon-nes/<sha256 of rom>/{1,2,3,4}.png
     * also keep a metadata file in the sha256 dir that correlates to the version of the emulator
     * if the running emulator is not the same as the saved one then return false
     * if all 4 png's are not there then return false
     * otherwise read each png and pass them to the addFrame
     */

    sha, err := common.GetSha256(path)
    if err != nil {
        // log.Printf("cached-thumbnail: could not get sha256")
        return false
    }
    cachePath := getCachedPath(sha)

    if !dirExists(cachePath) {
        // log.Printf("cached-thumbnail: cached path doesnt exist: %v", cachePath)
        return false
    }

    meta := filepath.Join(cachePath, "metadata")

    programSha, err := ioutil.ReadFile(meta)
    if err != nil {
        // log.Printf("cached-thumbnail: could not read metadata")
        return false
    }

    /* meta sha must match the current program identifier */
    if strings.TrimSpace(string(programSha)) != getUniqueProgramIdentifier() {
        // log.Printf("cached-thumbnail: expected program sha doesn't match expected='%v' actual='%v'", strings.TrimSpace(string(programSha)), getUniqueProgramIdentifier())
        return false
    }

    expectedFiles := []string{"1.png", "2.png", "3.png", "4.png"}
    for _, file := range expectedFiles {
        imagePath := filepath.Join(cachePath, file)
        if !common.FileExists(imagePath) {
            // log.Printf("cached-thumbnail: could not find png %v", imagePath)
            return false
        }
    }

    for _, file := range expectedFiles {
        imagePath := filepath.Join(cachePath, file)
        frame, err := loadPng(imagePath)
        if err != nil {
            // log.Printf("cached-thumbnail: could not load png %v", err)
            return false
        }

        load := RomLoaderFrame{
            Id: romId,
            Frame: imageToScreen(frame),
        }

        select {
            case addFrame <- load:
            case <-loaderQuit.Done():
                return false
        }
    }

    return true
}

/* FIXME: stole this from test/screenshot/screenshot.go */
func convertToPng(screen nes.VirtualScreen) image.Image {
    out := image.NewRGBA(image.Rect(0, 0, screen.Width, screen.Height))

    for x := 0; x < screen.Width; x++ {
        for y := 0; y < screen.Height; y++ {
            r, g, b, a := screen.GetRGBA(x, y)
            out.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
        }
    }

    return out
}

/* save the nes screen to a file */
func saveCachedFrame(count int, cachedSha256 string, path string, screen nes.VirtualScreen) error {
    cachedPath := getCachedPath(cachedSha256)
    if !dirExists(cachedPath) {
        err := os.MkdirAll(cachedPath, 0755)
        if err != nil {
            return err
        }
    }

    metadata := filepath.Join(cachedPath, "metadata")
    err := os.WriteFile(metadata, []byte(getUniqueProgramIdentifier() + "\n"), 0644)
    if err != nil {
        return err
    }

    // write the path of the rom just for info purposes
    os.WriteFile(filepath.Join(cachedPath, "info"), []byte(path + "\n"), 0644)

    name := fmt.Sprintf("%v.png", count)
    image := convertToPng(screen)

    out, err := os.Create(filepath.Join(cachedPath, name))
    if err != nil {
        return err
    }
    defer out.Close()

    return png.Encode(out, image)
}

func generateThumbnails(loaderQuit context.Context, cpu nes.CPUState, romId RomId, path string, addFrame chan<- RomLoaderFrame, doCache bool){
    if loaderQuit.Err() != nil {
        return
    }
    quit, cancel := context.WithCancel(loaderQuit)
    defer cancel()

    cpu.Input = nes.MakeInput(&NullInput{})

    audioOutput := make(chan []float32, 1)
    emulatorActionsInput := make(chan common.EmulatorAction, 5)
    emulatorActionsInput <- common.MakeEmulatorAction(common.EmulatorInfinite)
    var screenListeners common.ScreenListeners
    const AudioSampleRate float32 = 44100.0

    toDraw := make(chan nes.VirtualScreen, 1)
    bufferReady := make(chan nes.VirtualScreen, 1)

    buffer := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)
    bufferReady <- buffer

    var err error
    cachedSha := ""
    if doCache {
        cachedSha, err = common.GetSha256(path)
        if err != nil {
            log.Printf("Could not get sha of '%v': %v", path, err)
            cachedSha = "x"
        }
    }

    go func(){
        frameNumber := 1
        count := 0
        for {
            select {
            case <-quit.Done():
                return
            case screen := <-toDraw:
                count += 1
                /* every 60 frames should be 1 second */
                if count == 60 {
                    frame := RomLoaderFrame{
                        Id: romId,
                        Frame: screen.Copy(),
                    }

                    if doCache {
                        err := saveCachedFrame(frameNumber, cachedSha, path, screen)
                        if err != nil {
                            log.Printf("Could not save cached frame: %v", err)
                        }
                    }

                    frameNumber += 1
                    /* once we have all 4 frames stop running the emulator */
                    if frameNumber == 5 {
                        cancel()
                    }

                    select {
                    case addFrame <- frame:
                    case <-quit.Done():
                        return
                    }
                    count = 0
                }

                bufferReady <- screen
            }
        }
    }()

    /* don't load more than 30s worth */
    const maxCycles = uint64(30 * nes.CPUSpeed)

    log.Printf("Start loading %v", path)
    err = common.RunNES(path, &cpu, maxCycles, quit, toDraw, bufferReady, audioOutput, emulatorActionsInput, &screenListeners, make(chan string, 100), AudioSampleRate, 0, nil)
    if err == common.MaxCyclesReached {
        log.Printf("%v complete", path)
    }
}

/* Find roms and show thumbnails of them, then let the user select one */
func romLoader(mainQuit context.Context, romLoaderState *RomLoaderState) error {
    /* for each rom call runNES() and pass in EmulatorInfiniteSpeed to let
     * the emulator run as fast as possible. Pass in a maxCycle of whatever
     * correlates to about 4 seconds of runtime. Save the screens produced
     * every 1s, so there should be about 4 screenshots. Then the thumbnail
     * should cycle through all the screenshots.
     * Let the user pick a thumbnail, and when selecting a thumbnail
     * return that nesfile so it can be played normally.
     */

    possibleRoms := make(chan PossibleRom, 1000)

    loaderQuit, loaderCancel := context.WithCancel(mainQuit)
    _ = loaderCancel

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
        go func(){
            defer wait.Done()

            for possibleRom := range possibleRoms {
                nesFile, err := nes.ParseNesFile(possibleRom.Path, false)
                if err != nil {
                    log.Printf("Unable to parse nes file %v: %v", possibleRom.Path, err)
                    continue
                }

                cpu, err := common.SetupCPU(nesFile, false, false)
                if err != nil {
                    log.Printf("Unable to setup cpu for %v: %v", possibleRom.Path, err)
                    continue
                }

                romId := possibleRom.RomId

                add := RomLoaderAdd{
                    Id: romId,
                    Path: possibleRom.Path,
                }

                select {
                    case romLoaderState.NewRom <- add:
                    case <-loaderQuit.Done():
                        return
                }

                /* Run the actual frame generation in a separate goroutine */
                generator := func(){
                    if !getCachedThumbnails(loaderQuit, romId, add.Path, romLoaderState.AddFrame) {
                        generateThumbnails(loaderQuit, cpu, romId, add.Path, romLoaderState.AddFrame, true)
                    }
                }

                select {
                    case generatorChannel <- generator:
                    case <-loaderQuit.Done():
                        return
                }
            }
        }()
    }

    var romId RomId
    err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
        if mainQuit.Err() != nil {
            return fmt.Errorf("quitting")
        }

        if nes.IsNESFile(path){
            romId += 1
            // log.Printf("Possible nes file %v", path)
            rom := PossibleRom{
                Path: path,
                RomId: romId,
            }

            select {
                case possibleRoms <- rom:
                case <-mainQuit.Done():
                    return fmt.Errorf("quitting")
            }
        }

        return nil
    })

    close(possibleRoms)
    wait.Wait()
    /* Wait till the writers of the generatorChannel have stopped */

    close(generatorChannel)
    generatorWait.Wait()

    log.Printf("Rom loader done")

    return err
}

func (loader *RomLoaderState) UpdateWindowSize(width int, height int){
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    loader.WindowSizeWidth = width
    loader.WindowSizeHeight = height
}

func (loader *RomLoaderState) GetSelectedRomInfo() (*RomLoaderInfo, bool) {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if loader.SelectedRomKey != "" {
        roms := loader.RomIdsAndPaths.All()
        index := loader.FindSortedIdIndex(roms, loader.SelectedRomKey)
        return loader.Roms[roms[index].Id], true
    }

    return nil, false
}

func (loader *RomLoaderState) GetSelectedRom() (string, bool) {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if loader.SelectedRomKey != "" {
        roms := loader.RomIdsAndPaths.All()
        index := loader.FindSortedIdIndex(roms, loader.SelectedRomKey)
        return roms[index].Path, true
    }

    return "", false
}

func (loader *RomLoaderState) moveSelection(count int){
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if loader.RomIdsAndPaths.Size() == 0 {
        return
    }

    roms := loader.GetFilteredRoms()

    currentIndex := loader.FindSortedIdIndex(roms, loader.SelectedRomKey)
    if currentIndex != -1 {
        length := len(roms)
        currentIndex = (currentIndex + count + length) % length
        if currentIndex < 0 {
            currentIndex = 0
        }
        loader.SelectedRomKey = roms[currentIndex].SortKey()
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

func (loader *RomLoaderState) SearchBackspace() {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if loader.RomIdsAndPaths.BackspaceFilter() {
        loader.updateSelectedRom()
    }
}

func (loader *RomLoaderState) SearchAdd(letter string) {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    loader.RomIdsAndPaths.AddFilter(letter)
    loader.updateSelectedRom()
}

func (loader *RomLoaderState) updateSelectedRom() {
    if loader.SelectedRomKey != "" {
        roms := loader.GetFilteredRoms()
        for _, rom := range roms {
            /* found the rom in the filtered list so its fine */
            if rom.SortKey() == loader.SelectedRomKey {
                return
            }
        }

        if len(roms) > 0 {
            loader.SelectedRomKey = roms[0].SortKey()
        } else {
            loader.SelectedRomKey = ""
        }
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

func (loader *RomLoaderState) FindSortedIdIndex(roms []*RomIdAndPath, path string) int {
    baseKey := filepath.Base(path)
    /* must hold the loader.Lock before calling this */
    index := sort.Search(len(roms), func (check int) bool {
        info := roms[check]
        return strings.Compare(baseKey, info.SortKey()) <= 0
    })

    if index == len(roms) {
        return -1
    }

    return int(index)
}

func (loader *RomLoaderState) FindRomIdByPath(path string) RomId {
    roms := loader.RomIdsAndPaths.All()
    index := loader.FindSortedIdIndex(roms, path)
    if index == -1 {
        return 0
    }

    return roms[index].Id
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

func (info *RomIdAndPath) Less(other *RomIdAndPath) bool {
    return strings.Compare(info.SortKey(), other.SortKey()) == -1
}

func (info *RomIdAndPath) Contains(filter string) bool {
    return strings.Contains(strings.ToLower(filepath.Base(info.Path)), strings.ToLower(filter))
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
    Thumbnail float32
}

func (loader *RomLoaderState) TileLayout() *TileLayout {
    return &loader.Layout
    /*
    return TileLayout{
        XStart: 50,
        YStart: 80,
        XSpace: 20,
        YSpace: 25,
        Thumbnail: 2,
    }
    */
}

func (loader *RomLoaderState) TilesPerRow(maxWidth int) int {
    count := 0
    layout := loader.TileLayout()
    x := float32(layout.XStart)
    width := nes.VideoWidth

    for x + float32(width) / layout.Thumbnail + 5 < float32(maxWidth) {
        x += float32(width) / layout.Thumbnail + float32(layout.XSpace)
        count += 1
    }

    return count
}

func (loader *RomLoaderState) TileRows(maxHeight int) int {
    layout := loader.TileLayout()

    height := float32(nes.VideoHeight - nes.OverscanPixels * 2)

    y := float32(layout.YStart)
    count := 0

    yDiff := height / layout.Thumbnail + float32(layout.YSpace)
    for y + yDiff < float32(maxHeight) {
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

func (loader *RomLoaderState) GetFilteredRoms() []*RomIdAndPath {
    return loader.RomIdsAndPaths.Filtered()
}

func (loader *RomLoaderState) ZoomIn() {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if loader.TileLayout().Thumbnail > 1 {
        loader.TileLayout().Thumbnail -= 0.2
    }
}

func (loader *RomLoaderState) ZoomOut() {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()
    if loader.TileLayout().Thumbnail < 4 {
        loader.TileLayout().Thumbnail += 0.2
    }
}

// draw part of the string in a new color where the substring is from 'startPosition' and goes for 'length' characters
func drawOverlayString(font *ttf.Font, renderer *sdl.Renderer, x int, y int, base string, startPosition int, length int, color sdl.Color) error {
    rendered := base[0:startPosition+1]
    // get the length of the text minus the last character
    startLength := gfx.TextWidth(font, rendered) - gfx.TextWidth(font, string(rendered[len(rendered)-1]))
    matched := base[startPosition:startPosition+length]
    // show the matched part of the selected rom
    return gfx.WriteFont(font, renderer, x + startLength, y, matched, color)
}

func maxTextWidth(font *ttf.Font, maxWidth int) int {
    if maxWidth <= 0 {
        return 0
    }

    size := gfx.TextWidth(font, "A")
    return maxWidth / size
}

func (loader *RomLoaderState) Render(maxWidth int, maxHeight int, font *ttf.Font, smallFont *ttf.Font, renderer *sdl.Renderer, textureManager *TextureManager) error {
    /* FIXME: this coarse grained lock will slow things down a bit */
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
    green := sdl.Color{R: 0, G: 255, B: 0, A: 255}

    showTiles := loader.GetFilteredRoms()
    gfx.WriteFont(font, renderer, 1, 1, fmt.Sprintf("Press enter to load a rom. Roms found %v (%v filtered)", loader.RomIdsAndPaths.Size(), len(showTiles)), white)

    layout := loader.TileLayout()

    overscanPixels := 8
    width := nes.VideoWidth
    height := nes.VideoHeight-nes.OverscanPixels*2
    x := float32(layout.XStart)
    y := float32(layout.YStart)

    selectedId := RomId(0)

    if loader.SelectedRomKey != "" && len(showTiles) > 0 {
        selectedIndex := loader.FindSortedIdIndex(showTiles, loader.SelectedRomKey)
        if selectedIndex == -1 {
            selectedIndex = 0
        }
        selectedId = showTiles[selectedIndex].Id

        selectedX := 100
        selectedY := font.Height() + 3

        // show the filename of the selected rom
        gfx.WriteFont(font, renderer, selectedX, selectedY, showTiles[selectedIndex].Path, white)

        if loader.RomIdsAndPaths.Filter() != "" {
            path := showTiles[selectedIndex].Path
            base := filepath.Base(path)
            // the path without the basename on it
            startPath := path[0:len(path)-len(base)]

            index := strings.Index(strings.ToLower(base), strings.ToLower(loader.RomIdsAndPaths.Filter()))
            if index != -1 {
                drawOverlayString(font, renderer, selectedX, selectedY, path, len(startPath) + index, len(loader.RomIdsAndPaths.Filter()), green)
            }
        }
    }

    err := renderer.SetDrawBlendMode(sdl.BLENDMODE_NONE)
    _ = err

    raw_pixels := make([]byte, width*height * 4)
    pixelFormat := gfx.FindPixelFormat()

    /* if the rom doesn't have any frames loaded then show a blank thumbnail */
    blankScreen := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)
    blankScreen.ClearToColor(0, 0, 0)

    outlineSize := 3

    start := loader.MinRenderIndex
    if start < 0 {
        start = 0
    }
    if start >= len(showTiles) {
        start = 0
    }

    end := start + loader.MaximumTiles()
    if end >= len(showTiles) {
        end = len(showTiles) - 1
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

    if loader.MinRenderIndex + loader.MaximumTiles() < len(showTiles) {
        if arrowInfo.Texture != nil {
            downY := maxHeight - 50
            if downY < 30 {
                downY = 30
            }
            renderDownArrow(10, downY, arrowInfo.Texture, renderer)
        }
    }

    gfx.WriteFont(font, renderer, 30, maxHeight - 30, loader.RomIdsAndPaths.Filter(), green)

    MaxNameSize := maxTextWidth(smallFont, int(float32(width) / layout.Thumbnail))

    for _, romIdAndPath := range showTiles[start:end+1] {
        info := loader.Roms[romIdAndPath.Id]
        frame, has := info.GetFrame()
        if !has {
            frame = blankScreen
        }

        /* Highlight the selected rom with a yellow outline */
        if selectedId == romIdAndPath.Id {
            renderer.SetDrawColor(255, 255, 0, 255)
            rect := sdl.Rect{X: int32(x-float32(outlineSize)), Y: int32(y-float32(outlineSize)), W: int32(float32(width) / layout.Thumbnail + float32(outlineSize*2)), H: int32(float32(height) / layout.Thumbnail + float32(outlineSize*2))}
            renderer.FillRect(&rect)
        }

        /* FIXME: cache these textures with the texture manager */
        common.RenderPixelsRGBA(frame, raw_pixels, overscanPixels)
        doRender(width, height, raw_pixels, int(x), int(y), int(float32(width) / layout.Thumbnail), int(float32(height) / layout.Thumbnail), pixelFormat, renderer)

        name := filepath.Base(info.Path)
        if len(name) > MaxNameSize {
            name = fmt.Sprintf("%v..", name[0:MaxNameSize-2])
        }

        gfx.WriteFont(smallFont, renderer, int(x), int(y + float32(height) / layout.Thumbnail + 1), name, white)

        x += float32(width) / layout.Thumbnail + float32(layout.XSpace)
        if x + float32(width) / layout.Thumbnail + 5 > float32(maxWidth) {
            x = float32(layout.XStart)
            y += float32(height) / layout.Thumbnail + float32(layout.YSpace)

            if y + float32(height) / layout.Thumbnail > float32(maxHeight) {
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
        selectedIndex := loader.FindSortedIdIndex(loader.RomIdsAndPaths.All(), loader.SelectedRomKey)
        distanceToMin = selectedIndex - loader.MinRenderIndex
    }

    newRomIdAndPath := RomIdAndPath{Id: rom.Id, Path: rom.Path}
    loader.RomIdsAndPaths.Add(&newRomIdAndPath)

    if loader.SelectedRomKey == "" {
        loader.MinRenderIndex = 0
        loader.SelectedRomKey = newRomIdAndPath.SortKey()
    } else {
        selectedIndex := loader.FindSortedIdIndex(loader.GetFilteredRoms(), loader.SelectedRomKey)
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
        Layout: TileLayout{
            XStart: 50,
            YStart: 80,
            XSpace: 20,
            YSpace: 25,
            Thumbnail: 2,
        },
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

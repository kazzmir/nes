package menu

import (
    "sync"
    "context"
    "log"
    "path/filepath"
    "os"
    "math"
    "math/rand/v2"
    "image"
    "fmt"
    "sort"
    "time"
    "io/fs"
    "strings"

    imagelib "image"
    "image/png"
    "image/color"

    "github.com/kazzmir/nes/cmd/nes/menu/filterlist"
    "github.com/kazzmir/nes/cmd/nes/common"
    // "github.com/kazzmir/nes/cmd/nes/gfx"
    "github.com/kazzmir/nes/cmd/nes/thread"
    "github.com/kazzmir/nes/data"
    nes "github.com/kazzmir/nes/lib"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
    "github.com/hajimehoshi/ebiten/v2/vector"
)

type RomId uint64

type RomLoaderAdd struct {
    Id RomId
    Path string
    File common.MakeFile
}

type RomLoaderFrame struct {
    Id RomId
    Frame *ebiten.Image
}

type RomLoaderInfo struct {
    Path string
    Frames []*ebiten.Image
    ShowFrame int
}

func (info *RomLoaderInfo) GetFrame() (*ebiten.Image, bool) {
    // FIXME: might need a lock here
    if len(info.Frames) > 0 {
        return info.Frames[info.ShowFrame], true
    }

    return nil, false
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

    Arrow *ebiten.Image

    CurrentScan string
    CurrentScanLock sync.Mutex

    renderOnce sync.Once

    blankScreen *ebiten.Image
    raw_pixels []byte
    views map[RomId]*ebiten.Image

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

func (loader *RomLoaderState) CurrentScanDescription(maxWidth int) string {
    loader.CurrentScanLock.Lock()
    defer loader.CurrentScanLock.Unlock()

    maxLength := maxWidth

    if loader.CurrentScan == "" {
        return "Scan complete"
    } else {
        if len(loader.CurrentScan) > maxLength {
            return "Scanning: " + loader.CurrentScan[0:maxLength] + "..."
        } else {
            return "Scanning: " + loader.CurrentScan
        }
    }
}

type PossibleRom struct {
    Path string
    File common.MakeFile
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
            uniqueIdentifier = fmt.Sprintf("%v-%v", time.Now().UnixNano(), rand.N(max-min) + min)
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

    programSha, err := os.ReadFile(meta)
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
            Frame: ebiten.NewImageFromImage(frame),
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
func convertToPng(screen nes.VirtualScreen) imagelib.Image {
    out := imagelib.NewRGBA(imagelib.Rect(0, 0, screen.Width, screen.Height))

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

type IgnoreMessages struct {}
func (ignore *IgnoreMessages) Add(message string) {}

func generateThumbnails(loaderQuit context.Context, cpu nes.CPUState, romId RomId, path string, makeFile common.MakeFile, addFrame chan<- RomLoaderFrame, doCache bool, pixelPool *sync.Pool){
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

    bufferReady := make(chan bool, 1)

    buffer := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)

    var err error
    cachedSha := ""
    if doCache {
        open, err := makeFile()
        if err == nil {
            cachedSha, err = common.GetSha256From(open)
            if err != nil {
                log.Printf("Could not get sha of '%v': %v", path, err)
                cachedSha = "x"
            }
            open.Close()
        } else {
            cachedSha = "x"
        }
    }

    frameNumber := 1
    count := 0
    handleDraw := func() error {
        select {
            case <-bufferReady:
                count += 1
                /* every 60 frames should be 1 second */
                if count == 60 {
                    raw_pixels := pixelPool.Get().([]byte)
                    frame := RomLoaderFrame{
                        Id: romId,
                        Frame: nesToImage(buffer, raw_pixels),
                    }
                    pixelPool.Put(raw_pixels)

                    if doCache {
                        err := saveCachedFrame(frameNumber, cachedSha, path, buffer)
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
                            return nil
                    }
                    count = 0
                }

            default:
        }

        return nil
    }

    /* don't load more than 30s worth */
    const maxCycles = uint64(30 * nes.CPUSpeed)

    log.Printf("Start loading %v", path)
    err = common.RunNES(path, &cpu, maxCycles, quit, bufferReady, buffer, audioOutput, emulatorActionsInput, &screenListeners, &IgnoreMessages{}, AudioSampleRate, 0, nil, handleDraw)
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

    generatorChannel := make(chan func(), 500)

    generatorGroup := thread.NewThreadGroup(loaderQuit)
    romGroup := thread.NewThreadGroup(loaderQuit)

    for range 4 {
        generatorGroup.Spawn(func(){
            for generator := range generatorChannel {
                generator()
            }
        })
    }

    pixelPool := sync.Pool{
        New: func() any {
            return make([]byte, nes.VideoWidth * (nes.VideoHeight - nes.OverscanPixels * 2) * 4)
        },
    }

    /* Have 4 go routines running roms */
    for range 4 {
        romGroup.Spawn(func(){
            for possibleRom := range possibleRoms {
                openFile, err := possibleRom.File()
                if err != nil {
                    log.Printf("Unable to open file %v: %v", possibleRom.Path, err)
                    continue
                }
                defer openFile.Close()

                nesFile, err := nes.ParseNes(openFile, false, possibleRom.Path)
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
                    File: possibleRom.File,
                }

                select {
                    case romLoaderState.NewRom <- add:
                    case <-romGroup.Done():
                        return
                }

                /* Run the actual frame generation in a separate goroutine */
                generator := func(){
                    if !getCachedThumbnails(loaderQuit, romId, add.Path, romLoaderState.AddFrame) {
                        generateThumbnails(loaderQuit, cpu, romId, add.Path, possibleRom.File, romLoaderState.AddFrame, true, &pixelPool)
                    }
                }

                select {
                    case generatorChannel <- generator:
                    case <-romGroup.Done():
                        return
                }
            }
        })
    }

    var romId RomId

    err := fs.WalkDir(data.RomsFS, ".", func(path string, dir fs.DirEntry, err error) error {
        if mainQuit.Err() != nil {
            return fmt.Errorf("quitting")
        }

        if dir.IsDir() {
            return nil
        }

        romLoaderState.CurrentScanLock.Lock()
        romLoaderState.CurrentScan = path
        romLoaderState.CurrentScanLock.Unlock()

        romId += 1
        // log.Printf("Possible nes file %v", path)
        open := func() (fs.File, error){
            return data.RomsFS.Open(path)
        }
        rom := PossibleRom{
            Path: "<embedded>/" + path,
            RomId: romId,
            File: open,
        }

        select {
            case possibleRoms <- rom:
            case <-mainQuit.Done():
                return fmt.Errorf("quitting")
        }

        return nil
    })

    err = filepath.WalkDir(".", func(path string, dir fs.DirEntry, err error) error {
        if mainQuit.Err() != nil {
            return fmt.Errorf("quitting")
        }

        romLoaderState.CurrentScanLock.Lock()
        romLoaderState.CurrentScan = path
        romLoaderState.CurrentScanLock.Unlock()

        if nes.IsNESFile(path){
            romId += 1
            // log.Printf("Possible nes file %v", path)
            open := func() (fs.File, error){
                return os.Open(path)
            }
            rom := PossibleRom{
                Path: path,
                RomId: romId,
                File: open,
            }

            select {
                case possibleRoms <- rom:
                case <-mainQuit.Done():
                    return fmt.Errorf("quitting")
            }
        }

        return nil
    })

    romLoaderState.CurrentScanLock.Lock()
    romLoaderState.CurrentScan = ""
    romLoaderState.CurrentScanLock.Unlock()

    close(possibleRoms)
    romGroup.Wait()
    /* Wait till the writers of the generatorChannel have stopped */

    close(generatorChannel)
    generatorGroup.Wait()

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

func (loader *RomLoaderState) GetSelectedRom() (string, common.MakeFile, bool) {
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    if loader.SelectedRomKey != "" {
        roms := loader.RomIdsAndPaths.All()
        index := loader.FindSortedIdIndex(roms, loader.SelectedRomKey)
        return roms[index].Path, roms[index].File, true
    }

    return "", nil, false
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
    File common.MakeFile
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

/*
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
*/

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
func drawOverlayString(font text.Face, out *ebiten.Image, x int, y int, base string, startPosition int, length int, col color.Color) error {
    rendered := base[0:startPosition+1]
    // get the length of the text minus the last character

    width1, _ := text.Measure(rendered, font, 1)
    width2, _ := text.Measure(string(rendered[len(rendered)-1]), font, 1)

    startLength := width1 - width2
    matched := base[startPosition:startPosition+length]
    // show the matched part of the selected rom

    var options text.DrawOptions
    options.GeoM.Translate(float64(x) + startLength, float64(y))
    options.ColorScale.ScaleWithColor(col)
    text.Draw(out, matched, font, &options)
    return nil
}

func maxTextWidth(face text.Face, maxWidth int) int {
    if maxWidth <= 0 {
        return 0
    }

    width, _ := text.Measure("A", face, 1)

    if width < 1 {
        width = 1
    }
    return int(float64(maxWidth) / width)
}

func (loader *RomLoaderState) GetRomView(romId RomId, width int, height int) *ebiten.Image {
    view, has := loader.views[romId]
    if !has {
        view = ebiten.NewImage(width, height)
        loader.views[romId] = view
    }
    return view
}

func (loader *RomLoaderState) Render(font text.Face, smallFont text.Face, screen *ebiten.Image) error {
    /* FIXME: this coarse grained lock will slow things down a bit */
    loader.Lock.Lock()
    defer loader.Lock.Unlock()

    loader.renderOnce.Do(func(){
        var options text.DrawOptions
        options.GeoM.Translate(2, 2)
        options.GeoM.Scale(2, 2)
        text.Draw(loader.blankScreen, "No Image", font, &options)
    })

    // white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
    green := color.RGBA{R: 0, G: 255, B: 0, A: 255}

    showTiles := loader.GetFilteredRoms()

    maxWidth := screen.Bounds().Dx()
    maxHeight := screen.Bounds().Dy()

    fontWidth, fontHeight := text.Measure("A", font, 1)

    var textOptions text.DrawOptions
    textOptions.GeoM.Translate(1, 1)
    text.Draw(screen, fmt.Sprintf("Press enter to load a rom. Roms found %v (%v filtered). %v", loader.RomIdsAndPaths.Size(), len(showTiles), loader.CurrentScanDescription(int((float64(maxWidth) / fontWidth) - 40))), font, &textOptions)

    layout := loader.TileLayout()

    // overscanPixels := 8
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
        selectedY := fontHeight + 3

        // show the filename of the selected rom
        var options text.DrawOptions
        options.GeoM.Translate(float64(selectedX), float64(selectedY))
        text.Draw(screen, showTiles[selectedIndex].Path, font, &options)

        if loader.RomIdsAndPaths.Filter() != "" {
            path := showTiles[selectedIndex].Path
            base := filepath.Base(path)
            // the path without the basename on it
            startPath := path[0:len(path)-len(base)]

            index := strings.Index(strings.ToLower(base), strings.ToLower(loader.RomIdsAndPaths.Filter()))
            if index != -1 {
                drawOverlayString(font, screen, selectedX, int(selectedY), path, len(startPath) + index, len(loader.RomIdsAndPaths.Filter()), green)
            }
        }
    }

    outlineSize := 3
    _ = outlineSize

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

    /*
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
    */

    if loader.MinRenderIndex != 0 && loader.Arrow != nil {
        var options ebiten.DrawImageOptions
        options.GeoM.Translate(10, 30)
        screen.DrawImage(loader.Arrow, &options)
    }

    if loader.MinRenderIndex + loader.MaximumTiles() < len(showTiles) {
        if loader.Arrow != nil {
            downY := maxHeight - 50
            if downY < 30 {
                downY = 30
            }

            var options ebiten.DrawImageOptions
            options.GeoM.Translate(10, float64(downY))
            options.GeoM.Scale(1, -1)
            screen.DrawImage(loader.Arrow, &options)
        }
    }

    textOptions.GeoM.Reset()
    textOptions.GeoM.Translate(30, float64(maxHeight - 30))
    textOptions.ColorScale.ScaleWithColor(green)
    text.Draw(screen, loader.RomIdsAndPaths.Filter(), font, &textOptions)

    MaxNameSize := maxTextWidth(smallFont, int(float32(width) / layout.Thumbnail))

    for _, romIdAndPath := range showTiles[start:end+1] {
        // log.Printf("Rendering rom id %v path %v at x=%v y=%v", romIdAndPath.Id, romIdAndPath.Path, x, y)

        info := loader.Roms[romIdAndPath.Id]
        frame, has := info.GetFrame()
        if !has {
            frame = loader.blankScreen
        }

        /* Highlight the selected rom with a yellow outline */
        if selectedId == romIdAndPath.Id {
            /*
            renderer.SetDrawColor(255, 255, 0, 255)
            rect := sdl.Rect{X: int32(x-float32(outlineSize)), Y: int32(y-float32(outlineSize)), W: int32(float32(width) / layout.Thumbnail + float32(outlineSize*2)), H: int32(float32(height) / layout.Thumbnail + float32(outlineSize*2))}
            renderer.FillRect(&rect)
            */

            x1 := x - float32(outlineSize)
            y1 := y - float32(outlineSize)
            width := float32(nes.VideoWidth) / layout.Thumbnail + float32(outlineSize*2)
            height := float32(nes.VideoHeight - nes.OverscanPixels*2) / layout.Thumbnail + float32(outlineSize*2)

            vector.FillRect(screen, x1, y1, width, height, color.RGBA{R: 255, G: 255, B: 0, A: 255}, true)
        }

        var options ebiten.DrawImageOptions
        options.GeoM.Scale(1.0 / float64(layout.Thumbnail), 1.0 / float64(layout.Thumbnail))
        options.GeoM.Translate(float64(x), float64(y))
        screen.DrawImage(frame, &options)

        name := filepath.Base(info.Path)
        if len(name) > MaxNameSize {
            name = fmt.Sprintf("%v..", name[0:MaxNameSize-2])
        }

        textOptions.GeoM.Reset()
        textOptions.GeoM.Translate(float64(x), float64(y + float32(height) / layout.Thumbnail + 1))
        textOptions.ColorScale.Reset()
        text.Draw(screen, name, smallFont, &textOptions)

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

    newRomIdAndPath := RomIdAndPath{Id: rom.Id, Path: rom.Path, File: rom.File}
    loader.RomIdsAndPaths.Add(&newRomIdAndPath)

    if loader.SelectedRomKey == "" {
        loader.MinRenderIndex = 0
        loader.SelectedRomKey = newRomIdAndPath.SortKey()
    } else {
        selectedIndex := loader.FindSortedIdIndex(loader.GetFilteredRoms(), loader.SelectedRomKey)
        loader.MinRenderIndex = selectedIndex - distanceToMin
    }
}

func nesToImage(screen nes.VirtualScreen, raw_pixels []byte) *ebiten.Image {
    width := nes.VideoWidth
    height := nes.VideoHeight-nes.OverscanPixels*2
    overscanPixels := 8
    view := ebiten.NewImage(width, height)
    common.RenderPixelsRGBA(screen, raw_pixels, overscanPixels)
    view.WritePixels(raw_pixels)
    return view
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

func loadArrowPicture() (image.Image, error) {
    reader, err := data.OpenFile("arrow.png")
    if err != nil {
        return nil, err
    }

    return png.Decode(reader)
}

func MakeRomLoaderState(quit context.Context, windowWidth int, windowHeight int) *RomLoaderState {
    var arrowImg *ebiten.Image
    arrow, err := loadArrowPicture()
    if err != nil {
        log.Printf("Could not load arrow image: %v", err)
        arrow = nil
    } else {
        arrowImg = ebiten.NewImageFromImage(arrow)
    }
    state := RomLoaderState{
        Roms: make(map[RomId]*RomLoaderInfo),
        NewRom: make(chan RomLoaderAdd, 5),
        AddFrame: make(chan RomLoaderFrame, 5),
        MinRenderIndex: 0,
        WindowSizeWidth: windowWidth,
        WindowSizeHeight: windowHeight,
        Arrow: arrowImg,
        blankScreen: ebiten.NewImage(nes.VideoWidth, nes.VideoHeight),
        views: make(map[RomId]*ebiten.Image),
        raw_pixels: make([]byte, nes.VideoWidth * (nes.VideoHeight-nes.OverscanPixels*2) * 4),
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

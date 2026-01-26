package main

import (
    "fmt"
    "log"
    "strconv"
    "os"
    "os/signal"
    // "io"
    // "io/fs"
    "path/filepath"
    // "math"
    "strings"
    "bufio"

    // "encoding/binary"
    // "bytes"
    "time"
    "sync"
    "context"
    "runtime/pprof"
    // "runtime"

    nes "github.com/kazzmir/nes/lib"
    "github.com/kazzmir/nes/util"

    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/kazzmir/nes/cmd/nes/gfx"
    "github.com/kazzmir/nes/cmd/nes/menu"
    "github.com/kazzmir/nes/cmd/nes/debug"
    // "github.com/kazzmir/nes/data"

    // rdebug "runtime/debug"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
)

/*
func setupAudio(sampleRate float32) (sdl.AudioDeviceID, error) {
    var audioSpec sdl.AudioSpec
    var obtainedSpec sdl.AudioSpec

    audioSpec.Freq = int32(sampleRate)
    audioSpec.Format = sdl.AUDIO_F32LSB
    audioSpec.Channels = 1
    audioSpec.Samples = 1024
    // audioSpec.Callback = sdl.AudioCallback(C.generate_audio_c)
    audioSpec.Callback = nil
    audioSpec.UserData = nil

    var device sdl.AudioDeviceID
    var err error
    sdl.Do(func(){
        device, err = sdl.OpenAudioDevice("", false, &audioSpec, &obtainedSpec, sdl.AUDIO_ALLOW_FORMAT_CHANGE)
    })
    return device, err
}
*/


func stripExtension(path string) string {
    extension := filepath.Ext(path)
    if len(extension) > 0 {
        return path[0:len(path) - len(extension)]
    }

    return path
}

func RecordMp4(stop context.Context, romName string, overscanPixels int, sampleRate int, screenListeners *common.ScreenListeners) error {
    video_channel := make(chan nes.VirtualScreen, 2)
    audio_channel := make(chan []float32, 2)

    mp4Path := fmt.Sprintf("%v-%v.mp4", romName, time.Now().Format("2006-01-02-15:04:05"))

    screenListeners.AddVideoListener(video_channel)
    screenListeners.AddAudioListener(audio_channel)

    go func(){
        defer screenListeners.RemoveAudioListener(audio_channel)
        defer screenListeners.RemoveVideoListener(video_channel)
        err := util.RecordMp4(stop, mp4Path, overscanPixels, sampleRate, video_channel, audio_channel)
        if err != nil {
            log.Printf("Error recording mp4: %v", err)
        }
    }()

    return nil
}

type AudioActions interface {
}

type AudioToggle struct {
}

type AudioQueryEnabled struct {
    Response chan bool
}

func makeAudioWorker(audio <-chan []float32, audioActions <-chan AudioActions, mainQuit context.Context) func() {
    /* FIXME convert to ebiten
    if audioDevice != 0 {
        / * runNES will generate arrays of samples that we enqueue into the SDL audio system * /
        return func(){
            // var buffer bytes.Buffer
            var audioBytes []byte
            enabled := true
            for {
                select {
                    case <-mainQuit.Done():
                        return
                    case action := <-audioActions:
                        switch action.(type) {
                            case *AudioToggle:
                                enabled = !enabled
                            case *AudioQueryEnabled:
                                query := action.(*AudioQueryEnabled)
                                query.Response <- enabled
                        }
                    case samples := <-audio:
                        if !enabled {
                            break
                        }
                        // log.Printf("Prepare audio to queue")
                        // log.Printf("Enqueue data %v", samples)
                        // buffer.Reset()
                        / * convert []float32 into []byte * /
                        // slow method that does allocations
                        // binary.Write(&buffer, binary.LittleEndian, samples)

                        // fast method with no allocations, copied from binary.Write
                        totalSize := len(samples) * 4
                        for len(audioBytes) < totalSize {
                            audioBytes = append(audioBytes, 0)
                        }

                        for i, sample := range samples {
                            binary.LittleEndian.PutUint32(audioBytes[4*i:], math.Float32bits(sample))
                        }

                        // log.Printf("Enqueue audio")
                        var err error
                        sdl.Do(func(){
                            err = sdl.QueueAudio(audioDevice, audioBytes[:totalSize])
                        })
                        if err != nil {
                            log.Printf("Error: could not queue audio data: %v", err)
                            return
                        }
                }
            }
        }
    } else {
        return func(){
            for {
                select {
                    case <-mainQuit.Done():
                        return
                    case action := <-audioActions:
                        switch action.(type) {
                            case *AudioQueryEnabled:
                                query := action.(*AudioQueryEnabled)
                                query.Response <- false
                        }
                    case <-audio:
                }
            }
        }
    }
    */
    return func(){}
}

/* must be called in a sdl.Do */
func doRenderNesPixels(width int, height int, raw_pixels []byte, pixelFormat gfx.PixelFormat, out *ebiten.Image) error {

    /*
    pixels := C.CBytes(raw_pixels)
    defer C.free(pixels)

    depth := 8 * 4 // RGBA8888
    pitch := int(width) * int(depth) / 8

    // pixelFormat := sdl.PIXELFORMAT_ABGR8888

    / * pixelFormat should be ABGR8888 on little-endian (x86) and
     * RBGA8888 on big-endian (arm)
     * /

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


    renderer.SetLogicalSize(int32(width), int32(height))
    err = renderer.Copy(texture, nil, nil)
    if err != nil {
        log.Printf("Warning: could not copy texture to renderer: %v\n", err)
    }

    renderer.SetLogicalSize(0, 0)
    */

    return nil
}


type NesAction interface {
}

type NesActionDebugger struct {
}

type NesActionLoad struct {
    File nes.NESFile
}

type NesActionRestart struct {
}

type EmulatorMessage struct {
    Message string
    DeathTime time.Time
}

type DefaultRenderLayer struct {
    RenderFunc func(gfx.RenderInfo) error
    Index int
}

func (layer *DefaultRenderLayer) Render(info gfx.RenderInfo) error {
    return layer.RenderFunc(info)
}

func (layer *DefaultRenderLayer) ZIndex() int {
    return layer.Index
}

type EmulatorMessageLayer struct {
    /* these messages appear in the bottom right */
    emulatorMessages []EmulatorMessage
    Index int
    ReceiveMessages chan string
    Lock sync.Mutex
}

func (layer *EmulatorMessageLayer) ZIndex() int {
    return layer.Index
}

func (layer *EmulatorMessageLayer) Render(renderInfo gfx.RenderInfo) error {
    // FIXME
    /*
    windowWidth, windowHeight := renderInfo.Window.GetSize()

    font := renderInfo.SmallFont

    layer.Lock.Lock()
    messages := gfx.CopyArray(layer.emulatorMessages)
    layer.Lock.Unlock()

    y := int(windowHeight) - font.Height() - 1
    now := time.Now()
    for i := len(messages)-1; i >= 0; i-- {
        message := messages[i]
        if message.DeathTime.After(now){
            x := int(windowWidth) - 100
            remaining := message.DeathTime.Sub(now)
            alpha := 255

            white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
            red := sdl.Color{R: 255, G: 0, B: 0, A: 255}

            N := 800

            color := gfx.InterpolateColor(red, white, N, N - int((remaining - time.Millisecond*500) / time.Millisecond))

            if remaining < time.Millisecond * 500 {
                alpha = int(255 * float64(remaining) / (float64(time.Millisecond) * 500))
                if alpha > 255 {
                    alpha = 255
                }
                / * strangely if alpha=0 it renders without transparency so the pixels are fully white * /
                if alpha < 1 {
                    alpha = 1
                }

                color = white
            }
            color.A = uint8(alpha)
            // white := sdl.Color{R: 255, G: 255, B: 255, A: uint8(alpha)}
            // log.Printf("Write message '%v' at %v, %v remaining=%v color=%v", message, x, y, remaining, white)
            gfx.WriteFont(font, renderInfo.Renderer, x, y, message.Message, color)
            y -= font.Height() + 2
        }
    }
            */

    return nil
}

func (layer *EmulatorMessageLayer) Run(quit context.Context){
    emulatorMessageTicker := time.NewTicker(time.Second * 1)
    defer emulatorMessageTicker.Stop()
    maxEmulatorMessages := 10

    for {
        select {
            case <-quit.Done():
                return
            case message := <-layer.ReceiveMessages:
                layer.Lock.Lock()
                layer.emulatorMessages = append(layer.emulatorMessages, EmulatorMessage{
                    Message: message,
                    DeathTime: time.Now().Add(time.Millisecond * 1500),
                })
                if len(layer.emulatorMessages) > maxEmulatorMessages {
                    layer.emulatorMessages = layer.emulatorMessages[len(layer.emulatorMessages) - maxEmulatorMessages:len(layer.emulatorMessages)]
                }
                layer.Lock.Unlock()
                /* remove deceased messages */
            case <-emulatorMessageTicker.C:
                now := time.Now()
                layer.Lock.Lock()
                i := 0
                for i < len(layer.emulatorMessages) {
                    /* find the first non-dead message */
                    if layer.emulatorMessages[i].DeathTime.Before(now) {
                        i += 1
                    } else {
                        break
                    }
                }
                layer.emulatorMessages = layer.emulatorMessages[i:]
                layer.Lock.Unlock()
        }
    }
}

type OverlayMessageLayer struct {
    Message string
    Index int
}

func (layer *OverlayMessageLayer) ZIndex() int {
    return layer.Index
}

func (layer *OverlayMessageLayer) Render(info gfx.RenderInfo) error {
    /*
    width, height := info.Window.GetSize()

    font := info.Font
    renderer := info.Renderer

    black := sdl.Color{R: 0, G: 0, B: 0, A: 200}
    white := sdl.Color{R: 255, G: 255, B: 255, A: 200}
    messageLength := gfx.TextWidth(font, layer.Message)
    x := int(width)/2 - messageLength / 2
    y := int(height)/2
    renderer.SetDrawColor(black.R, black.G, black.B, black.A)
    renderer.FillRect(&sdl.Rect{X: int32(x - 10), Y: int32(y - 10), W: int32(messageLength + 10 + 5), H: int32(font.Height() + 10 + 5)})

    gfx.WriteFont(font, renderer, x, y, layer.Message, white)
    */
    return nil
}

/*
func getWindowIdFromEvent(event sdl.Event) uint32 {
    switch event.GetType() {
        case sdl.TEXTEDITING:
            text_event, ok := event.(*sdl.TextEditingEvent)
            if ok {
                return text_event.WindowID
            }
        case sdl.TEXTINPUT:
            text_input, ok := event.(*sdl.TextInputEvent)
            if ok {
                return text_input.WindowID
            }
        case sdl.WINDOWEVENT:
            window_event, ok := event.(*sdl.WindowEvent)
            if ok {
                return window_event.WindowID
            }
        case sdl.KEYDOWN, sdl.KEYUP:
            keyboard_event, ok := event.(*sdl.KeyboardEvent)
            if ok {
                return keyboard_event.WindowID
            }
    }

    log.Printf("Warning: unknown event type: %v", event.GetType())

    / * FIXME: what is the invalid window id * /
    return 0
}
*/

type ReplayKeysInput struct {
    Cpu *nes.CPUState
    File *os.File
    Scanner *bufio.Scanner
    Buttons nes.ButtonMapping

    NextCycle uint64
    NextChange map[nes.Button]bool
}

func parseReplayLine(line string) (uint64, map[nes.Button]bool) {
    var cycle uint64
    mapping := make(map[nes.Button]bool)

    /* 2442067: Start=true Select=true ... */

    parts := strings.Split(line, ":")
    if len(parts) != 2 {
        /* invalid line */
        return 0, mapping
    }
    cycle, err := strconv.ParseUint(parts[0], 10, 64)
    if err != nil {
        cycle = 0
    }

    /* split buttons, and parse each one into its button value and bool value */
    for _, buttonPart := range strings.Fields(parts[1]) {
        buttonParts := strings.Split(buttonPart, "=")
        if len(buttonParts) != 2 {
            continue
        }
        button := nes.NameToButton(buttonParts[0])
        value, err := strconv.ParseBool(buttonParts[1])
        if err != nil {
            continue
        }

        mapping[button] = value
    }

    return cycle, mapping
}

func (replay *ReplayKeysInput) Get() nes.ButtonMapping {
    /*
    mapping := make(nes.ButtonMapping)

    mapping[nes.ButtonIndexA] = false
    mapping[nes.ButtonIndexB] = false
    mapping[nes.ButtonIndexSelect] = false
    mapping[nes.ButtonIndexStart] = false
    mapping[nes.ButtonIndexUp] = false
    mapping[nes.ButtonIndexDown] = false
    mapping[nes.ButtonIndexLeft] = false
    mapping[nes.ButtonIndexRight] = false

    return mapping
    */

    currentCycle := replay.Cpu.Cycle
    /* keep reading lines from the replay file and apply the changes to replay.Buttons
     * until we reach the current cycle
     */

    for replay.NextCycle <= currentCycle {
        if replay.NextChange != nil {
            // log.Printf("Cycle: %v Replay input %v\n", currentCycle, replay.NextCycle)
            for button, value := range replay.NextChange {
                // log.Printf("  change %v to %v\n", nes.ButtonName(button), value)
                replay.Buttons[button] = value
            }
        }

        if replay.Scanner.Scan() {
            line := replay.Scanner.Text()
            replay.NextCycle, replay.NextChange = parseReplayLine(line)
        } else {
            /* ran out of lines of input */
            replay.NextChange = nil
            break
        }
    }

    /*
    for button, value := range replay.Buttons {
        log.Printf(" replay button %v: %v", nes.ButtonName(button), value)
    }
    */

    return replay.Buttons
}

func (replay *ReplayKeysInput) Close() {
    replay.File.Close()
}

func makeReplayKeys(cpu *nes.CPUState, replayKeysPath string) (*ReplayKeysInput, error) {
    file, err := os.Open(replayKeysPath)
    if err != nil {
        return nil, err
    }

    return &ReplayKeysInput{
        Cpu: cpu,
        File: file,
        Buttons: make(nes.ButtonMapping),
        Scanner: bufio.NewScanner(file),
    }, nil
}

/*
func loadTTF(path string, size int) (*ttf.Font, error) {
    file, err := data.OpenFile(path)
    if err != nil {
        return nil, err
    }

    defer file.Close()

    // make rwops, use OpenFontRW, close rwops
    memory, err := io.ReadAll(file)
    if err != nil {
        return nil, err
    }

    rwops, err := sdl.RWFromMem(memory)
    if err != nil {
        return nil, err
    }

    // defer rwops.Close()

    out, err := ttf.OpenFontRW(rwops, 1, size)
    if err != nil {
        rwops.Close()
        return nil, err
    } else {
        // the memory must exist longer than the rwops. we can't add a finalizer directly to the rwops
        // because it is not a golang object, but rather allocated by sdl directly using malloc()
        // instead we add the finalizer to the font, which is going to close the rwops anyway
        runtime.SetFinalizer(out, func(font *ttf.Font){
            memory = nil
        })
        return out, nil
    }
}
*/

func RunNES(path string, debugCpu bool, debugPpu bool, maxCycles uint64, windowSizeMultiple int, recordOnStart bool, desiredFps int, recordInput bool, replayKeys string) error {
    nesChannel := make(chan NesAction, 10)
    doMenu := make(chan bool, 5)
    renderOverlayUpdate := make(chan string, 5)

    var renderManager gfx.RenderManager

    if path != "" {
        log.Printf("Opening NES file '%v'", path)
        nesFile, err := nes.ParseNesFile(path, true)
        if err != nil {
            return err
        }
        nesChannel <- &NesActionLoad{File: nesFile}
    } else {
        /* if no nes file given then just load the main menu */
        doMenu <- true
        renderOverlayUpdate <- "No ROM loaded"
    }

    // force a software renderer
    /*
    if !util.HasGlxinfo() {
        sdl.Do(func(){
            sdl.SetHint(sdl.HINT_RENDER_DRIVER, "software")
        })
    }
    */

    // log.Printf("Initializing SDL")

    var err error
    _ = err
    /*
    sdl.Do(func(){
        err = sdl.Init(sdl.INIT_EVERYTHING)
    })
    if err != nil {
        return err
    }
    defer sdl.Do(func(){
        sdl.Quit()
    })

    sdl.Do(func(){
        sdl.DisableScreenSaver()
    })
    defer sdl.Do(func(){
        sdl.EnableScreenSaver()
    })
    */

    log.Printf("Create window")
    /* to resize the window */
    /*
    var window *sdl.Window
    var renderer *sdl.Renderer
    */

    /* 7/5/2021: its apparently very important that the window and renderer be created
     * in the sdl thread via sdl.Do. If the renderer calls are in sdl.Do, then so must
     * also be the creation of the window and the renderer. Initially I did not have
     * the creation of the window and renderer in sdl.Do, and thus in opengl mode
     * the window would not be rendered.
     */
     /*
    sdl.Do(func(){
        window, renderer, err = sdl.CreateWindowAndRenderer(
            int32(nes.VideoWidth * windowSizeMultiple),
            int32((nes.VideoHeight - nes.OverscanPixels * 2) * windowSizeMultiple),
            sdl.WINDOW_SHOWN | sdl.WINDOW_RESIZABLE)

        if window != nil {
            window.SetTitle("Nes Emulator")
        }
    })
    */

    /*
    window, err := sdl.CreateWindow("nes",
                                    sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
                                    int32(nes.VideoWidth * windowSizeMultiple),
                                    int32((nes.VideoHeight - nes.OverscanPixels * 2) * windowSizeMultiple),
                                    sdl.WINDOW_SHOWN | sdl.WINDOW_RESIZABLE)
    if err != nil {
        return err
    }
    defer window.Destroy()

    softwareRenderer := true
    _ = softwareRenderer
    // renderer, err := sdl.CreateSoftwareRenderer(surface)
    // renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_SOFTWARE)

    log.Printf("Create renderer")
    / * Create an accelerated renderer * /
    // renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
    renderer, err := sdl.CreateRenderer(window, -1, 0)
    */

    /* debug stuff
    numDrivers, err := sdl.GetNumRenderDrivers()
    if err != nil {
        log.Printf("Could not get the number of render drivers\n")
    } else {
        for i := 0; i < numDrivers; i++ {
            var renderInfo sdl.RendererInfo
            _, err = sdl.GetRenderDriverInfo(0, &renderInfo)
            if err != nil {
                log.Printf("Could not get render driver info: %v\n", err)
            } else {
                log.Printf("Render driver info %v\n", i + 1)
                log.Printf(" Name: %v\n", renderInfo.Name)
                log.Printf(" Flags: %v\n", renderInfo.Flags)
                log.Printf(" Number of texture formats: %v\n", renderInfo.NumTextureFormats)
                log.Printf(" Texture formats: %v\n", renderInfo.TextureFormats)
                log.Printf(" Max texture width: %v\n", renderInfo.MaxTextureWidth)
                log.Printf(" Max texture height: %v\n", renderInfo.MaxTextureHeight)
            }
        }
    }
    */

    const AudioSampleRate float32 = 44100

    /*
    err = mix.Init(mix.INIT_OGG)
    if err != nil {
        log.Printf("Could not initialize SDL mixer: %v", err)
    } else {
        err = mix.OpenAudio(int(AudioSampleRate), sdl.AUDIO_F32LSB, 2, 4096)
        if err != nil {
            log.Printf("Could not open mixer audio: %v", err)
        }
    }
    */

    /*
    renderInfo, err := renderer.GetInfo()
    if err != nil {
        log.Printf("Could not get render info from renderer: %v\n", err)
    } else {
        log.Printf("Current render info\n")
        log.Printf(" Name: %v\n", renderInfo.Name)
        log.Printf(" Flags: %v\n", renderInfo.Flags)
        log.Printf(" Number of texture formats: %v\n", renderInfo.NumTextureFormats)
        var buffer bytes.Buffer
        for texture := uint32(0); texture < renderInfo.NumTextureFormats; texture++ {
            value := uint(renderInfo.TextureFormats[texture])
            buffer.WriteString(sdl.GetPixelFormatName(value))
            buffer.WriteString(" ")
        }
        // log.Printf(" Texture formats: %v\n", renderInfo.TextureFormats)
        log.Printf(" Texture formats: %v\n", buffer.String())
        log.Printf(" Max texture width: %v\n", renderInfo.MaxTextureWidth)
        log.Printf(" Max texture height: %v\n", renderInfo.MaxTextureHeight)
    }
    */
   
    /*
    audioDevice, err := setupAudio(AudioSampleRate)
    if err != nil {
        log.Printf("Warning: could not set up audio: %v", err)
        audioDevice = 0
    } else {
        defer sdl.CloseAudioDevice(audioDevice)
        log.Printf("Opened SDL audio device %v", audioDevice)
        sdl.PauseAudioDevice(audioDevice, false)
    }
    */

    var waiter sync.WaitGroup

    mainQuit, mainCancel := context.WithCancel(context.Background())
    defer mainCancel()

    signalChannel := make(chan os.Signal, 10)
    signal.Notify(signalChannel, os.Interrupt)

    go func(){
        for i := 0; i < 2; i++ {
            select {
                // case <-mainQuit.Done():
                case <-signalChannel:
                    if i == 0 {
                        log.Printf("Shutting down due to signal")
                        mainCancel()
                        go func(){
                            time.Sleep(2 * time.Second)
                            log.Printf("Bailing..")
                            os.Exit(1)
                        }()
                    } else {
                        log.Printf("Hard kill")
                        os.Exit(1)
                    }
            }
        }
    }()

    toDraw := make(chan nes.VirtualScreen, 1)
    bufferReady := make(chan nes.VirtualScreen, 1)

    /*
    pixelFormat := gfx.FindPixelFormat()
    log.Printf("Using pixel format %v\n", sdl.GetPixelFormatName(uint(pixelFormat)))
    */

    var font text.Face
    var smallFont text.Face
    /*
    err = ttf.Init()
    if err != nil {
        return fmt.Errorf("Unable to initialize ttf: %v", err)
    }

    defer ttf.Quit()

    font, err := loadTTF("DejaVuSans.ttf", 20)
    if err != nil {
        return fmt.Errorf("Unable to load font size 20: %v", err)
    }
    defer font.Close()

    smallFont, err := loadTTF("DejaVuSans.ttf", 15)
    if err != nil {
        return fmt.Errorf("Unable to load font size 15: %v", err)
    }
    defer smallFont.Close()
    */

    joystickManager := common.NewJoystickManager()
    /*
    log.Printf("Found joysticks: %v\n", sdl.NumJoysticks())
    for i := 0; i < sdl.NumJoysticks(); i++ {
        guid := sdl.JoystickGetDeviceGUID(i)
        log.Printf("Joystick %v: %v\n", i, guid)
    }

    defer joystickManager.Close()

    // sdl.Do(sdl.StopTextInput)
    sdl.Do(sdl.StartTextInput)
    */

    // var joystickInput nes.HostInput
    /*
    var joystickInput *common.SDLJoystickButtons
    if sdl.NumJoysticks() > 0 {
        input, err := common.OpenJoystick(0)
        // input, err := MakeIControlPadInput(0)
        if err == nil {
            defer input.Close()
            joystickInput = &input
        }
    }
    */

    /*
    err = renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
    if err != nil {
        log.Printf("Could not set blend mode: %v", err)
    }
    */

    emulatorMessages := EmulatorMessageLayer{
        ReceiveMessages: make(chan string, 10),
        Index: 1,
    }

    go emulatorMessages.Run(mainQuit)

    renderManager.AddLayer(&emulatorMessages)

    /* Show black bars on the sides or top/bottom when the window changes size */
    // renderer.SetLogicalSize(int32(256), int32(240-overscanPixels * 2))

    /* create a surface from the pixels in one call, then create a texture and render it */

    renderNow := make(chan bool, 2)

    /* FIXME: kind of ugly to keep this here */
    raw_pixels := make([]byte, nes.VideoWidth*(nes.VideoHeight-nes.OverscanPixels*2) * 4)

    waiter.Add(1)
    go func(){
        buffer := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)
        bufferReady <- buffer
        defer waiter.Done()
        fpsCounter := 2.0
        fps := 0
        fpsTimer := time.NewTicker(time.Duration(fpsCounter) * time.Second)
        defer fpsTimer.Stop()

        renderTimer := time.NewTicker(time.Second / time.Duration(desiredFps))
        defer renderTimer.Stop()
        canRender := false

        render := func (){
            /*
            renderer.SetDrawColor(0, 0, 0, 0)
            renderer.Clear()

            err := renderManager.RenderAll(gfx.RenderInfo{
                Renderer: renderer,
                Font: font,
                SmallFont: smallFont,
                Window: window,
            })

            if err != nil {
                log.Printf("Warning: could not render: %v", err)
            }

            renderer.Present()
            */
        }

        _ = render

        for {
            select {
                case <-mainQuit.Done():
                    return
                case screen := <-toDraw:
                    if canRender {
                        fps += 1
                        common.RenderPixelsRGBA(screen, raw_pixels, nes.OverscanPixels)
                        // sdl.Do(render)
                    }
                    canRender = false
                    bufferReady <- screen
                case message := <-renderOverlayUpdate:
                    if message == "" {
                        renderManager.RemoveByIndex(2)
                    } else {
                        renderManager.Replace(2, &OverlayMessageLayer{
                            Message: message,
                            Index: 2,
                        })
                    }
                    // sdl.Do(render)
                case <-renderNow:
                    /* Force a rerender */
                    // sdl.Do(render)
                case <-renderTimer.C:
                    canRender = true
                case <-fpsTimer.C:
                    /* FIXME: don't print this while the menu is running */
                    log.Printf("FPS: %v", int(float64(fps) / fpsCounter))
                    fps = 0
            }
        }
    }()

    emulatorActions := make(chan common.EmulatorAction, 50)
    emulatorActionsInput := (<-chan common.EmulatorAction)(emulatorActions)
    emulatorActionsOutput := (chan<- common.EmulatorAction)(emulatorActions)

    var screenListeners common.ScreenListeners

    audioActions := make(chan AudioActions, 2)
    audioActionsInput := (<-chan AudioActions)(audioActions)
    audioActionsOutput := (chan<- AudioActions)(audioActions)

    audioChannel := make(chan []float32, 2)
    audioInput := (<-chan []float32)(audioChannel)
    audioOutput := (chan<- []float32)(audioChannel)

    go makeAudioWorker(audioInput, audioActionsInput, mainQuit)()

    emulatorKeys := common.LoadEmulatorKeys()
    input := &common.SDLKeyboardButtons{
        Keys: &emulatorKeys,
    }

    debugWindow := debug.MakeDebugWindow(mainQuit, font, smallFont)

    startNES := func(nesFile nes.NESFile, quit context.Context){
        cpu, err := common.SetupCPU(nesFile, debugCpu, debugPpu)

        debugger := debug.MakeDebugger(&cpu, debugWindow)
        defer debugger.Close()

        input.Reset()
        if replayKeys != "" {
            replay, err := makeReplayKeys(&cpu, replayKeys)
            if err != nil {
                log.Printf("Warning: could not open replay file: %v", err)
                mainCancel()
                return
            } else {
                defer replay.Close()
                cpu.Input = nes.MakeInput(replay)
            }
        } else {
            combined := common.MakeCombineButtons(input, joystickManager)
            cpu.Input = nes.MakeInput(&combined)
        }

        if recordInput {
            filename := fmt.Sprintf("%v-%v-input.txt", filepath.Base(nesFile.Path), time.Now().Unix())
            output, err := os.Create(filename)
            if err != nil {
                log.Printf("Error: Could not save input buttons to file %v: %v", filename, err)
            } else {
                log.Printf("Saving output to %v", filename)

                inputData := make(chan nes.RecordInput, 3)
                /* closed when the nes simulation is done */
                defer close(inputData)
                go func(){
                    defer output.Close()
                    for data := range inputData {
                        difference := false
                        var out strings.Builder
                        out.WriteString(fmt.Sprintf("%v: ", data.Cycle))
                        for k, v := range data.Difference {
                            difference = true
                            out.WriteString(fmt.Sprintf("%v=%v ", nes.ButtonName(k), v))
                        }
                        if difference {
                            output.WriteString(out.String() + "\n")
                        }
                    }
                }()

                cpu.Input.RecordedInput = inputData
                cpu.Input.RecordInput = true
            }
        }

        renderNes := func(info gfx.RenderInfo) error {
            // return doRenderNesPixels(nes.VideoWidth, nes.VideoHeight-nes.OverscanPixels*2, raw_pixels, pixelFormat, info.Renderer)
            return nil
        }

        layer := &DefaultRenderLayer{
            RenderFunc: renderNes,
            Index: 0,
        }

        renderManager.AddLayer(layer)
        defer renderManager.RemoveLayer(layer)

        if err != nil {
            log.Printf("Error: CPU initialization error: %v", err)
            /* The main loop below is waiting for an event so we push the quit event */
            select {
                case renderOverlayUpdate <- "Unable to load":
                default:
            }
            common.RunDummyNES(quit, emulatorActionsInput)
        } else {
            /* make sure no message appears on the screen in front of the nes output */
            select {
                case renderOverlayUpdate <- "":
                default:
            }
            log.Printf("Run NES")
            err = common.RunNES(nesFile.Path, &cpu, maxCycles, quit, toDraw, bufferReady, audioOutput, emulatorActionsInput, &screenListeners, renderOverlayUpdate, AudioSampleRate, 1, debugger)
            if err != nil {
                if err == common.MaxCyclesReached {
                } else {
                    log.Printf("Error running NES: %v", err)
                }

                mainCancel()
            }
        }
    }

    /* runs the nes emulator */
    waiter.Add(1)
    go func(){
        defer waiter.Done()

        var nesWaiter sync.WaitGroup
        nesQuit, nesCancel := context.WithCancel(mainQuit)

        go common.RunDummyNES(nesQuit, emulatorActionsInput)

        var currentFile nes.NESFile

        for {
            select {
                case <-mainQuit.Done():
                    nesCancel()
                    return
                case action := <-nesChannel:
                    doRestart := false
                    switch action.(type) {
                        case *NesActionLoad:
                            load := action.(*NesActionLoad)
                            currentFile = load.File
                            doRestart = true
                        case *NesActionRestart:
                            doRestart = true
                        case *NesActionDebugger:
                            debugWindow.Open(mainQuit)
                    }

                    if doRestart && currentFile.Path != "" {
                        nesCancel()
                        nesWaiter.Wait()
                        nesQuit, nesCancel = context.WithCancel(mainQuit)

                        nesWaiter.Add(1)
                        go func(nesFile nes.NESFile, quit context.Context){
                            defer nesWaiter.Done()
                            startNES(nesFile, quit)
                        }(currentFile, nesQuit)
                    }
            }
        }

        // nesWaiter.Wait()
    }()

    recordQuit, recordCancel := context.WithCancel(mainQuit)
    if recordOnStart {
        err := RecordMp4(recordQuit, stripExtension(filepath.Base(path)), nes.OverscanPixels, int(AudioSampleRate), &screenListeners)
        if err != nil {
            log.Printf("Error: could not record: %v", err)
        }
        defer recordCancel()
    } else {
        recordCancel()
    }

    /* FIXME: this would be good to do as a generic function.
     *   reader, writer := makeChannel(common.WindowSize, 2)
     * Where the reader gets the <-chan and the writer gets the chan<-
     */
    /* Notify the menu when the window changes size */
    /*
    windowSizeUpdates := make(chan common.WindowSize, 10)
    windowSizeUpdatesInput := (<-chan common.WindowSize)(windowSizeUpdates)
    windowSizeUpdatesOutput := (chan<- common.WindowSize)(windowSizeUpdates)
    */

    /* Actions done in the menu that should affect the program */
    programActions := make(chan common.ProgramActions, 2)
    programActionsInput := (<-chan common.ProgramActions)(programActions)
    programActionsOutput := (chan<- common.ProgramActions)(programActions)

    // theMenu := menu.MakeMenu(font, smallFont, mainQuit, renderFuncUpdate, windowSizeUpdatesInput, programActionsOutput)

    go func(){
        for {
            select {
                case <-mainQuit.Done():
                    return
                case action := <-programActionsInput:
                    switch action.(type) {
                        case *common.ProgramToggleSound:
                            audioActionsOutput <- &AudioToggle{}
                        case *common.ProgramQueryAudioState:
                            query := action.(*common.ProgramQueryAudioState)
                            audioActionsOutput <- &AudioQueryEnabled{Response: query.Response}
                        case *common.ProgramQuit:
                            mainCancel()
                        case *common.ProgramPauseEmulator:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSetPause):
                                default:
                            }
                        case *common.ProgramUnpauseEmulator:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorUnpause):
                                default:
                            }
                        case *common.ProgramLoadRom:
                            loadRom := action.(*common.ProgramLoadRom)
                            file, err := loadRom.File()

                            if err != nil {
                                log.Printf("Could not load rom '%v'", loadRom.Name)
                                break
                            }

                            nesFile, err := nes.ParseNes(file, true, loadRom.Name)
                            file.Close()
                            if err != nil {
                                log.Printf("Could not load rom '%v'", path)
                            } else {
                                log.Printf("Loaded rom '%v'", loadRom.Name)
                                nesChannel <- &NesActionLoad{File: nesFile}
                                select {
                                    case emulatorMessages.ReceiveMessages <- "Loaded rom":
                                    default:
                                }
                            }
                    }
            }
        }
    }()

    /* enable drag/drop events */
    /*
    sdl.Do(func(){
        sdl.EventState(sdl.DROPFILE, sdl.ENABLE)
    })
    */

    console := MakeConsole(6, &renderManager, mainCancel, mainQuit, emulatorActionsOutput, nesChannel, renderNow)
    _ = console

    /*
    mainWindowId, err := window.GetID()
    if err != nil {
        log.Printf("Could not get main window id: %v", err)
        return err
    }
    */

    /*
    events := make(chan sdl.Event, 20)

    handleOneEvent := func(event sdl.Event){
        switch event.GetType() {
            case sdl.QUIT: mainCancel()
            case sdl.WINDOWEVENT:
                window_event := event.(*sdl.WindowEvent)
                useWindowId := getWindowIdFromEvent(window_event)
                switch window_event.Event {
                    case sdl.WINDOWEVENT_EXPOSED:
                        if useWindowId == mainWindowId {
                            select {
                                case renderNow <- true:
                                default:
                            }
                        } else if debugWindow.IsWindow(useWindowId) {
                            debugWindow.Redraw()
                        }
                    case sdl.WINDOWEVENT_CLOSE:
                        if useWindowId == mainWindowId {
                            mainCancel()
                        } else if debugWindow.IsWindow(useWindowId) {
                            debugWindow.Close()
                        }
                    case sdl.WINDOWEVENT_RESIZED:
                        // log.Printf("Window resized")

                }

                / *
                width, height := window.GetSize()
                / * Not great but tolerate not updating the system when the window changes * /
                select {
                    case windowSizeUpdatesOutput <- common.WindowSize{X: int(width), Y: int(height)}:
                    default:
                        log.Printf("Warning: dropping a window event")
                }
                * /
            case sdl.TEXTINPUT, sdl.TEXTEDITING:
                useWindowId := getWindowIdFromEvent(event)

                if useWindowId == mainWindowId {
                    if console.IsActive() {
                        console.HandleText(event)
                    }
                } else if debugWindow.IsWindow(useWindowId) {
                    debugWindow.HandleText(event)
                }
            case sdl.DROPFILE:
                drop_event := event.(*sdl.DropEvent)
                switch drop_event.Type {
                    case sdl.DROPFILE:
                        // log.Printf("drop file '%v'\n", drop_event.File)
                        open := func() (fs.File, error){
                            return os.Open(drop_event.File)
                        }
                        programActionsOutput <- &common.ProgramLoadRom{Name: drop_event.File, File: open}
                    case sdl.DROPBEGIN:
                        log.Printf("drop begin '%v'\n", drop_event.File)
                    case sdl.DROPCOMPLETE:
                        log.Printf("drop complete '%v'\n", drop_event.File)
                    case sdl.DROPTEXT:
                        log.Printf("drop text '%v'\n", drop_event.File)
                }

            case sdl.KEYDOWN:
                keyboard_event := event.(*sdl.KeyboardEvent)
                useWindowId := getWindowIdFromEvent(event)
                if useWindowId == mainWindowId {
                    // log.Printf("key down %+v pressed %v escape %v", keyboard_event, keyboard_event.State == sdl.PRESSED, keyboard_event.Keysym.Sym == sdl.K_ESCAPE)
                    quit_pressed := keyboard_event.State == sdl.PRESSED && (keyboard_event.Keysym.Sym == sdl.K_ESCAPE || keyboard_event.Keysym.Sym == sdl.K_CAPSLOCK)

                    if quit_pressed {
                        select {
                            case doMenu <- true:
                            default:
                        }

                        // theMenu.Input <- menu.MenuToggle
                    }

                    if console.IsActive() {
                        console.HandleKey(keyboard_event, emulatorKeys)
                        return
                    }

                    / * Pass input to nes * /
                    input.HandleEvent(keyboard_event)

                    switch keyboard_event.Keysym.Sym {
                        case emulatorKeys.Console:
                            console.Toggle()
                        case emulatorKeys.Turbo:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorTurbo):
                                default:
                            }
                        case emulatorKeys.StepFrame:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorStepFrame):
                                default:
                            }
                        case emulatorKeys.SaveState:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSaveState):
                                    select {
                                        case emulatorMessages.ReceiveMessages <- "Saved state":
                                        default:
                                    }
                                default:
                            }
                        case emulatorKeys.LoadState:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorLoadState):
                                    select {
                                        case emulatorMessages.ReceiveMessages <- "Loaded state":
                                        default:
                                    }
                                default:
                            }
                        case emulatorKeys.Record:
                            if recordQuit.Err() == nil {
                                recordCancel()
                                select {
                                    case emulatorMessages.ReceiveMessages <- "Stopped recording":
                                    default:
                                }
                            } else {
                                recordCancel()

                                recordQuit, recordCancel = context.WithCancel(mainQuit)
                                err := RecordMp4(recordQuit, stripExtension(filepath.Base(path)), nes.OverscanPixels, int(AudioSampleRate), &screenListeners)
                                if err != nil {
                                    log.Printf("Could not record video: %v", err)
                                }
                                select {
                                    case emulatorMessages.ReceiveMessages <- "Started recording":
                                    default:
                                }
                            }
                        case emulatorKeys.Pause:
                            log.Printf("Pause/unpause")
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorTogglePause):
                                default:
                            }
                        case emulatorKeys.PPUDebug:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorTogglePPUDebug):
                                default:
                            }
                        case emulatorKeys.SlowDown:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSlowDown):
                                default:
                            }
                        case emulatorKeys.SpeedUp:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSpeedUp):
                                default:
                            }
                        case emulatorKeys.Normal:
                            select {
                                case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorNormal):
                                default:
                            }
                        case emulatorKeys.HardReset:
                            log.Printf("Hard reset")
                            nesChannel <- &NesActionRestart{}
                            select {
                                case emulatorMessages.ReceiveMessages <- "Hard reset":
                                default:
                            }
                    }
                } else if debugWindow.IsWindow(useWindowId) {
                    debugWindow.HandleKey(event)
                }
            case sdl.KEYUP:
                keyboard_event := event.(*sdl.KeyboardEvent)
                useWindowId := getWindowIdFromEvent(keyboard_event)
                if useWindowId == mainWindowId {
                    input.HandleEvent(keyboard_event)
                    code := keyboard_event.Keysym.Sym
                    if code == emulatorKeys.Turbo || code == emulatorKeys.Pause {
                        select {
                            case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorNormal):
                            default:
                        }
                    }
                } else if debugWindow.IsWindow(useWindowId) {
                    debugWindow.HandleKey(event)
                }
            case sdl.JOYBUTTONDOWN, sdl.JOYBUTTONUP, sdl.JOYAXISMOTION:
                action := joystickManager.HandleEvent(event)
                select {
                    case emulatorActionsOutput <- action:
                    default:
                }
        }
    }
    */

    /* Process events */
    /*
    go func(){
        for {
            select {
                case <-mainQuit.Done():
                    return
                case event := <-events:
                    handleOneEvent(event)
            }
        }
    }()
    */

    /* This function executes in a sdl.Do context */
    /*
    eventFunction := func(){
        event := sdl.WaitEventTimeout(1)
        if event != nil {
            // log.Printf("Event %+v\n", event)
            events <- event
            / *
            select {
                case events <- event:
                default:
                    log.Printf("Dropping event %+v", event)
            }
            * /
        }
    }
    */

    for mainQuit.Err() == nil {
        select {
            case <-doMenu:
                activeMenu := menu.MakeMenu(mainQuit, font)
                emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSetPause)
                activeMenu.Run(mainCancel, font, smallFont, programActionsOutput, renderNow, &renderManager, joystickManager, &emulatorKeys)
                emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorUnpause)
                select {
                    case renderNow<-true:
                    default:
                }
            default:
                // sdl.Do(eventFunction)
        }
    }

    log.Printf("Waiting to quit..")
    waiter.Wait()

    return nil
}


/*
func get_pixel_format(format uint32) string {
    switch format {
        case sdl.PIXELFORMAT_BGR888: return "BGR888"
        case sdl.PIXELFORMAT_ARGB8888: return "ARGB8888"
        case sdl.PIXELFORMAT_RGB888: return "RGB888"
        case sdl.PIXELFORMAT_RGBA8888: return "RGBA8888"
    }

    return fmt.Sprintf("%v?", format)
}
*/

type Arguments struct {
    NESPath string
    Debug bool
    DebugCpu bool
    DebugPpu bool
    MaxCycles uint64
    WindowSizeMultiple int
    CpuProfile bool
    MemoryProfile bool
    Record bool
    DesiredFps int
    RecordKeys bool
    ReplayKeys string // set to a file to replay keys from, or empty if replay is not desired
}

func parseArguments() (Arguments, error) {
    var arguments Arguments
    arguments.WindowSizeMultiple = 3
    arguments.CpuProfile = false
    arguments.MemoryProfile = false
    arguments.DesiredFps = 60
    arguments.RecordKeys = false
    arguments.ReplayKeys = ""

    for argIndex := 1; argIndex < len(os.Args); argIndex++ {
        arg := os.Args[argIndex]
        switch arg {
            case "-h", "--help":
                return arguments, fmt.Errorf(`NES emulator by Jon Rafkind
$ nes [rom.nes]
Options:
  -h, --help: this help
  -debug, --debug: enable all debug output
  -debug=cpu, --debug=cpu: enable cpu debug output
  -debug=ppu, --debug=ppu: enable ppu debug output
  -size, --size #: start the window at a multiple of the nes screen size of 320x200 (default 3)
  -record: enable video recording to an mp4 when the rom loads
  -fps #: set a desired frame rate
  -profile: Write profile.cpu and profile.memory, which are the pprof cpu and memory profiles
  -cycles, --cycles #: limit the emulator to only run for some number of cycles
  -record-input: record key presses
  -replay-input <input file>: replay key presses. A rom must also be specified
`)
            case "-debug", "--debug":
                arguments.Debug = true
            case "-debug=cpu", "--debug=cpu":
                arguments.DebugCpu = true
            case "-debug=ppu", "--debug=ppu":
                arguments.DebugPpu = true
            case "-profile":
                arguments.CpuProfile = true
                arguments.MemoryProfile = true
            case "-record-input":
                arguments.RecordKeys = true
            case "-replay-input":
                argIndex += 1
                if argIndex >= len(os.Args) {
                    return arguments, fmt.Errorf("Expected a filename for -replay-input")
                }
                arguments.ReplayKeys = os.Args[argIndex]
            case "-size", "--size":
                var err error
                argIndex += 1
                if argIndex >= len(os.Args) {
                    return arguments, fmt.Errorf("Expected an integer argument for -size")
                }
                windowSizeMultiple, err := strconv.ParseInt(os.Args[argIndex], 10, 64)
                if err != nil {
                    return arguments, fmt.Errorf("Error reading size argument: %v", err)
                }
                if windowSizeMultiple < 1 {
                    windowSizeMultiple = 1
                }
                arguments.WindowSizeMultiple = int(windowSizeMultiple)
            case "-record":
                arguments.Record = true
            case "-fps":
                argIndex += 1
                if argIndex >= len(os.Args) {
                    return arguments, fmt.Errorf("Expected an integer for argument -fps")
                }
                fps, err := strconv.ParseInt(os.Args[argIndex], 10, 64)
                if err != nil {
                    return arguments, fmt.Errorf("Error reading fps argument: %v", err)
                }
                if fps < 1 {
                    fps = 1
                }
                arguments.DesiredFps = int(fps)
            case "-cycles", "--cycles":
                var err error
                argIndex += 1
                if argIndex >= len(os.Args) {
                    return arguments, fmt.Errorf("Expected a number of cycles")
                }
                arguments.MaxCycles, err = strconv.ParseUint(os.Args[argIndex], 10, 64)
                if err != nil {
                    return arguments, fmt.Errorf("Error parsing cycles: %v", err)
                }
            default:
                arguments.NESPath = arg
        }
    }

    if arguments.ReplayKeys != "" && arguments.NESPath == "" {
        return arguments, fmt.Errorf("A rom must be specified when replaying keys")
    }

    if arguments.ReplayKeys != "" && arguments.RecordKeys {
        return arguments, fmt.Errorf("Cannot record and replay keys at the same time")
    }

    return arguments, nil
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

    arguments, err := parseArguments()
    if err != nil {
        fmt.Printf("%v\n", err)
        return
    }

    ebiten.SetWindowTitle("NES Emulator")
    ebiten.SetWindowSize(nes.VideoWidth * arguments.WindowSizeMultiple, (nes.VideoHeight - nes.OverscanPixels * 2) * arguments.WindowSizeMultiple)
    ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

    /*
    go func(){
        var stats rdebug.GCStats
        for {
            time.Sleep(2 * time.Second)
            rdebug.ReadGCStats(&stats)
            log.Printf("GC stats last=%v gc=%v pause=%v", stats.LastGC, stats.NumGC, stats.PauseTotal)
        }
    }()
    */

    if arguments.CpuProfile {
        profile, err := os.Create("profile.cpu")
        if err != nil {
            log.Fatal(err)
        }
        defer profile.Close()
        pprof.StartCPUProfile(profile)
        defer pprof.StopCPUProfile()
    }

    if nes.IsNESFile(arguments.NESPath) {
        err := RunNES(arguments.NESPath, arguments.Debug || arguments.DebugCpu, arguments.Debug || arguments.DebugPpu, arguments.MaxCycles, arguments.WindowSizeMultiple, arguments.Record, arguments.DesiredFps, arguments.RecordKeys, arguments.ReplayKeys)
        if err != nil {
            log.Printf("Error: %v\n", err)
        }
    } else if nes.IsNSFFile(arguments.NESPath) {
        err := RunNSF(arguments.NESPath)
        if err != nil {
            log.Printf("Error: %v\n", err)
        }
    } else {
        /* Open up the loading menu immediately */
        err := RunNES(arguments.NESPath, arguments.Debug || arguments.DebugCpu, arguments.Debug || arguments.DebugPpu, arguments.MaxCycles, arguments.WindowSizeMultiple, arguments.Record, arguments.DesiredFps, arguments.RecordKeys, arguments.ReplayKeys)
        if err != nil {
            log.Printf("Error: %v\n", err)
        }
    }
    log.Printf("Bye")

    if arguments.MemoryProfile {
        file, err := os.Create("profile.memory")
        if err != nil {
            log.Fatal(err)
        }
        pprof.WriteHeapProfile(file)
        file.Close()
        return
    }
}

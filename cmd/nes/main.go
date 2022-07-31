package main

/* golang sdl https://pkg.go.dev/github.com/veandco/go-sdl2@v0.4.8/sdl
 */

/*
#include <stdlib.h>
*/
import "C"
import (
    "fmt"
    "log"
    "strconv"
    "os"
    "os/signal"
    "path/filepath"
    "math/rand"

    nes "github.com/kazzmir/nes/lib"
    "github.com/kazzmir/nes/util"

    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/mix"
    "github.com/veandco/go-sdl2/ttf"

    "encoding/binary"
    "bytes"
    "time"
    "sync"
    "context"
    "runtime/pprof"

    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/kazzmir/nes/cmd/nes/menu"

    // rdebug "runtime/debug"
)

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

func makeAudioWorker(audioDevice sdl.AudioDeviceID, audio <-chan []float32, audioActions <-chan AudioActions, mainQuit context.Context) func() {
    if audioDevice != 0 {
        /* runNES will generate arrays of samples that we enqueue into the SDL audio system */
        return func(){
            var buffer bytes.Buffer
            enabled := true
            for {
                select {
                    case <-mainQuit.Done():
                        return
                    case action := <-audioActions:
                        _, ok := action.(*AudioToggle)
                        if ok {
                            enabled = !enabled
                        }

                        query, ok := action.(*AudioQueryEnabled)
                        if ok {
                            query.Response <- enabled
                        }
                    case samples := <-audio:
                        if !enabled {
                            break
                        }
                        // log.Printf("Prepare audio to queue")
                        // log.Printf("Enqueue data %v", samples)
                        buffer.Reset()
                        /* convert []float32 into []byte */
                        for _, sample := range samples {
                            binary.Write(&buffer, binary.LittleEndian, sample)
                        }
                        // log.Printf("Enqueue audio")
                        var err error
                        sdl.Do(func(){
                            err = sdl.QueueAudio(audioDevice, buffer.Bytes())
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
                        query, ok := action.(*AudioQueryEnabled)
                        if ok {
                            query.Response <- false
                        }
                    case <-audio:
                }
            }
        }
    }
}

/* must be called in a sdl.Do */
func doRenderNesPixels(width int, height int, raw_pixels []byte, pixelFormat common.PixelFormat, renderer *sdl.Renderer) error {

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


    renderer.SetLogicalSize(int32(width), int32(height))
    err = renderer.Copy(texture, nil, nil)
    if err != nil {
        log.Printf("Warning: could not copy texture to renderer: %v\n", err)
    }

    renderer.SetLogicalSize(0, 0)

    return nil
}


type NesAction interface {
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
    RenderFunc func(common.RenderInfo) error
    Index int
}

func (layer *DefaultRenderLayer) Render(info common.RenderInfo) error {
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

func (layer *EmulatorMessageLayer) Render(renderInfo common.RenderInfo) error {
    windowWidth, windowHeight := renderInfo.Window.GetSize()

    font := renderInfo.SmallFont

    layer.Lock.Lock()
    messages := common.CopyArray(layer.emulatorMessages)
    layer.Lock.Unlock()

    y := int(windowHeight) - font.Height() - 1
    now := time.Now()
    for i := len(messages)-1; i >= 0; i-- {
        message := messages[i]
        if message.DeathTime.After(now){
            x := int(windowWidth) - 100
            remaining := message.DeathTime.Sub(now)
            alpha := 255
            if remaining < time.Millisecond * 500 {
                alpha = int(255 * float64(remaining) / (float64(time.Millisecond) * 500))
                if alpha > 255 {
                    alpha = 255
                }
                /* strangely if alpha=0 it renders without transparency so the pixels are fully white */
                if alpha < 1 {
                    alpha = 1
                }
            }
            white := sdl.Color{R: 255, G: 255, B: 255, A: uint8(alpha)}
            // log.Printf("Write message '%v' at %v, %v remaining=%v color=%v", message, x, y, remaining, white)
            common.WriteFont(font, renderInfo.Renderer, x, y, message.Message, white)
            y -= font.Height() + 2
        }
    }

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

func (layer *OverlayMessageLayer) Render(info common.RenderInfo) error {
    width, height := info.Window.GetSize()

    font := info.Font
    renderer := info.Renderer

    black := sdl.Color{R: 0, G: 0, B: 0, A: 200}
    white := sdl.Color{R: 255, G: 255, B: 255, A: 200}
    messageLength := common.TextWidth(font, layer.Message)
    x := int(width)/2 - messageLength / 2
    y := int(height)/2
    renderer.SetDrawColor(black.R, black.G, black.B, black.A)
    renderer.FillRect(&sdl.Rect{X: int32(x - 10), Y: int32(y - 10), W: int32(messageLength + 10 + 5), H: int32(font.Height() + 10 + 5)})

    common.WriteFont(font, renderer, x, y, layer.Message, white)
    return nil
}

func RunNES(path string, debug bool, maxCycles uint64, windowSizeMultiple int, recordOnStart bool, desiredFps int) error {
    randomSeed := time.Now().UnixNano()

    rand.Seed(randomSeed)

    nesChannel := make(chan NesAction, 10)
    doMenu := make(chan bool, 5)
    renderOverlayUpdate := make(chan string, 5)

    var renderManager common.RenderManager

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
    if !util.HasGlxinfo() {
        sdl.Do(func(){
            sdl.SetHint(sdl.HINT_RENDER_DRIVER, "software")
        })
    }

    log.Printf("Initializing SDL")

    var err error
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

    log.Printf("Create window")
    /* to resize the window */
    var window *sdl.Window
    var renderer *sdl.Renderer

    /* 7/5/2021: its apparently very important that the window and renderer be created
     * in the sdl thread via sdl.Do. If the renderer calls are in sdl.Do, then so must
     * also be the creation of the window and the renderer. Initially I did not have
     * the creation of the window and renderer in sdl.Do, and thus in opengl mode
     * the window would not be rendered.
     */
    sdl.Do(func(){
        window, renderer, err = sdl.CreateWindowAndRenderer(
            int32(nes.VideoWidth * windowSizeMultiple),
            int32((nes.VideoHeight - nes.OverscanPixels * 2) * windowSizeMultiple),
            sdl.WINDOW_SHOWN | sdl.WINDOW_RESIZABLE)

        if window != nil {
            window.SetTitle("Nes Emulator")
        }
    })

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

    if err != nil {
        return err
    }

    defer sdl.Do(func(){
        window.Destroy()
    })
    defer sdl.Do(func(){
        renderer.Destroy()
    })

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

    err = mix.Init(mix.INIT_OGG)
    if err != nil {
        log.Printf("Could not initialize SDL mixer: %v", err)
    } else {
        err = mix.OpenAudio(int(AudioSampleRate), sdl.AUDIO_F32LSB, 2, 4096)
        if err != nil {
            log.Printf("Could not open mixer audio: %v", err)
        }
    }

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
   
    audioDevice, err := setupAudio(AudioSampleRate)
    if err != nil {
        log.Printf("Warning: could not set up audio: %v", err)
        audioDevice = 0
    } else {
        defer sdl.CloseAudioDevice(audioDevice)
        log.Printf("Opened SDL audio device %v", audioDevice)
        sdl.PauseAudioDevice(audioDevice, false)
    }

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
                    } else {
                        log.Printf("Hard kill")
                        os.Exit(1)
                    }
            }
        }
    }()

    toDraw := make(chan nes.VirtualScreen, 1)
    bufferReady := make(chan nes.VirtualScreen, 1)

    pixelFormat := common.FindPixelFormat()

    log.Printf("Using pixel format %v\n", sdl.GetPixelFormatName(uint(pixelFormat)))

    err = ttf.Init()
    if err != nil {
        return err
    }

    defer ttf.Quit()

    font, err := ttf.OpenFont(filepath.Join(filepath.Dir(os.Args[0]), "data/DejaVuSans.ttf"), 20)
    if err != nil {
        return err
    }
    defer font.Close()

    smallFont, err := ttf.OpenFont(filepath.Join(filepath.Dir(os.Args[0]), "data/DejaVuSans.ttf"), 15)
    if err != nil {
        return err
    }
    defer smallFont.Close()

    log.Printf("Found joysticks: %v\n", sdl.NumJoysticks())
    for i := 0; i < sdl.NumJoysticks(); i++ {
        guid := sdl.JoystickGetDeviceGUID(i)
        log.Printf("Joystick %v: %v\n", i, guid)
    }

    joystickManager := common.NewJoystickManager()
    defer joystickManager.Close()

    sdl.Do(sdl.StopTextInput)

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
            renderer.SetDrawColor(0, 0, 0, 0)
            renderer.Clear()

            err := renderManager.RenderAll(common.RenderInfo{
                Renderer: renderer,
                Font: font,
                SmallFont: smallFont,
                Window: window,
            })

            if err != nil {
                log.Printf("Warning: could not render: %v", err)
            }

            renderer.Present()
        }

        for {
            select {
                case <-mainQuit.Done():
                    return
                case screen := <-toDraw:
                    if canRender {
                        fps += 1
                        common.RenderPixelsRGBA(screen, raw_pixels, nes.OverscanPixels)
                        sdl.Do(render)
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
                    sdl.Do(render)
                case <-renderNow:
                    /* Force a rerender */
                    sdl.Do(render)
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

    go makeAudioWorker(audioDevice, audioInput, audioActionsInput, mainQuit)()

    emulatorKeys := common.DefaultEmulatorKeys()
    input := &common.SDLKeyboardButtons{
        Keys: emulatorKeys,
    }

    startNES := func(nesFile nes.NESFile, quit context.Context){
        cpu, err := common.SetupCPU(nesFile, debug)

        input.Reset()
        combined := common.MakeCombineButtons(input, joystickManager)
        cpu.Input = nes.MakeInput(&combined)

        renderNes := func(info common.RenderInfo) error {
            return doRenderNesPixels(nes.VideoWidth, nes.VideoHeight-nes.OverscanPixels*2, raw_pixels, pixelFormat, info.Renderer)
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
            err = common.RunNES(nesFile.Path, &cpu, maxCycles, quit, toDraw, bufferReady, audioOutput, emulatorActionsInput, &screenListeners, renderOverlayUpdate, AudioSampleRate, 1)
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
                    load, ok := action.(*NesActionLoad)
                    if ok {
                        currentFile = load.File
                        doRestart = true
                    }

                    _, ok = action.(*NesActionRestart)
                    if ok {
                        doRestart = true
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

        nesWaiter.Wait()
    }()

    recordQuit, recordCancel := context.WithCancel(mainQuit)
    if recordOnStart {
        err := RecordMp4(recordQuit, stripExtension(filepath.Base(path)), nes.OverscanPixels, int(AudioSampleRate), &screenListeners)
        if err != nil {
            log.Printf("Error: could not record: %v", err)
        }
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
                    _, ok := action.(*common.ProgramToggleSound)
                    if ok {
                        audioActionsOutput <- &AudioToggle{}
                    }

                    query, ok := action.(*common.ProgramQueryAudioState)
                    if ok {
                        audioActionsOutput <- &AudioQueryEnabled{Response: query.Response}
                    }

                    _, ok = action.(*common.ProgramQuit)
                    if ok {
                        mainCancel()
                    }

                    _, ok = action.(*common.ProgramPauseEmulator)
                    if ok {
                        select {
                            case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSetPause):
                            default:
                        }
                    }

                    _, ok = action.(*common.ProgramUnpauseEmulator)
                    if ok {
                        select {
                            case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorUnpause):
                            default:
                        }
                    }

                    loadRom, ok := action.(*common.ProgramLoadRom)
                    if ok {
                        nesFile, err := nes.ParseNesFile(loadRom.Path, true)
                        if err != nil {
                            log.Printf("Could not load rom '%v'", path)
                        } else {
                            log.Printf("Loaded rom '%v'", loadRom.Path)
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
    sdl.Do(func(){
        sdl.EventState(sdl.DROPFILE, sdl.ENABLE)
    })

    console := MakeConsole(6, &renderManager, mainCancel, mainQuit, emulatorActionsOutput, nesChannel, renderNow)

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

                    /*
                    width, height := window.GetSize()
                    / * Not great but tolerate not updating the system when the window changes * /
                    select {
                        case windowSizeUpdatesOutput <- common.WindowSize{X: int(width), Y: int(height)}:
                        default:
                            log.Printf("Warning: dropping a window event")
                    }
                    */
                case sdl.TEXTINPUT, sdl.TEXTEDITING:
                    console.HandleText(event)
                case sdl.DROPFILE:
                    drop_event := event.(*sdl.DropEvent)
                    switch drop_event.Type {
                        case sdl.DROPFILE:
                            // log.Printf("drop file '%v'\n", drop_event.File)
                            programActionsOutput <- &common.ProgramLoadRom{Path: drop_event.File}
                        case sdl.DROPBEGIN:
                            log.Printf("drop begin '%v'\n", drop_event.File)
                        case sdl.DROPCOMPLETE:
                            log.Printf("drop complete '%v'\n", drop_event.File)
                        case sdl.DROPTEXT:
                            log.Printf("drop text '%v'\n", drop_event.File)
                    }

                case sdl.KEYDOWN:
                    keyboard_event := event.(*sdl.KeyboardEvent)
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
                case sdl.KEYUP:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    input.HandleEvent(keyboard_event)
                    code := keyboard_event.Keysym.Sym
                    if code == emulatorKeys.Turbo || code == emulatorKeys.Pause {
                        select {
                            case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorNormal):
                            default:
                        }
                    }
                case sdl.JOYBUTTONDOWN, sdl.JOYBUTTONUP, sdl.JOYAXISMOTION:
                    action := joystickManager.HandleEvent(event)
                    select {
                        case emulatorActionsOutput <- action:
                        default:
                    }
            }
        }
    }

    for mainQuit.Err() == nil {
        select {
            case <-doMenu:
                activeMenu := menu.MakeMenu(mainQuit, font)
                emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSetPause)
                activeMenu.Run(window, mainCancel, font, smallFont, programActionsOutput, renderNow, &renderManager, joystickManager, &emulatorKeys)
                emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorUnpause)
                select {
                    case renderNow<-true:
                    default:
                }
            default:
                sdl.Do(eventFunction)
        }
    }

    log.Printf("Waiting to quit..")
    waiter.Wait()

    return nil
}


func get_pixel_format(format uint32) string {
    switch format {
        case sdl.PIXELFORMAT_BGR888: return "BGR888"
        case sdl.PIXELFORMAT_ARGB8888: return "ARGB8888"
        case sdl.PIXELFORMAT_RGB888: return "RGB888"
        case sdl.PIXELFORMAT_RGBA8888: return "RGBA8888"
    }

    return fmt.Sprintf("%v?", format)
}

type Arguments struct {
    NESPath string
    Debug bool
    MaxCycles uint64
    WindowSizeMultiple int
    CpuProfile bool
    MemoryProfile bool
    Record bool
    DesiredFps int
}

func parseArguments() (Arguments, error) {
    var arguments Arguments
    arguments.WindowSizeMultiple = 3
    arguments.CpuProfile = true
    arguments.MemoryProfile = true
    arguments.DesiredFps = 60

    for argIndex := 1; argIndex < len(os.Args); argIndex++ {
        arg := os.Args[argIndex]
        switch arg {
            case "-h", "--help":
                return arguments, fmt.Errorf(`NES emulator by Jon Rafkind
$ nes [rom.nes]
Options:
  -h, --help: this help
  -debug, --debug: enable debug output
  -size, --size #: start the window at a multiple of the nes screen size of 320x200 (default 3)
  -record: enable recording to an mp4 when the rom loads
  -fps #: set a desired frame rate
  -cycles, --cycles #: limit the emulator to only run for some number of cycles
`)
            case "-debug", "--debug":
                arguments.Debug = true
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

    return arguments, nil
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

    arguments, err := parseArguments()
    if err != nil {
        fmt.Printf("%v", err)
        return
    }

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
        sdl.Main(func (){
            err := RunNES(arguments.NESPath, arguments.Debug, arguments.MaxCycles, arguments.WindowSizeMultiple, arguments.Record, arguments.DesiredFps)
            if err != nil {
                log.Printf("Error: %v\n", err)
            }
        })
    } else if nes.IsNSFFile(arguments.NESPath) {
        sdl.Main(func (){
            err := RunNSF(arguments.NESPath)
            if err != nil {
                log.Printf("Error: %v\n", err)
            }
        })
    } else {
        /* Open up the loading menu immediately */
        sdl.Main(func (){
            err := RunNES(arguments.NESPath, arguments.Debug, arguments.MaxCycles, arguments.WindowSizeMultiple, arguments.Record, arguments.DesiredFps)
            if err != nil {
                log.Printf("Error: %v\n", err)
            }
        })
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

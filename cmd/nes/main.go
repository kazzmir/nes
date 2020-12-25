package main

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

    device, err := sdl.OpenAudioDevice("", false, &audioSpec, &obtainedSpec, sdl.AUDIO_ALLOW_FORMAT_CHANGE)
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
                        err := sdl.QueueAudio(audioDevice, buffer.Bytes())
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

func doRender(width int, height int, raw_pixels []byte, pixelFormat common.PixelFormat, renderer *sdl.Renderer, renderFunc common.RenderFunction) error {
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

    renderer.SetDrawColor(0, 0, 0, 0)
    renderer.Clear()

    renderer.SetLogicalSize(int32(width), int32(height))
    renderer.Copy(texture, nil, nil)

    renderer.SetLogicalSize(0, 0)
    err = renderFunc(renderer)
    if err != nil {
        log.Printf("Warning: render error: %v", err)
    }

    renderer.Present()

    return nil
}

type NesAction interface {
}

type NesActionLoad struct {
    File nes.NESFile
}

type NesActionRestart struct {
}

func RunNES(path string, debug bool, maxCycles uint64, windowSizeMultiple int, recordOnStart bool) error {
    nesFile, err := nes.ParseNesFile(path, true)
    if err != nil {
        return err
    }

    randomSeed := time.Now().UnixNano()

    rand.Seed(randomSeed)

    // force a software renderer
    if !util.HasGlxinfo() {
        sdl.SetHint(sdl.HINT_RENDER_DRIVER, "software")
    }

    err = sdl.Init(sdl.INIT_EVERYTHING)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    sdl.DisableScreenSaver()
    defer sdl.EnableScreenSaver()

    /* to resize the window */
    window, err := sdl.CreateWindow("nes", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, int32(nes.VideoWidth * windowSizeMultiple), int32((nes.VideoHeight - nes.OverscanPixels * 2) * windowSizeMultiple), sdl.WINDOW_SHOWN | sdl.WINDOW_RESIZABLE)
    if err != nil {
        return err
    }
    defer window.Destroy()

    softwareRenderer := true
    _ = softwareRenderer
    // renderer, err := sdl.CreateSoftwareRenderer(surface)
    // renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_SOFTWARE)

    /* Create an accelerated renderer */
    renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)

    if err != nil {
        return err
    }
    defer renderer.Destroy()

    const AudioSampleRate float32 = 44100

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

    nesChannel := make(chan NesAction, 10)
    nesChannel <- &NesActionLoad{File: nesFile}

    mainQuit, mainCancel := context.WithCancel(context.Background())
    defer mainCancel()

    signalChannel := make(chan os.Signal, 10)
    signal.Notify(signalChannel, os.Interrupt)

    go func(){
        for i := 0; i < 2; i++ {
            select {
                case <-mainQuit.Done():
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

    desiredFps := 60.0
    pixelFormat := common.FindPixelFormat()

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

    var joystickInput nes.HostInput
    if sdl.NumJoysticks() > 0 {
        // input, err := OpenJoystick(0)
        input, err := MakeIControlPadInput(0)
        if err == nil {
            defer input.Close()
            joystickInput = &input
        }
    }

    /*
    err = renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
    if err != nil {
        log.Printf("Could not set blend mode: %v", err)
    }
    */

    /* Show black bars on the sides or top/bottom when the window changes size */
    // renderer.SetLogicalSize(int32(256), int32(240-overscanPixels * 2))

    /* create a surface from the pixels in one call, then create a texture and render it */

    renderFuncUpdate := make(chan common.RenderFunction, 5)
    renderNow := make(chan bool, 2)

    waiter.Add(1)
    go func(){
        buffer := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)
        bufferReady <- buffer
        defer waiter.Done()
        raw_pixels := make([]byte, nes.VideoWidth*(nes.VideoHeight-nes.OverscanPixels*2) * 4)
        fpsCounter := 2.0
        fps := 0
        fpsTimer := time.NewTicker(time.Duration(fpsCounter) * time.Second)
        defer fpsTimer.Stop()

        renderTimer := time.NewTicker(time.Second / time.Duration(desiredFps))
        defer renderTimer.Stop()
        canRender := false

        renderFunc := func(renderer *sdl.Renderer) error {
            return nil
        }

        render := func (){
            err := doRender(nes.VideoWidth, nes.VideoHeight-nes.OverscanPixels*2, raw_pixels, pixelFormat, renderer, renderFunc)
            if err != nil {
                log.Printf("Could not render: %v\n", err)
            }
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
                case newFunction := <-renderFuncUpdate:
                    if newFunction == nil {
                        newFunction = func(renderer *sdl.Renderer) error {
                            return nil
                        }
                    }
                    renderFunc = newFunction
                    sdl.Do(render)
                case <-renderNow:
                    /* Force a rerender */
                    sdl.Do(render)
                case <-renderTimer.C:
                    canRender = true
                case <-fpsTimer.C:
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

    startNES := func(nesFile nes.NESFile, quit context.Context){
        cpu, err := common.SetupCPU(nesFile, debug)

        var input nes.HostInput = &SDLButtons{}
        if joystickInput != nil {
            combined := MakeCombineButtons(input, joystickInput)
            input = &combined
        }
        cpu.Input = nes.MakeInput(input)

        var quitEvent sdl.QuitEvent
        quitEvent.Type = sdl.QUIT
        /* FIXME: does quitEvent.Timestamp need to be set? */

        if err != nil {
            log.Printf("Error: CPU initialization error: %v", err)
            /* The main loop below is waiting for an event so we push the quit event */
            sdl.PushEvent(&quitEvent)
        } else {
            log.Printf("Run NES")
            err = common.RunNES(&cpu, maxCycles, quit, toDraw, bufferReady, audioOutput, emulatorActionsInput, &screenListeners, AudioSampleRate, 1)
            if err != nil {
                if err == common.MaxCyclesReached {
                } else {
                    log.Printf("Error running NES: %v", err)
                }

                sdl.PushEvent(&quitEvent)
            }
        }
    }

    /* runs the nes emulator */
    waiter.Add(1)
    go func(){
        defer waiter.Done()

        var nesWaiter sync.WaitGroup
        nesQuit, nesCancel := context.WithCancel(mainQuit)

        var currentFile nes.NESFile

        for {
            select {
                case <-mainQuit.Done():
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

                    if doRestart {
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

    var turboKey sdl.Scancode = sdl.SCANCODE_GRAVE
    var pauseKey sdl.Scancode = sdl.SCANCODE_SPACE
    var hardResetKey sdl.Scancode = sdl.SCANCODE_R
    var ppuDebugKey sdl.Scancode = sdl.SCANCODE_P
    var slowDownKey sdl.Scancode = sdl.SCANCODE_MINUS
    var speedUpKey sdl.Scancode = sdl.SCANCODE_EQUALS
    var normalKey sdl.Scancode = sdl.SCANCODE_0
    var stepFrameKey sdl.Scancode = sdl.SCANCODE_O
    var recordKey sdl.Scancode = sdl.SCANCODE_M

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
                            case emulatorActionsOutput <- common.EmulatorSetPause:
                            default:
                        }
                    }

                    _, ok = action.(*common.ProgramUnpauseEmulator)
                    if ok {
                        select {
                            case emulatorActionsOutput <- common.EmulatorUnpause:
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
                        }
                    }
            }
        }
    }()

    doMenu := make(chan bool, 5)

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

                    /*
                    if theMenu.IsActive() {
                        switch keyboard_event.Keysym.Scancode {
                            case sdl.SCANCODE_LEFT, sdl.SCANCODE_H:
                                theMenu.Input <- menu.MenuPrevious
                            case sdl.SCANCODE_RIGHT, sdl.SCANCODE_L:
                                theMenu.Input <- menu.MenuNext
                            case sdl.SCANCODE_UP, sdl.SCANCODE_K:
                                theMenu.Input <- menu.MenuUp
                            case sdl.SCANCODE_DOWN, sdl.SCANCODE_J:
                                theMenu.Input <- menu.MenuDown
                            case sdl.SCANCODE_RETURN:
                                theMenu.Input <- menu.MenuSelect
                        }
                    } else {
                        */
                        switch keyboard_event.Keysym.Scancode {
                            case turboKey:
                                select {
                                    case emulatorActionsOutput <- common.EmulatorTurbo:
                                    default:
                                }
                            case stepFrameKey:
                                select {
                                    case emulatorActionsOutput <- common.EmulatorStepFrame:
                                    default:
                                }
                            case recordKey:
                                if recordQuit.Err() == nil {
                                    recordCancel()
                                } else {
                                    recordQuit, recordCancel = context.WithCancel(mainQuit)
                                    err := RecordMp4(recordQuit, stripExtension(filepath.Base(path)), nes.OverscanPixels, int(AudioSampleRate), &screenListeners)
                                    if err != nil {
                                        log.Printf("Could not record video: %v", err)
                                    }
                                }
                            case pauseKey:
                                log.Printf("Pause/unpause")
                                select {
                                    case emulatorActionsOutput <- common.EmulatorTogglePause:
                                    default:
                                }
                            case ppuDebugKey:
                                select {
                                    case emulatorActionsOutput <- common.EmulatorTogglePPUDebug:
                                    default:
                                }
                            case slowDownKey:
                                select {
                                    case emulatorActionsOutput <- common.EmulatorSlowDown:
                                    default:
                                }
                            case speedUpKey:
                                select {
                                    case emulatorActionsOutput <- common.EmulatorSpeedUp:
                                    default:
                                }
                            case normalKey:
                                select {
                                    case emulatorActionsOutput <- common.EmulatorNormal:
                                    default:
                                }
                            case hardResetKey:
                                log.Printf("Hard reset")
                                nesChannel <- &NesActionRestart{}
                        }
                    // }
                case sdl.KEYUP:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    scancode := keyboard_event.Keysym.Scancode
                    if scancode == turboKey || scancode == pauseKey {
                        select {
                            case emulatorActionsOutput <- common.EmulatorNormal:
                            default:
                        }
                    }
            }
        }
    }

    for mainQuit.Err() == nil {
        select {
            case <-doMenu:
                activeMenu := menu.MakeMenu(mainQuit, font)
                emulatorActionsOutput <- common.EmulatorSetPause
                activeMenu.Run(window, mainCancel, font, smallFont, programActionsOutput, renderNow, renderFuncUpdate)
                emulatorActionsOutput <- common.EmulatorUnpause
                renderFuncUpdate <- nil
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
}

func parseArguments() (Arguments, error) {
    var arguments Arguments
    arguments.WindowSizeMultiple = 3
    arguments.CpuProfile = true
    arguments.MemoryProfile = true

    for argIndex := 1; argIndex < len(os.Args); argIndex++ {
        arg := os.Args[argIndex]
        switch arg {
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
                arguments.WindowSizeMultiple = int(windowSizeMultiple)
            case "-record":
                arguments.Record = true
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

    if arguments.NESPath != "" {
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
                err := RunNES(arguments.NESPath, arguments.Debug, arguments.MaxCycles, arguments.WindowSizeMultiple, arguments.Record)
                if err != nil {
                    log.Printf("Error: %v\n", err)
                }
            })
        } else if nes.IsNSFFile(arguments.NESPath) {
            err := RunNSF(arguments.NESPath)
            if err != nil {
                log.Printf("Error: %v\n", err)
            }
        } else {
            fmt.Printf("%v is neither a .nes nor .nsf file\n", arguments.NESPath)
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
    } else {
        fmt.Printf("Give a .nes argument\n")
    }
}

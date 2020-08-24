package main

/*
#include <stdlib.h>
*/
import "C"
import (
    "fmt"
    "log"
    "strconv"
    "errors"
    "os"
    "os/exec"
    "path/filepath"

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
    "syscall"

    // rdebug "runtime/debug"
)

func setupCPU(nesFile nes.NESFile, debug bool) (nes.CPUState, error) {
    cpu := nes.StartupState()

    cpu.PPU.SetHorizontalMirror(nesFile.HorizontalMirror)
    cpu.PPU.SetVerticalMirror(nesFile.VerticalMirror)

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom, nesFile.CharacterRom)
    if err != nil {
        return cpu, err
    }
    cpu.SetMapper(mapper)

    maxCharacterRomLength := len(nesFile.CharacterRom)
    if maxCharacterRomLength > 0x2000 {
        maxCharacterRomLength = 0x2000
    }
    cpu.PPU.CopyCharacterRom(0x0000, nesFile.CharacterRom[:maxCharacterRomLength])

    cpu.Input = nes.MakeInput(&SDLButtons{})

    if debug {
        cpu.Debug = 1
        cpu.PPU.Debug = 1
    }

    cpu.Reset()

    return cpu, nil
}

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

type SDLButtons struct {
}

func (buttons *SDLButtons) Get() nes.ButtonMapping {
    mapping := make(nes.ButtonMapping)

    keyboard := sdl.GetKeyboardState()
    mapping[nes.ButtonIndexA] = keyboard[sdl.SCANCODE_A] == 1
    mapping[nes.ButtonIndexB] = keyboard[sdl.SCANCODE_S] == 1
    mapping[nes.ButtonIndexSelect] = keyboard[sdl.SCANCODE_Q] == 1
    mapping[nes.ButtonIndexStart] = keyboard[sdl.SCANCODE_RETURN] == 1
    mapping[nes.ButtonIndexUp] = keyboard[sdl.SCANCODE_UP] == 1
    mapping[nes.ButtonIndexDown] = keyboard[sdl.SCANCODE_DOWN] == 1
    mapping[nes.ButtonIndexLeft] = keyboard[sdl.SCANCODE_LEFT] == 1
    mapping[nes.ButtonIndexRight] = keyboard[sdl.SCANCODE_RIGHT] == 1

    return mapping
}

type EmulatorAction int
const (
    EmulatorNormal = iota
    EmulatorTurbo
    EmulatorSlowDown
    EmulatorSpeedUp
    EmulatorTogglePause
    EmulatorTogglePPUDebug
    EmulatorStepFrame
)

func stripExtension(path string) string {
    extension := filepath.Ext(path)
    if len(extension) > 0 {
        return path[0:len(path) - len(extension)]
    }

    return path
}

/* Returns the absolute path to ffmpeg, or an error if not found
 */
func findFfmpegBinary() (string, error) {
    return exec.LookPath("ffmpeg")
}

func waitForProcess(process *os.Process, timeout int){
    done := time.Now().Add(time.Second * time.Duration(timeout))
    dead := false
    for time.Now().Before(done) {
        err := os.Signal(syscall.Signal(0)) // on linux sending signal 0 will have no impact, but will fail
                            // if the process doesn't exist (or we don't own it)
        if err == nil {
            time.Sleep(time.Millisecond * 100)
        } else {
            dead = true
            break
        }
    }
    if !dead {
        /* Didn't die on its own, so we forcifully kill it */
        log.Printf("Killing pid %v", process.Pid)
        process.Kill()
    }
    process.Wait()
}

func niceSize(path string) string {
    info, err := os.Stat(path)
    if err != nil {
        return ""
    }

    size := float64(info.Size())
    suffixes := []string{"b", "kb", "mb", "gb"}
    suffix := 0

    for size > 1024 && suffix < len(suffixes) - 1 {
        size /= 1024
        suffix += 1
    }

    return fmt.Sprintf("%.2f%v", size, suffixes[suffix])
}

func RecordMp4(stop context.Context, romName string, overscanPixels int, sampleRate int, screenListeners *ScreenListeners) error {
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

type ScreenListeners struct {
    VideoListeners []chan nes.VirtualScreen
    AudioListeners []chan []float32
    Lock sync.Mutex
}

func (listeners *ScreenListeners) ObserveVideo(screen nes.VirtualScreen){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    if len(listeners.VideoListeners) == 0 {
        return
    }

    buffer := screen.Copy()

    for _, listener := range listeners.VideoListeners {
        select {
            case listener<- buffer:
            default:
        }
    }
}

func (listeners *ScreenListeners) AddVideoListener(listener chan nes.VirtualScreen){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    listeners.VideoListeners = append(listeners.VideoListeners, listener)
}

func (listeners *ScreenListeners) RemoveVideoListener(remove chan nes.VirtualScreen){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    var out []chan nes.VirtualScreen
    for _, listener := range listeners.VideoListeners {
        if listener != remove {
            out = append(out, listener)
        }
    }

    listeners.VideoListeners = out
}

func (listeners *ScreenListeners) ObserveAudio(pcm []float32){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    if len(listeners.AudioListeners) == 0 {
        return
    }

    for _, listener := range listeners.AudioListeners {
        select {
            case listener<- pcm:
            default:
                log.Printf("Cannot observe audio")
        }
    }
}

func (listeners *ScreenListeners) AddAudioListener(listener chan []float32){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    listeners.AudioListeners = append(listeners.AudioListeners, listener)
}

func (listeners *ScreenListeners) RemoveAudioListener(remove chan []float32){
    listeners.Lock.Lock()
    defer listeners.Lock.Unlock()

    var out []chan []float32
    for _, listener := range listeners.AudioListeners {
        if listener != remove {
            out = append(out, listener)
        }
    }

    listeners.AudioListeners = out
}

type NSFRenderState struct {
    SongName string
    PlayTime uint64
    Track int
    MaxTrack int
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

    sourceRect := sdl.Rect{X: 0, Y: 0, W: int32(surfaceBounds.Max.X), H: int32(surfaceBounds.Max.Y)}
    destRect := sourceRect
    destRect.X = int32(x)
    destRect.Y = int32(y)

    renderer.Copy(texture, &sourceRect, &destRect)

    return nil
}

func RunNSF(path string) error {
    nsfFile, err := nes.LoadNSF(path)
    if err != nil {
        return err
    }

    err = sdl.Init(sdl.INIT_EVERYTHING)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    /* to resize the window */
    // | sdl.WINDOW_RESIZABLE
    window, err := sdl.CreateWindow("nes", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, int32(640), int32(480), sdl.WINDOW_SHOWN)
    if err != nil {
        return err
    }
    defer window.Destroy()

    renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
    if err != nil {
        return err
    }

    err = ttf.Init()
    if err != nil {
        return err
    }

    defer ttf.Quit()

    /* FIXME: choose a font somehow */
    font, err := ttf.OpenFont("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 32)
    if err != nil {
        return err
    }
    defer font.Close()

    quit, cancel := context.WithCancel(context.Background())

    renderUpdates := make(chan NSFRenderState)

    go func(){
        white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
        fontHeight := font.Height()
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case state := <-renderUpdates:
                    renderer.Clear()

                    _ = state

                    y := 0
                    err := writeFont(font, renderer, 0, y, fmt.Sprintf("Song: %v", state.SongName), white)
                    if err != nil {
                        log.Printf("Unable to write font: %v", err)
                    }
                    y += fontHeight + 3

                    err = writeFont(font, renderer, 0, y, fmt.Sprintf("Track %v/%v", state.Track, state.MaxTrack), white)
                    if err != nil {
                        log.Printf("Unable to write font: %v", err)
                    }
                    y += fontHeight + 3
                    err = writeFont(font, renderer, 0, y, fmt.Sprintf("Play time %d:%02d", state.PlayTime / 60, state.PlayTime % 60), white)

                    // renderer.Copy(texture, nil, nil)
                    renderer.Present()
            }
        }
    }()

    go func(){
        var renderState NSFRenderState
        renderState.SongName = nsfFile.SongName
        renderState.MaxTrack = int(nsfFile.TotalSongs + 1)
        renderState.Track = 1
        renderUpdates <- renderState
        tick := 5.0 / 60.0 * 1000.0 * 1000.0
        timer := time.NewTicker(time.Duration(tick) * time.Microsecond)
        second := time.NewTicker(1 * time.Second)
        defer second.Stop()
        defer timer.Stop()
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case <-timer.C:
                    /* Force a refresh at least this often */
                    renderUpdates <- renderState
                case <-second.C:
                    renderState.PlayTime += 1
                    renderUpdates <- renderState
            }
        }
    }()

    const AudioSampleRate float32 = 44100

    audioDevice, err := setupAudio(AudioSampleRate)
    if err != nil {
        return fmt.Errorf("Could not initialize audio: %v", err)
    }

    defer sdl.CloseAudioDevice(audioDevice)
    log.Printf("Opened SDL audio device %v", audioDevice)
    sdl.PauseAudioDevice(audioDevice, false)

    audioOut := make(chan []float32, 2)
    actions := make(chan nes.NSFActions)

    go func(){
        <-quit.Done()
        close(audioOut)
    }()

    var waiter sync.WaitGroup

    if audioDevice != 0 {
        waiter.Add(1)
        go func(){
            defer waiter.Done()
            runAudio(audioDevice, audioOut)
        }()
    }

    track := byte(0)
    go func(){
        err := nes.PlayNSF(nsfFile, track, audioOut, AudioSampleRate, actions, quit)
        if err != nil {
            log.Printf("Error playing nsf: %v", err)
            cancel()
        }
    }()

    for quit.Err() == nil {
        event := sdl.WaitEvent()
        if event != nil {
            // log.Printf("Event %+v\n", event)
            switch event.GetType() {
                case sdl.QUIT: cancel()
                case sdl.KEYDOWN:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    quit_pressed := keyboard_event.Keysym.Scancode == sdl.SCANCODE_ESCAPE || keyboard_event.Keysym.Sym == sdl.K_ESCAPE || keyboard_event.Keysym.Sym == sdl.K_CAPSLOCK
                    if quit_pressed {
                        cancel()
                    }

                case sdl.KEYUP:
            }
        }
    }

    waiter.Wait()

    return err
}

func runAudio(audioDevice sdl.AudioDeviceID, audio chan []float32){
    var buffer bytes.Buffer
    for samples := range audio {
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

func RunNES(path string, debug bool, maxCycles uint64, windowSizeMultiple int, recordOnStart bool) error {
    nesFile, err := nes.ParseNesFile(path, true)
    if err != nil {
        return err
    }

    // force a software renderer
    // sdl.SetHint(sdl.HINT_RENDER_DRIVER, "software")

    err = sdl.Init(sdl.INIT_EVERYTHING)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    /* Number of pixels on the top and bottom of the screen to hide */
    overscanPixels := 8

    /* to resize the window */
    // | sdl.WINDOW_RESIZABLE
    window, err := sdl.CreateWindow("nes", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, int32(256 * windowSizeMultiple), int32((240 - overscanPixels * 2) * windowSizeMultiple), sdl.WINDOW_SHOWN)
    if err != nil {
        return err
    }
    defer window.Destroy()

    /*
    surface, err := window.GetSurface()
    if err != nil {
        return err
    }

    surface.FillRect(nil, 0)
    window.UpdateSurface()
    */

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

    // renderer.SetScale(2, 2)

    /*
    texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_RGB888, sdl.TEXTUREACCESS_TARGET, 640, 480)
    if err != nil {
        return err
    }

    // _ = texture
    // renderer.SetRenderTarget(texture)
    */

    var waiter sync.WaitGroup

    mainQuit, mainCancel := context.WithCancel(context.Background())
    defer mainCancel()
    toDraw := make(chan nes.VirtualScreen, 1)
    bufferReady := make(chan nes.VirtualScreen, 1)

    desiredFps := 60.0
    pixelFormat := findPixelFormat()

    /* create a surface from the pixels in one call, then create a texture and render it */
    doRender := func(screen nes.VirtualScreen, raw_pixels []byte) error {
        width := int32(256)
        height := int32(240 - overscanPixels * 2)
        depth := 8 * 4 // RGBA8888
        pitch := int(width) * int(depth) / 8

        startPixel := overscanPixels * int(width)
        endPixel := (240 - overscanPixels) * int(width)

        for i, pixel := range screen.Buffer[startPixel:endPixel] {
            /* red */
            raw_pixels[i*4+0] = byte(pixel >> 24)
            /* green */
            raw_pixels[i*4+1] = byte(pixel >> 16)
            /* blue */
            raw_pixels[i*4+2] = byte(pixel >> 8)
            /* alpha */
            raw_pixels[i*4+3] = byte(pixel >> 0)
        }

        pixels := C.CBytes(raw_pixels)
        defer C.free(pixels)

        // pixelFormat := sdl.PIXELFORMAT_ABGR8888

        /* pixelFormat should be ABGR8888 on little-endian (x86) and
         * RBGA8888 on big-endian (arm)
         */

        surface, err := sdl.CreateRGBSurfaceWithFormatFrom(pixels, width, height, int32(depth), int32(pitch), pixelFormat)
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

        renderer.Clear()
        renderer.Copy(texture, nil, nil)
        renderer.Present()

        return nil
    }

    go func(){
        waiter.Add(1)
        buffer := nes.MakeVirtualScreen(256, 240)
        bufferReady <- buffer
        defer waiter.Done()
        raw_pixels := make([]byte, 256*(240-overscanPixels*2) * 4)
        fpsCounter := 2.0
        fps := 0
        fpsTimer := time.NewTicker(time.Duration(fpsCounter) * time.Second)
        defer fpsTimer.Stop()

        renderTimer := time.NewTicker(time.Second / time.Duration(desiredFps))
        defer renderTimer.Stop()
        canRender := false
        for {
            select {
                case <-mainQuit.Done():
                    return
                case screen := <-toDraw:
                    if canRender {
                        err := doRender(screen, raw_pixels)
                        fps += 1
                        if err != nil {
                            log.Printf("Could not render: %v\n", err)
                        }
                    }
                    canRender = false
                    bufferReady <- screen
                case <-renderTimer.C:
                    canRender = true
                case <-fpsTimer.C:
                    log.Printf("FPS: %v", int(float64(fps) / fpsCounter))
                    fps = 0
            }
        }
    }()

    emulatorActions := make(chan EmulatorAction, 50)

    var screenListeners ScreenListeners

    startNES := func(quit context.Context, waiter *sync.WaitGroup){
        waiter.Add(1)
        defer waiter.Done()

        cpu, err := setupCPU(nesFile, debug)

        var quitEvent sdl.QuitEvent
        quitEvent.Type = sdl.QUIT
        /* FIXME: does quitEvent.Timestamp need to be set? */

        if err != nil {
            log.Printf("Error: CPU initialization error: %v", err)
            /* The main loop below is waiting for an event so we push the quit event */
            sdl.PushEvent(&quitEvent)
        } else {
            audio := make(chan []float32, 2)
            defer close(audio)

            if audioDevice != 0 {
                /* runNES will generate arrays of samples that we enqueue into the SDL audio system */
                go func(){
                    waiter.Add(1)
                    defer waiter.Done()

                    var buffer bytes.Buffer
                    for samples := range audio {
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
                }()
            }

            log.Printf("Run NES")
            err = runNES(&cpu, maxCycles, quit, toDraw, bufferReady, audio, emulatorActions, &screenListeners, AudioSampleRate)
            if err != nil {
                if err == MaxCyclesReached {
                } else {
                    log.Printf("Error running NES: %v", err)
                }

                sdl.PushEvent(&quitEvent)
            }
        }
    }

    var nesWaiter sync.WaitGroup
    nesQuit, nesCancel := context.WithCancel(mainQuit)
    go startNES(nesQuit, &nesWaiter)

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
        err := RecordMp4(recordQuit, stripExtension(filepath.Base(path)), overscanPixels, int(AudioSampleRate), &screenListeners)
        if err != nil {
            log.Printf("Error: could not record: %v", err)
        }
    } else {
        recordCancel()
    }

    for mainQuit.Err() == nil {
        event := sdl.WaitEvent()
        if event != nil {
            // log.Printf("Event %+v\n", event)
            switch event.GetType() {
                case sdl.QUIT: mainCancel()
                case sdl.KEYDOWN:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    // log.Printf("key down %+v pressed %v escape %v", keyboard_event, keyboard_event.State == sdl.PRESSED, keyboard_event.Keysym.Sym == sdl.K_ESCAPE)
                    quit_pressed := keyboard_event.State == sdl.PRESSED && (keyboard_event.Keysym.Sym == sdl.K_ESCAPE || keyboard_event.Keysym.Sym == sdl.K_CAPSLOCK)
                    if quit_pressed {
                        mainCancel()
                    }

                    if keyboard_event.Keysym.Scancode == turboKey {
                        select {
                            case emulatorActions <- EmulatorTurbo:
                            default:
                        }
                    }

                    if keyboard_event.Keysym.Scancode == stepFrameKey {
                        select {
                            case emulatorActions <- EmulatorStepFrame:
                        }
                    }

                    if keyboard_event.Keysym.Scancode == recordKey {
                        if recordQuit.Err() == nil {
                            recordCancel()
                        } else {
                            recordQuit, recordCancel = context.WithCancel(mainQuit)
                            err := RecordMp4(recordQuit, stripExtension(filepath.Base(path)), overscanPixels, int(AudioSampleRate), &screenListeners)
                            if err != nil {
                                log.Printf("Could not record video: %v", err)
                            }
                        }
                    }

                    if keyboard_event.Keysym.Scancode == pauseKey {
                        log.Printf("Pause/unpause")
                        select {
                            case emulatorActions <- EmulatorTogglePause:
                            default:
                        }
                    }

                    if keyboard_event.Keysym.Scancode == ppuDebugKey {
                        select {
                            case emulatorActions <- EmulatorTogglePPUDebug:
                            default:
                        }
                    }

                    if keyboard_event.Keysym.Scancode == slowDownKey {
                        select {
                            case emulatorActions <- EmulatorSlowDown:
                            default:
                        }
                    }

                    if keyboard_event.Keysym.Scancode == speedUpKey {
                        select {
                            case emulatorActions <- EmulatorSpeedUp:
                            default:
                        }
                    }

                    if keyboard_event.Keysym.Scancode == normalKey {
                        select {
                            case emulatorActions <- EmulatorNormal:
                            default:
                        }
                    }

                    if keyboard_event.Keysym.Scancode == hardResetKey {
                        log.Printf("Hard reset")
                        nesCancel()

                        nesWaiter.Wait()

                        nesQuit, nesCancel = context.WithCancel(mainQuit)
                        go startNES(nesQuit, &nesWaiter)
                    }
                case sdl.KEYUP:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    scancode := keyboard_event.Keysym.Scancode
                    if scancode == turboKey || scancode == pauseKey {
                        select {
                            case emulatorActions <- EmulatorNormal:
                            default:
                        }
                    }
            }
        }
    }

    log.Printf("Waiting to quit..")
    nesWaiter.Wait()
    waiter.Wait()

    return nil
}

/* determine endianness of the host by comparing the least-significant byte of a 32-bit number
 * versus a little endian byte array
 * if the first byte in the byte array is the same as the lowest byte of the 32-bit number
 * then the host is little endian
 */
func findPixelFormat() uint32 {
    red := uint32(32)
    green := uint32(128)
    blue := uint32(64)
    alpha := uint32(96)
    color := (red << 24) | (green << 16) | (blue << 8) | alpha

    var buffer bytes.Buffer
    binary.Write(&buffer, binary.LittleEndian, color)

    if buffer.Bytes()[0] == uint8(alpha) {
        return sdl.PIXELFORMAT_ABGR8888
    }

    return sdl.PIXELFORMAT_RGBA8888
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

var MaxCyclesReached error = errors.New("maximum cycles reached")
func runNES(cpu *nes.CPUState, maxCycles uint64, quit context.Context, toDraw chan<- nes.VirtualScreen,
            bufferReady <-chan nes.VirtualScreen, audio chan<-[]float32,
            emulatorActions <-chan EmulatorAction, screenListeners *ScreenListeners,
            sampleRate float32) error {
    instructionTable := nes.MakeInstructionDescriptiontable()

    screen := nes.MakeVirtualScreen(256, 240)

    var cycleCounter float64

    /* run the host timer at this frequency (in ms) so that the counter
     * doesn't tick too fast
     *
     * anything higher than 1 seems ok, with 10 probably being an upper limit
     */
    hostTickSpeed := 5
    cycleDiff := nes.CPUSpeed / (1000.0 / float64(hostTickSpeed))

    /* about 20.292 */
    baseCyclesPerSample := nes.CPUSpeed / 2 / float64(sampleRate)

    cycleTimer := time.NewTicker(time.Duration(hostTickSpeed) * time.Millisecond)
    defer cycleTimer.Stop()

    turboMultiplier := float64(1)

    lastCpuCycle := cpu.Cycle

    paused := false

    stepFrame := false

    for quit.Err() == nil {
        if maxCycles > 0 && cpu.Cycle >= maxCycles {
            log.Printf("Maximum cycles %v reached", maxCycles)
            return MaxCyclesReached
        }

        for cycleCounter <= 0 {
            select {
                case <-quit.Done():
                    return nil
                case action := <-emulatorActions:
                    switch action {
                        case EmulatorTurbo:
                            turboMultiplier = 3
                        case EmulatorNormal:
                            turboMultiplier = 1
                            log.Printf("Emulator speed set to %v", turboMultiplier)
                        case EmulatorSlowDown:
                            turboMultiplier -= 0.1
                            if turboMultiplier < 0.1 {
                                turboMultiplier = 0.1
                            }
                            log.Printf("Emulator speed set to %v", turboMultiplier)
                        case EmulatorStepFrame:
                            stepFrame = !stepFrame
                            log.Printf("Emulator step frame is %v", stepFrame)
                        case EmulatorSpeedUp:
                            turboMultiplier += 0.1
                            log.Printf("Emulator speed set to %v", turboMultiplier)
                        case EmulatorTogglePause:
                            paused = !paused
                        case EmulatorTogglePPUDebug:
                            cpu.PPU.ToggleDebug()
                    }
                case <-cycleTimer.C:
                    cycleCounter += cycleDiff * turboMultiplier
            }

            if paused {
                cycleCounter = 0
            }
        }

        // log.Printf("Cycle counter %v\n", cycleCounter)

        err := cpu.Run(instructionTable)
        if err != nil {
            return err
        }
        usedCycles := cpu.Cycle

        cycleCounter -= float64(usedCycles - lastCpuCycle)

        audioData := cpu.APU.Run((float64(usedCycles) - float64(lastCpuCycle)) / 2.0, turboMultiplier * baseCyclesPerSample, cpu)

        if audioData != nil {
            screenListeners.ObserveAudio(audioData)

            // log.Printf("Send audio data via channel")
            select {
                case audio<- audioData:
                default:
                    log.Printf("Warning: audio falling behind")
            }
        }

        /* ppu runs 3 times faster than cpu */
        nmi, drawn := cpu.PPU.Run((usedCycles - lastCpuCycle) * 3, screen)

        if drawn {
            screenListeners.ObserveVideo(screen)

            select {
                case buffer := <-bufferReady:
                    buffer.CopyFrom(&screen)
                    toDraw <- buffer
                    if stepFrame {
                        paused = true
                    }
                default:
            }
        }

        lastCpuCycle = usedCycles

        if nmi {
            if cpu.Debug > 0 {
                log.Printf("Cycle %v Do NMI\n", cpu.Cycle)
            }
            cpu.NMI()
        }
    }

    // log.Printf("CPU cycles %v waited %v nanoseconds out of %v", cpu.Cycle, totalWait, time.Now().Sub(realStart).Nanoseconds())

    return nil
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
            err := RunNES(arguments.NESPath, arguments.Debug, arguments.MaxCycles, arguments.WindowSizeMultiple, arguments.Record)
            if err != nil {
                log.Printf("Error: %v\n", err)
            }
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

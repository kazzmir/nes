package main

import (
    "fmt"
    "log"
    "strconv"
    "os"
    "os/signal"
    "io"
    "io/fs"
    "path/filepath"
    "math"
    "strings"
    "bufio"
    "errors"

    "encoding/binary"
    "bytes"
    "time"
    "context"
    "runtime/pprof"
    "runtime"

    nes "github.com/kazzmir/nes/lib"
    "github.com/kazzmir/nes/lib/coroutine"
    "github.com/kazzmir/nes/util"

    "github.com/kazzmir/nes/cmd/nes/common"
    "github.com/kazzmir/nes/cmd/nes/gfx"
    "github.com/kazzmir/nes/cmd/nes/menu"
    "github.com/kazzmir/nes/cmd/nes/debug"
    "github.com/kazzmir/nes/data"

    // rdebug "runtime/debug"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
    audiolib "github.com/hajimehoshi/ebiten/v2/audio"
    "github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

type DrawStep struct {
    Draw func(*ebiten.Image)
    DrawPrevious bool
}

type Engine struct {
    Coroutine *coroutine.Coroutine
    Draws []DrawStep
    Quit context.Context
    WindowSize common.WindowSize
}

func (engine *Engine) Update() error {
    if engine.Quit.Err() != nil {
        return ebiten.Termination
    }

    /*
    keys := inpututil.AppendJustPressedKeys(nil)

    for _, key := range keys {
        switch key {
            case ebiten.KeyEscape, ebiten.KeyCapsLock:
                return ebiten.Termination
        }
    }
    */

    return engine.Coroutine.Run()
}

func (engine *Engine) Layout(outsideWidth, outsideHeight int) (int, int) {
    engine.WindowSize = common.WindowSize{X: outsideWidth, Y: outsideHeight}
    return outsideWidth, outsideHeight
}

func (engine *Engine) GetWindowSize() common.WindowSize {
    return engine.WindowSize
}

func (engine *Engine) Draw(screen *ebiten.Image) {

    var draw func(int)

    draw = func(i int){
        if i < 0 {
            return
        }

        step := engine.Draws[i]
        if step.DrawPrevious {
            draw(i - 1)
        }

        step.Draw(screen)
    }

    draw(len(engine.Draws) - 1)
}

func (engine *Engine) PushDraw(draw func(*ebiten.Image), drawPrevious bool) {
    engine.Draws = append(engine.Draws, DrawStep{Draw: draw, DrawPrevious: drawPrevious})
}

func (engine *Engine) PopDraw() {
    if len(engine.Draws) > 0 {
        engine.Draws = engine.Draws[:len(engine.Draws)-1]
    }
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

type AudioState struct {
    Enabled bool
}

type AudioToggle struct {
}

type AudioQueryEnabled struct {
    Response chan bool
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

func loadFontSource() (*text.GoTextFaceSource, error) {
    file, err := data.OpenFile("DejaVuSans.ttf")
    if err != nil {
        return nil, err
    }
    defer file.Close()

    return text.NewGoTextFaceSource(file)
}

type AudioPlayer struct {
    AudioChannel <-chan []float32
    Buffer []float32
    position int
}

func (player *AudioPlayer) Read(output []byte) (int, error) {
    select {
        case samples := <-player.AudioChannel:
            player.Buffer = append(player.Buffer, samples...)
        default:
    }

    maxSamples := min(len(player.Buffer) - player.position, len(output) / 4 / 2)

    // in the browser we have to return something
    if runtime.GOOS == "js" && maxSamples == 0 {
        for _, i := range output {
            output[i] = 0
        }

        return len(output), nil
    }

    // log.Printf("Audio wants %v samples, will render %v", len(output) / 4 / 2, maxSamples)

    count := 0
    for count < maxSamples {
        i := count * 2
        sample := player.Buffer[player.position + count]
        binary.LittleEndian.PutUint32(output[4*i:], math.Float32bits(sample))
        binary.LittleEndian.PutUint32(output[4*(i+1):], math.Float32bits(sample))
        count += 1
    }
    // log.Printf("Audio read %v samples\n", maxSamples)
    player.position += maxSamples
    if len(player.Buffer) > 1024 * 1024 {
        log.Printf("reset audio buffer")
        player.Buffer = player.Buffer[player.position:]
        player.position = 0
    }

    return maxSamples * 4 * 2, nil

    /*
    for i, sample := range samples {
        binary.LittleEndian.PutUint32(output[4*i:], math.Float32bits(sample))
    }
    return len(samples) * 4, nil
    */
}

type ProgramState struct {
    loadRom chan common.ProgramLoadRom
    audioEnabled bool
}

func (state *ProgramState) IsSoundEnabled() bool {
    return state.audioEnabled
}

func (state *ProgramState) LoadRom(name string, file common.MakeFile) {
    select {
        case state.loadRom <- common.ProgramLoadRom{Name: name, File: file}:
        default:
            log.Printf("Warning: could not send load rom request")
    }
}

func (state *ProgramState) SetSoundEnabled(enabled bool) {
    state.audioEnabled = enabled
}

type MessageTime struct {
    Message string
    Time time.Time
}

type OverlayMessages struct {
    Messages []MessageTime
}

func (messages *OverlayMessages) Add(message string) {
    messages.Messages = append(messages.Messages, MessageTime{Message: message, Time: time.Now()})
}

func (messages *OverlayMessages) Clear() {
    messages.Messages = nil
}

func (messages *OverlayMessages) Process() {
    for len(messages.Messages) > 0 {
        if time.Since(messages.Messages[0].Time) > time.Second * 2 {
            messages.Messages = messages.Messages[1:]
        } else {
            break
        }
    }
}

type AudioManager struct {
    Beep *audiolib.Player
}

func (audioManager *AudioManager) PlayBeep() {
    audioManager.Beep.Rewind()
    audioManager.Beep.Play()
}

func MakeAudioManager(context *audiolib.Context) *AudioManager {
    beep := context.NewPlayerFromBytes([]byte{})
    file, err := data.OpenFile("beep.ogg")
    if err != nil {
    } else {
        defer file.Close()

        allData, err := io.ReadAll(file)
        if err == nil {
            reader, err := vorbis.DecodeWithSampleRate(context.SampleRate(), bytes.NewReader(allData))
            if err == nil {
                beep, err = context.NewPlayer(reader)
                if err != nil {
                    log.Printf("Warning: could not create beep audio player: %v", err)
                }
            }
        }
    }

    return &AudioManager{
        Beep: beep,
    }
}

func RunNES(path string, debugCpu bool, debugPpu bool, maxCycles uint64, windowSizeMultiple int, recordOnStart bool, desiredFps int, recordInput bool, replayKeys string) error {
    nesChannel := make(chan NesAction, 10)
    doMenu := make(chan bool, 5)

    // if mainCancel is called then the program should exit
    mainQuit, mainCancel := context.WithCancel(context.Background())
    defer mainCancel()

    engine := Engine{
        Quit: mainQuit,
    }

    var overlayMessages OverlayMessages

    programActions := ProgramState{
        loadRom: make(chan common.ProgramLoadRom, 1),
        audioEnabled: true,
    }

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
        overlayMessages.Add("No ROM loaded")
    }

    var err error
    _ = err

    log.Printf("Create window")

    const AudioSampleRate float32 = 44100

    audio := audiolib.NewContext(int(AudioSampleRate))

    audioManager := MakeAudioManager(audio)

    signalChannel := make(chan os.Signal, 10)
    signal.Notify(signalChannel, os.Interrupt)

    go func(){
        for i := range 2 {
            select {
                // case <-mainQuit.Done():
                case <-signalChannel:
                    if i == 0 {
                        log.Printf("Shutting down due to signal")
                        mainCancel()
                        go func(){
                            log.Printf("Will exit in at most 2 seconds")
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


    fontSource, err := loadFontSource() 
    if err != nil {
        return fmt.Errorf("Unable to load font source: %v", err)
    }

    font := &text.GoTextFace{
        Source: fontSource,
        Size: 20,
    }

    smallFont := &text.GoTextFace{
        Source: fontSource,
        Size: 15,
    }

    consoleFont := &text.GoTextFace{
        Source: fontSource,
        Size: 17,
    }

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

    makeRenderScreen := func(bufferReady chan bool, buffer nes.VirtualScreen) func(*ebiten.Image) {
        /* FIXME: kind of ugly to keep this here */
        raw_pixels := make([]byte, nes.VideoWidth*(nes.VideoHeight-nes.OverscanPixels*2) * 4)

        // buffer := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)
        // defer waiter.Done()
        fpsCounter := 2.0
        fps := 0
        fpsTimer := time.NewTicker(time.Duration(fpsCounter) * time.Second)
        // defer fpsTimer.Stop()

        bufferImage := ebiten.NewImage(nes.VideoWidth, nes.VideoHeight - nes.OverscanPixels * 2)

        return func(screen *ebiten.Image){

            // log.Printf("call draw")

            select {
                case <-fpsTimer.C:
                    /* FIXME: don't print this while the menu is running */
                    log.Printf("FPS: %v", int(float64(fps) / fpsCounter))
                    fps = 0
                default:
            }

            fps += 1
            select {
                case <-bufferReady:
                    common.RenderPixelsRGBA(buffer, raw_pixels, nes.OverscanPixels)
                    bufferImage.WritePixels(raw_pixels)
                default:
            }

            var options ebiten.DrawImageOptions
            screenBounds := screen.Bounds()
            bufferBounds := bufferImage.Bounds()

            scaleX := float64(screenBounds.Dx()) / float64(bufferBounds.Dx())
            scaleY := float64(screenBounds.Dy()) / float64(bufferBounds.Dy())
            scale := min(scaleX, scaleY)

            options.GeoM.Scale(scale, scale)

            xDiff := float64(screenBounds.Dx()) - float64(bufferBounds.Dx()) * scale
            yDiff := float64(screenBounds.Dy()) - float64(bufferBounds.Dy()) * scale

            options.GeoM.Translate(xDiff / 2, yDiff / 2)
            screen.DrawImage(bufferImage, &options)
        }
    }

    emulatorActions := make(chan common.EmulatorAction, 50)
    emulatorActionsInput := (<-chan common.EmulatorAction)(emulatorActions)
    emulatorActionsOutput := (chan<- common.EmulatorAction)(emulatorActions)

    var screenListeners common.ScreenListeners

    audioActions := make(chan AudioActions, 2)
    audioActionsInput := (<-chan AudioActions)(audioActions)
    audioActionsOutput := (chan<- AudioActions)(audioActions)

    /*
    audioChannel := make(chan []float32, 2)
    audioInput := (<-chan []float32)(audioChannel)
    audioOutput := (chan<- []float32)(audioChannel)
    */

    emulatorKeys := common.LoadEmulatorKeys()
    input := &common.SDLKeyboardButtons{
        Keys: &emulatorKeys,
    }

    debugWindow := debug.MakeDebugWindow(mainQuit, font, smallFont)
    console := MakeConsole(mainCancel, mainQuit, emulatorActionsOutput, nesChannel)

    startNES := func(nesFile nes.NESFile, quit context.Context, yield coroutine.YieldFunc){
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

        if err != nil {
            log.Printf("Error: CPU initialization error: %v", err)
            /* The main loop below is waiting for an event so we push the quit event */
            overlayMessages.Add("Unable to load")
            // common.RunDummyNES(quit, emulatorActionsInput)
        } else {
            /* make sure no message appears on the screen in front of the nes output */
            log.Printf("Run NES")

            bufferReady := make(chan bool, 1)

            buffer := nes.MakeVirtualScreen(nes.VideoWidth, nes.VideoHeight)

            engine.PushDraw(makeRenderScreen(bufferReady, buffer), false)
            defer engine.PopDraw()

            _, fontHeight := text.Measure("A", font, 1)
            engine.PushDraw(func(screen *ebiten.Image){
                var textOptions text.DrawOptions
                textOptions.GeoM.Translate(float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy() - 1))
                for i := len(overlayMessages.Messages) - 1; i >= 0; i-- {
                    message := overlayMessages.Messages[i]

                    width, _ := text.Measure(message.Message, font, 1)
                    textOptions.GeoM.Translate(-width - 1, -float64(fontHeight) - 1)

                    alpha := float32(1.0)

                    textOptions.ColorScale.Reset()

                    elapsed := time.Since(message.Time)
                    if elapsed > time.Second {
                        alpha = min(1.0, max(0, float32(1.0 - float32(elapsed - time.Second) / float32(time.Second))))
                    }

                    textOptions.ColorScale.ScaleAlpha(alpha)

                    text.Draw(screen, message.Message, font, &textOptions)

                    textOptions.GeoM.Translate(width + 1, 0)
                }
            }, true)
            defer engine.PopDraw()

            engine.PushDraw(func(screen *ebiten.Image){
                console.Render(screen, consoleFont)
            }, true)
            defer engine.PopDraw()

            verbose := 1

            /*
            nesAudio := AudioPlayer{
                AudioChannel: audioInput,
            }
            */

            musicPlayer, err := audio.NewPlayerF32(cpu.APU.GetAudioStream())
            if err != nil {
                log.Printf("Warning: could not create audio player: %v", err)
            } else {
                musicPlayer.SetBufferSize(time.Millisecond * 50)

                musicPlayer.Play()
                defer musicPlayer.Pause()

                if ! programActions.IsSoundEnabled() {
                    musicPlayer.SetVolume(0)
                }
            }

            runNes := func(nesYield coroutine.YieldFunc) error {
                return common.RunNES(nesFile.Path, &cpu, maxCycles, quit, bufferReady, buffer, emulatorActionsInput, &screenListeners, &overlayMessages, AudioSampleRate, verbose, debugger, nesYield)
            }

            nesCoroutine := coroutine.MakeCoroutine(runNes)
            defer nesCoroutine.Stop()

            var keys []ebiten.Key

            systemPaused := false
            audioPaused := 0

            for quit.Err() == nil {
                select {
                    case action := <-audioActionsInput:
                        switch action.(type) {
                            case *AudioState:
                                state := action.(*AudioState)
                                if state.Enabled {
                                    musicPlayer.SetVolume(1.0)
                                } else {
                                    musicPlayer.SetVolume(0)
                                }
                        }
                    default:
                }

                keys = inpututil.AppendJustPressedKeys(keys[:0])

                // toggling the console could lead to re-enabling it, so check if we should skip normal input
                skipInput := console.IsActive()

                console.Update(mainCancel, emulatorActionsOutput, nesChannel, keys, emulatorKeys.Console)
                overlayMessages.Process()

                if !skipInput {
                    for _, key := range keys {
                        switch key {
                            case ebiten.KeyEscape, ebiten.KeyCapsLock:
                                select {
                                    case doMenu <- true:
                                        musicPlayer.Pause()
                                        audioPaused += 1
                                        // the menu will launch by virtue of the doMenu channel
                                        yield()
                                        audioPaused -= 1
                                        if audioPaused == 0 {
                                            musicPlayer.Play()
                                        }
                                    default:
                                        // couldn't launch menu, just abort
                                        mainCancel()
                                }
                                // mainCancel()
                            case emulatorKeys.Turbo:
                                select {
                                    case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorTurbo):
                                    default:
                                }
                            case emulatorKeys.Console:
                                console.Toggle()
                            case emulatorKeys.StepFrame:
                                select {
                                    case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorStepFrame):
                                    default:
                                }
                            case emulatorKeys.SaveState:
                                select {
                                    case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSaveState):
                                        overlayMessages.Add("Saved state")
                                    default:
                                }
                            case emulatorKeys.LoadState:
                                select {
                                    case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorLoadState):
                                        overlayMessages.Add("Loaded state")
                                    default:
                                }
                            case emulatorKeys.Record:
                                /*
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
                                */
                            case emulatorKeys.Pause:
                                log.Printf("Pause/unpause")
                                select {
                                    case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorTogglePause):
                                    default:
                                }
                                systemPaused = !systemPaused
                                if systemPaused {
                                    audioPaused += 1
                                    musicPlayer.Pause()
                                } else {
                                    audioPaused -= 1
                                    if audioPaused == 0 {
                                        musicPlayer.Play()
                                    }
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
                                overlayMessages.Add("Hard reset")

                                /*
                                nesCoroutine.Stop()
                                nesCoroutine = coroutine.MakeCoroutine(runNes)
                                defer nesCoroutine.Stop()
                                */
                                return
                        }

                        input.HandleEvent(key, true)
                    }

                    keys = inpututil.AppendJustReleasedKeys(keys[:0])
                    for _, key := range keys {
                        switch key {
                            case emulatorKeys.Turbo:
                                select {
                                    case emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorNormal):
                                    default:
                                }
                        }

                        input.HandleEvent(key, false)
                    }
                }

                err := nesCoroutine.Run()
                if err != nil {
                    if err == common.MaxCyclesReached || errors.Is(err, coroutine.CoroutineCancelled) {
                    } else {
                        log.Printf("Error running NES: %v", err)
                    }

                    return
                }

                if yield() != nil {
                    return
                }
            }
        }
    }

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

    /*
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
    */

    /* enable drag/drop events */

    /*
    events := make(chan sdl.Event, 20)

    handleOneEvent := func(event sdl.Event){
        switch event.GetType() {
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

    menu := coroutine.MakeCoroutine(func(yield coroutine.YieldFunc) error {
        nesQuit, nesCancel := context.WithCancel(mainQuit)

        defer nesCancel()

        var currentFile nes.NESFile

        nesCoroutine := coroutine.MakeCoroutine(func(nesYield coroutine.YieldFunc) error {
            var textOptions text.DrawOptions
            engine.PushDraw(func(screen *ebiten.Image){
                textOptions.GeoM.Reset()
                textOptions.GeoM.Translate(20, 20)
                text.Draw(screen, "Drag and drop a rom", font, &textOptions)
                textOptions.GeoM.Translate(0, 30)
                text.Draw(screen, "or press Escape/CapsLock to open the menu", font, &textOptions)
            }, true)
            defer engine.PopDraw()

            var keys []ebiten.Key
            for nesQuit.Err() == nil {
                keys = inpututil.AppendJustPressedKeys(keys[:0])
                for _, key := range keys {
                    switch key {
                        case ebiten.KeyEscape, ebiten.KeyCapsLock:
                            select {
                                case doMenu <- true:
                                default:
                            }
                    }
                }

                common.RunDummyNES(emulatorActionsInput)

                if nesYield() != nil {
                    return coroutine.CoroutineCancelled
                }
            }

            return nil
        })

        for mainQuit.Err() == nil {
            droppedFiles := ebiten.DroppedFiles()
            if droppedFiles != nil {
                entries, err := fs.ReadDir(droppedFiles, ".")
                if err == nil {
                    for _, droppedFile := range entries {
                        func(){
                            name := droppedFile.Name()
                            file, err := droppedFiles.Open(name)
                            if err != nil {
                                log.Printf("Could not load dropped rom '%v'", name)
                                return
                            }
                            defer file.Close()
                            nesFile, err := nes.ParseNes(file, true, name)
                            if err != nil {
                                log.Printf("Could not load dropped rom '%v'", name)
                                return
                            }

                            log.Printf("Loaded rom '%v'", name)

                            select {
                                case nesChannel <- &NesActionLoad{File: nesFile}:
                                    overlayMessages.Add("Loaded rom")
                                default:
                                    log.Printf("Could not send load rom request for dropped file '%v'", name)
                            }
                        }()
                    }
                }
            }

            select {
                case <-doMenu:

                    activeMenu := menu.MakeMenu(mainQuit, font, audioManager)
                    // emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorSetPause)
                    activeMenu.Run(mainCancel, font, smallFont, &programActions, &renderManager, joystickManager, &emulatorKeys, yield, &engine)
                    // emulatorActionsOutput <- common.MakeEmulatorAction(common.EmulatorUnpause)

                    select {
                        case audioActionsOutput <- &AudioState{Enabled: programActions.IsSoundEnabled()}:
                        default:
                    }

                    select {
                        case loadRom := <-programActions.loadRom:
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
                                overlayMessages.Add("Loaded rom")
                            }

                        default:
                    }

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
                        nesQuit, nesCancel = context.WithCancel(mainQuit)
                        defer nesCancel()

                        nesCoroutine.Stop()
                        nesCoroutine = coroutine.MakeCoroutine(func(nesYield coroutine.YieldFunc) error {
                            startNES(currentFile, nesQuit, nesYield)
                            return nil
                        })
                    }

                default:
            }

            nesCoroutine.Run()

            if yield() != nil {
                return coroutine.CoroutineCancelled
            }
        }

        return nil
    })

    engine.Coroutine = menu

    return ebiten.RunGame(&engine)
}

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

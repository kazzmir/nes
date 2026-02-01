package main

import (
    "log"
    // "os"
    // "fmt"
    "context"
    // "encoding/binary"
    // "bytes"
    "time"
    // "sync"
    // "github.com/kazzmir/nes/util"
    nes "github.com/kazzmir/nes/lib"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
    audiolib "github.com/hajimehoshi/ebiten/v2/audio"
)

type NSFRenderState struct {
    SongName string
    Artist string
    Copyright string
    PlayTime uint64
    Paused bool
    Track int
    MaxTrack int
}

/*
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
*/

/*
func runAudio(audioDevice sdl.AudioDeviceID, audio chan []float32){
    var buffer bytes.Buffer
    for samples := range audio {
        // log.Printf("Prepare audio to queue")
        // log.Printf("Enqueue data %v", samples)
        buffer.Reset()
        / * convert []float32 into []byte * /
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
*/

type NSFPlayerActions int
const (
    NSFPlayerNext5Tracks = iota
    NSFPlayerNext
    NSFPlayerPrevious
    NSFPlayerPrevious5Tracks
    NSFPlayerPause
)

type NSFEngine struct {
    nsfFile *nes.NSFFile
    fontSource *text.GoTextFaceSource
    audio *audiolib.Context
    quit context.Context
}

func MakeNSFEngine(nsfFile *nes.NSFFile, fontSource *text.GoTextFaceSource, audio *audiolib.Context, quit context.Context) *NSFEngine {
    return &NSFEngine{
        nsfFile: nsfFile,
        fontSource: fontSource,
        audio: audio,
        quit: quit,
    }
}

func (engine *NSFEngine) Update() error {
    keys := inpututil.AppendJustPressedKeys(nil)
    for _, key := range keys {
        switch key {
            case ebiten.KeyEscape, ebiten.KeyCapsLock:
                return ebiten.Termination
        }
    }

    return nil
}

func (engine *NSFEngine) Draw(screen *ebiten.Image) {
}

func (engine *NSFEngine) Layout(outsideWidth, outsideHeight int) (int, int) {
    return outsideWidth, outsideHeight
}

func RunNSF(path string) error {
    nsfFile, err := nes.LoadNSF(path)
    if err != nil {
        return err
    }

    _ = nsfFile

    fontSource, err := loadFontSource()
    if err != nil {
        return err
    }

    quit, cancel := context.WithCancel(context.Background())
    defer cancel()

    const AudioSampleRate float32 = 44100
    audio := audiolib.NewContext(int(AudioSampleRate))

    nsfActions := make(chan NSFPlayerActions, 3)
    actions := make(chan nes.NSFActions)

    engine := MakeNSFEngine(&nsfFile, fontSource, audio, quit)

    ebiten.SetWindowTitle("NES Emulator")
    ebiten.SetWindowSize(600, 600)
    ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

    /* The 'controller' loop, that updates the 'renderState' model */
    go func(){
        var renderState NSFRenderState
        renderState.SongName = nsfFile.SongName
        renderState.Artist = nsfFile.Artist
        renderState.Copyright = nsfFile.Copyright
        renderState.MaxTrack = int(nsfFile.TotalSongs)
        renderState.Track = int(nsfFile.StartingSong)
        renderState.Paused = false

        playQuit, playCancel := context.WithCancel(quit)

        doPlay := func(playQuit context.Context, track byte){
            audioStream := make(chan *nes.AudioStream, 1)

            go func(){
                player, err := audio.NewPlayerF32(<-audioStream)
                if err != nil {
                    log.Printf("Error creating audio player: %v", err)
                    return
                }
                player.SetBufferSize(time.Millisecond * 50)
                player.Play()
                <-playQuit.Done()
                player.Pause()
            }()

            err := nes.PlayNSF(nsfFile, track, audioStream, AudioSampleRate, actions, playQuit, 0)
            if err != nil {
                log.Printf("Error playing nsf: %v", err)
                cancel()
            }
        }

        go doPlay(playQuit, byte(renderState.Track))

        second := time.NewTicker(1 * time.Second)
        defer second.Stop()
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case <-second.C:
                    if !renderState.Paused {
                        renderState.PlayTime += 1
                    }
                case action := <-nsfActions:
                    trackDelta := 0
                    switch action {
                        case NSFPlayerNext5Tracks:
                            trackDelta = 5
                        case NSFPlayerNext:
                            trackDelta = 1
                        case NSFPlayerPrevious:
                            trackDelta = -1
                        case NSFPlayerPrevious5Tracks:
                            trackDelta = -5
                        case NSFPlayerPause:
                            actions <- nes.NSFActionTogglePause
                            renderState.Paused = !renderState.Paused
                    }

                    if trackDelta != 0 {
                        newTrack := renderState.Track + trackDelta
                        if newTrack < 0 {
                            newTrack = 0
                        }
                        if newTrack >= renderState.MaxTrack {
                            newTrack = renderState.MaxTrack
                        }

                        if newTrack != renderState.Track {
                            renderState.Paused = false
                            renderState.Track = newTrack
                            renderState.PlayTime = 0
                            second.Reset(1 * time.Second)

                            playCancel()
                            playQuit, playCancel = context.WithCancel(quit)
                            go doPlay(playQuit, byte(renderState.Track))
                        }
                    }
            }
        }

        playCancel()
    }()

    err = ebiten.RunGame(engine)
    if err != nil {
        log.Printf("Error playing NSF: %v", err)
    }

    /*
    renderUpdates := make(chan NSFRenderState)

    / * The 'view' loop, that displays whats in the NSFRenderState model * /
    go func(){
        white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
        red := sdl.Color{R:255, G: 0, B: 0, A: 255}
        fontHeight := font.Height()
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case state := <-renderUpdates:
                    sdl.Do(func (){
                        renderer.Clear()

                        _ = state

                        x := 4
                        y := 4

                        err := writeFont(font, renderer, x, y, fmt.Sprintf("Song: %v", state.SongName), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("Artist: %v", state.Artist), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("Coyright: %v", state.Copyright), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("Track %v/%v", state.Track + 1, state.MaxTrack + 1), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        if state.Paused {
                            err = writeFont(font, renderer, x, y, fmt.Sprintf("Paused"), red)
                        } else {
                            err = writeFont(font, renderer, x, y, fmt.Sprintf("Play time %d:%02d", state.PlayTime / 60, state.PlayTime % 60), white)
                        }
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        y += fontHeight * 2

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("Keys"), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("> or l: skip 1 track ahead"), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("^ or k: skip 5 tracks ahead"), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("< or h: go 1 track back"), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("v or j: go 5 tracks back"), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }
                        y += fontHeight + 3

                        err = writeFont(font, renderer, x, y, fmt.Sprintf("esc: quit"), white)
                        if err != nil {
                            log.Printf("Unable to write font: %v", err)
                        }

                        // renderer.Copy(texture, nil, nil)
                        renderer.Present()
                    })
            }
        }
    }()

    doRender := make(chan bool, 2)
    audioOut := make(chan []float32, 2)

    

    audioDevice, err := setupAudio(AudioSampleRate)
    if err != nil {
        return fmt.Errorf("Could not initialize audio: %v", err)
    }

    defer sdl.Do(func(){
        sdl.CloseAudioDevice(audioDevice)
    })
    log.Printf("Opened SDL audio device %v", audioDevice)
    sdl.Do(func(){
        sdl.PauseAudioDevice(audioDevice, false)
    })

    go func(){
        <-quit.Done()
        / * FIXME: close this channel after writers are guaranteed not to be using it * /
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

    keyMapping := make(map[sdl.Keycode]NSFPlayerActions)
    keyMapping[sdl.K_UP] = NSFPlayerNext5Tracks
    keyMapping[sdl.K_k] = NSFPlayerNext5Tracks
    keyMapping[sdl.K_RIGHT] = NSFPlayerNext
    keyMapping[sdl.K_l] = NSFPlayerNext
    keyMapping[sdl.K_LEFT] = NSFPlayerPrevious
    keyMapping[sdl.K_h] = NSFPlayerPrevious
    keyMapping[sdl.K_DOWN] = NSFPlayerPrevious5Tracks
    keyMapping[sdl.K_j] = NSFPlayerPrevious5Tracks
    keyMapping[sdl.K_SPACE] = NSFPlayerPause

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

                    action, ok := keyMapping[keyboard_event.Keysym.Sym]
                    if ok {
                        nsfActions <- action
                    }
                case sdl.WINDOWEVENT:
                        window_event := event.(*sdl.WindowEvent)
                        switch window_event.Event {
                            case sdl.WINDOWEVENT_EXPOSED:
                                doRender <- true
                        }
                case sdl.KEYUP:
            }
        }
    }

    waiter.Wait()

    return err
    */
    return nil
}

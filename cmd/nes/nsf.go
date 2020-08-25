package main

import (
    "log"
    "path/filepath"
    "os"
    "fmt"
    "context"
    "encoding/binary"
    "bytes"
    "time"
    "sync"
    nes "github.com/kazzmir/nes/lib"
    "github.com/veandco/go-sdl2/sdl"
    "github.com/veandco/go-sdl2/ttf"
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

type NSFPlayerActions int
const (
    NSFPlayerNext5Tracks = iota
    NSFPlayerNext
    NSFPlayerPrevious
    NSFPlayerPrevious5Tracks
    NSFPlayerPause
)

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

    sdl.DisableScreenSaver()
    defer sdl.EnableScreenSaver()

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

    /* FIXME: choose a font somehow if this one is not found */
    font, err := ttf.OpenFont(filepath.Join(filepath.Dir(os.Args[0]), "font/DejaVuSans.ttf"), 20)
    if err != nil {
        return err
    }
    defer font.Close()

    quit, cancel := context.WithCancel(context.Background())

    renderUpdates := make(chan NSFRenderState)

    go func(){
        white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
        red := sdl.Color{R:255, G: 0, B: 0, A: 255}
        fontHeight := font.Height()
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case state := <-renderUpdates:
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
            }
        }
    }()

    doRender := make(chan bool, 2)
    nsfActions := make(chan NSFPlayerActions, 3)
    audioOut := make(chan []float32, 2)
    actions := make(chan nes.NSFActions)

    const AudioSampleRate float32 = 44100

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
            err := nes.PlayNSF(nsfFile, track, audioOut, AudioSampleRate, actions, playQuit)
            if err != nil {
                log.Printf("Error playing nsf: %v", err)
                cancel()
            }
        }

        go doPlay(playQuit, byte(renderState.Track))

        renderUpdates <- renderState
        second := time.NewTicker(1 * time.Second)
        defer second.Stop()
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case <-doRender:
                    renderUpdates <- renderState
                case <-second.C:
                    if !renderState.Paused {
                        renderState.PlayTime += 1
                        renderUpdates <- renderState
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
                            renderUpdates <- renderState
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
                            /* FIXME: in go 1.15 */
                            // second.Reset(1 * time.Second)

                            playCancel()
                            playQuit, playCancel = context.WithCancel(quit)
                            go doPlay(playQuit, byte(renderState.Track))
                        }

                        renderUpdates <- renderState
                    }
            }
        }
    }()

    audioDevice, err := setupAudio(AudioSampleRate)
    if err != nil {
        return fmt.Errorf("Could not initialize audio: %v", err)
    }

    defer sdl.CloseAudioDevice(audioDevice)
    log.Printf("Opened SDL audio device %v", audioDevice)
    sdl.PauseAudioDevice(audioDevice, false)

    go func(){
        <-quit.Done()
        /* FIXME: close this channel after writers are guaranteed not to be using it */
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

    keyMapping := make(map[sdl.Scancode]NSFPlayerActions)
    keyMapping[sdl.SCANCODE_UP] = NSFPlayerNext5Tracks
    keyMapping[sdl.SCANCODE_K] = NSFPlayerNext5Tracks
    keyMapping[sdl.SCANCODE_RIGHT] = NSFPlayerNext
    keyMapping[sdl.SCANCODE_L] = NSFPlayerNext
    keyMapping[sdl.SCANCODE_LEFT] = NSFPlayerPrevious
    keyMapping[sdl.SCANCODE_H] = NSFPlayerPrevious
    keyMapping[sdl.SCANCODE_DOWN] = NSFPlayerPrevious5Tracks
    keyMapping[sdl.SCANCODE_J] = NSFPlayerPrevious5Tracks
    keyMapping[sdl.SCANCODE_SPACE] = NSFPlayerPause

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

                    action, ok := keyMapping[keyboard_event.Keysym.Scancode]
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
}

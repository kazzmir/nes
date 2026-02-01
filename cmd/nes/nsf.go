package main

import (
    "log"
    "fmt"
    "context"
    "strings"
    "image/color"
    "time"
    "sync"
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
    keyMapping map[ebiten.Key]NSFPlayerActions
    nsfActions chan NSFPlayerActions
    renderState NSFRenderState

    lock sync.Mutex
}

func MakeNSFEngine(nsfFile *nes.NSFFile, fontSource *text.GoTextFaceSource, audio *audiolib.Context, quit context.Context, nsfActions chan NSFPlayerActions) *NSFEngine {
    keyMapping := make(map[ebiten.Key]NSFPlayerActions)
    keyMapping[ebiten.KeyUp] = NSFPlayerNext5Tracks
    keyMapping[ebiten.KeyK] = NSFPlayerNext5Tracks
    keyMapping[ebiten.KeyRight] = NSFPlayerNext
    keyMapping[ebiten.KeyL] = NSFPlayerNext
    keyMapping[ebiten.KeyLeft] = NSFPlayerPrevious
    keyMapping[ebiten.KeyH] = NSFPlayerPrevious
    keyMapping[ebiten.KeyDown] = NSFPlayerPrevious5Tracks
    keyMapping[ebiten.KeyJ] = NSFPlayerPrevious5Tracks
    keyMapping[ebiten.KeySpace] = NSFPlayerPause


    return &NSFEngine{
        nsfFile: nsfFile,
        fontSource: fontSource,
        audio: audio,
        quit: quit,
        keyMapping: keyMapping,
        nsfActions: nsfActions,
    }
}

func (engine *NSFEngine) SetRenderState(state NSFRenderState) {
    engine.lock.Lock()
    defer engine.lock.Unlock()
    engine.renderState = state
}

func (engine *NSFEngine) Update() error {
    keys := inpututil.AppendJustPressedKeys(nil)
    for _, key := range keys {
        switch key {
            case ebiten.KeyEscape, ebiten.KeyCapsLock:
                return ebiten.Termination
        }

        action, ok := engine.keyMapping[key]
        if ok {
            engine.nsfActions <- action
        }
    }

    return nil
}

func (engine *NSFEngine) Draw(screen *ebiten.Image) {
    engine.lock.Lock()
    state := engine.renderState
    engine.lock.Unlock()

    font := &text.GoTextFace{
        Source: engine.fontSource,
        Size: 20,
    }

    _, fontHeight := text.Measure("A", font, 1)

    var textOptions text.DrawOptions
    textOptions.GeoM.Translate(4, 4)
    for _, line := range []string{
        fmt.Sprintf("Song: %v", state.SongName),
        fmt.Sprintf("Artist: %v", state.Artist),
        fmt.Sprintf("Coyright: %v", state.Copyright),
        fmt.Sprintf("Track %v/%v", state.Track + 1, state.MaxTrack + 1),
    } {
        text.Draw(screen, line, font, &textOptions)
        textOptions.GeoM.Translate(0, fontHeight + 3)
    }

    if state.Paused {
        red := color.RGBA{R: 255, A: 255}
        textOptions.ColorScale.ScaleWithColor(red)
        text.Draw(screen, fmt.Sprintf("Paused"), font, &textOptions)
        textOptions.ColorScale.Reset()
    } else {
        text.Draw(screen, fmt.Sprintf("Play time %d:%02d", state.PlayTime / 60, state.PlayTime % 60), font, &textOptions)
    }

    textOptions.GeoM.Translate(0, fontHeight * 2)

    for _, line := range []string{
        "Keys",
        "> or l: skip 1 track ahead",
        "^ or k: skip 5 tracks ahead",
        "< or h: go 1 track back",
        "v or j: go 5 tracks back",
        "space: pause/resume",
        "esc: quit",
    } {
        text.Draw(screen, line, font, &textOptions)
        textOptions.GeoM.Translate(0, fontHeight + 3)
    }
}

func (engine *NSFEngine) Layout(outsideWidth, outsideHeight int) (int, int) {
    return outsideWidth, outsideHeight
}

func RunNSF(path string) error {
    nsfFile, err := nes.LoadNSF(path)
    if err != nil {
        return err
    }

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

    engine := MakeNSFEngine(&nsfFile, fontSource, audio, quit, nsfActions)

    ebiten.SetWindowTitle("NES Emulator")
    ebiten.SetWindowSize(600, 600)
    ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

    /* The 'controller' loop, that updates the 'renderState' model */
    go func(){
        var renderState NSFRenderState
        renderState.SongName = strings.TrimRight(nsfFile.SongName, "\x00")
        renderState.Artist = strings.TrimRight(nsfFile.Artist, "\x00")
        renderState.Copyright = strings.TrimRight(nsfFile.Copyright, "\x00")
        renderState.MaxTrack = int(nsfFile.TotalSongs)
        renderState.Track = int(nsfFile.StartingSong)
        renderState.Paused = false

        engine.SetRenderState(renderState)

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
            update := false
            select {
                case <-quit.Done():
                case <-second.C:
                    if !renderState.Paused {
                        renderState.PlayTime += 1
                        update = true
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
                            update = true
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
                            update = true
                            second.Reset(1 * time.Second)

                            playCancel()
                            playQuit, playCancel = context.WithCancel(quit)
                            go doPlay(playQuit, byte(renderState.Track))
                        }
                    }
            }

            if update {
                engine.SetRenderState(renderState)
            }
        }

        playCancel()
    }()

    err = ebiten.RunGame(engine)
    if err != nil {
        log.Printf("Error playing NSF: %v", err)
    }
    
    return nil
}

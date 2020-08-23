package main

import (
    "log"
    "fmt"
    "os"
    "context"
    "time"
    "path/filepath"

    nes "github.com/kazzmir/nes/lib"
    "github.com/veandco/go-sdl2/sdl"

    "github.com/jroimartin/gocui"
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


type PlayerAction int
const (
    PlayerNextTrack = iota
    PlayerNext5Track = iota
    PlayerPreviousTrack
    PlayerPrevious5Track
    PlayerQuit
    PlayerTogglePause
)

type RenderState struct {
    track byte
    playTime uint64
    paused bool
}

func terminalGui(quit context.Context, cancel context.CancelFunc, nsfPath string, nsf nes.NSFFile, pauseChannel chan bool, updateTrack chan byte, playerActions chan PlayerAction) (*gocui.Gui, error) {
    gui, err := gocui.NewGui(gocui.OutputNormal)
    gui.InputEsc = true
    // gui.Cursor = true

    var mainView *gocui.View

    gui.Update(func (gui *gocui.Gui) error {
        var err error

        infoView, err := gui.SetView("info", 0, 0, 50, 10)
        if err != nil && err != gocui.ErrUnknownView {
            log.Printf("GUI error: %v", err)
            return err
        }

        fmt.Fprintf(infoView, "NSF Player by Jon Rafkind\n")
        fmt.Fprintf(infoView, "File: %v\n", filepath.Base(nsfPath))
        fmt.Fprintf(infoView, "Artist: %v\n", nsf.Artist)
        fmt.Fprintf(infoView, "Song: %v\n", nsf.SongName)
        fmt.Fprintf(infoView, "Copyright: %v\n", nsf.Copyright)

        infoWidth, infoHeight := infoView.Size()
        mainView, err = gui.SetView("main", 0, infoHeight + 2, infoWidth + 1, infoHeight + 2 + 10)
        if err != nil && err != gocui.ErrUnknownView {
            return err
        }
        // fmt.Fprintf(mainView, "this is a view %v", os.Getpid())


        mainWidth, mainHeight := mainView.Size()
        _ = mainWidth
        _ = mainHeight
        keyView, err := gui.SetView("keys", infoWidth + 2, 0, infoWidth + 2 + 30, infoHeight + 1)
        if err != nil && err != gocui.ErrUnknownView {
            return err
        }

        keyView.Frame = true

        fmt.Fprintf(keyView, "Keys\n")
        fmt.Fprintf(keyView, "> or l: next track\n")
        fmt.Fprintf(keyView, "^ or k: skip 5 tracks ahead\n")
        fmt.Fprintf(keyView, "< or h: previous track\n")
        fmt.Fprintf(keyView, "v or j: skip 5 tracks back\n")
        fmt.Fprintf(keyView, "space: pause/unpause\n")
        fmt.Fprintf(keyView, "esc or ctrl-c: quit\n")

        viewUpdates := make(chan RenderState, 3)

        go func(){
            for quit.Err() == nil {
                select {
                    case state := <-viewUpdates:
                        gui.Update(func (gui *gocui.Gui) error {
                            mainView.Clear()
                            fmt.Fprintf(mainView, "Track %v / %v\n", state.track + 1, nsf.TotalSongs)
                            if !state.paused {
                                fmt.Fprintf(mainView, "Play time %v:%02d\n", state.playTime / 60, state.playTime % 60)
                            } else {
                                fmt.Fprintf(mainView, "Paused\n")
                            }
                            return nil
                        })
                    case <-quit.Done():
                }
            }
        }()

        go func(){
            renderState := RenderState{}
            timer := time.NewTicker(1 * time.Second)
            defer timer.Stop()
            for quit.Err() == nil {
                select {
                    case paused := <-pauseChannel:
                        renderState.paused = paused
                        viewUpdates <- renderState
                    case track := <-updateTrack:
                        renderState.paused = false
                        renderState.track = track
                        renderState.playTime = 0
                        viewUpdates <- renderState
                    case <-timer.C:
                        if !renderState.paused {
                            renderState.playTime += 1
                            viewUpdates <- renderState
                        }
                    case <-quit.Done():
                }
            }
        }()

        return nil
    })

    guiQuit := func(gui *gocui.Gui, view *gocui.View) error {
        return gocui.ErrQuit
    }

    for _, key := range []gocui.Key{gocui.KeyEsc, gocui.KeyCtrlC} {
        err = gui.SetKeybinding("", key, gocui.ModNone, guiQuit)
        if err != nil {
            log.Printf("Failed to bind esc in the gui: %v", err)
            return nil, err
        }
    }

    bindAction := func(key interface{}, action PlayerAction) error {
        return gui.SetKeybinding("", key, gocui.ModNone, func(gui *gocui.Gui, view *gocui.View) error {
            playerActions <- action
            return nil
        })
    }

    err = bindAction(gocui.KeyArrowLeft, PlayerPreviousTrack)
    if err != nil {
        return nil, err
    }

    err = bindAction('h', PlayerPreviousTrack)
    if err != nil {
        return nil, err
    }

    err = bindAction(gocui.KeyArrowDown, PlayerPrevious5Track)
    if err != nil {
        return nil, err
    }

    err = bindAction('j', PlayerPrevious5Track)
    if err != nil {
        return nil, err
    }

    err = bindAction(gocui.KeyArrowRight, PlayerNextTrack)
    if err != nil {
        return nil, err
    }

    err = bindAction('l', PlayerNextTrack)
    if err != nil {
        return nil, err
    }

    err = bindAction(gocui.KeyArrowUp, PlayerNext5Track)
    if err != nil {
        return nil, err
    }

    err = bindAction('k', PlayerNext5Track)
    if err != nil {
        return nil, err
    }

    err = bindAction(gocui.KeySpace, PlayerTogglePause)
    if err != nil {
        return nil, err
    }

    go func(){
        err := gui.MainLoop()
        if err != nil && err != gocui.ErrQuit {
            log.Printf("Error from gocui: %v", err)
        }

        cancel()
    }()

    return gui, nil
}

func run(nsfPath string) error {
    nsf, err := nes.LoadNSF(nsfPath)
    if err != nil {
        return err
    }

    _ = nsf

    err = sdl.Init(sdl.INIT_AUDIO)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    sampleRate := float32(44100)

    audioDevice, err := setupAudio(sampleRate)
    if err != nil {
        log.Printf("Warning: could not set up audio: %v", err)
        audioDevice = 0
    } else {
        defer sdl.CloseAudioDevice(audioDevice)
        log.Printf("Opened SDL audio device %v", audioDevice)
        sdl.PauseAudioDevice(audioDevice, false)
    }

    audioOut := make(chan []byte, 2)

    quit, cancel := context.WithCancel(context.Background())

    go func(){
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case audio := <-audioOut:
                    err := sdl.QueueAudio(audioDevice, audio)
                    if err != nil {
                        log.Printf("Error: could not queue audio data: %v", err)
                    }
            }
        }
    }()

    playerActions := make(chan PlayerAction)
    updateTrack := make(chan byte, 10)
    pauseChannel := make(chan bool)

    gui, err := terminalGui(quit, cancel, nsfPath, nsf, pauseChannel, updateTrack, playerActions)
    if err != nil {
        return err
    }

    defer gui.Close()

    track := byte(nsf.StartingSong - 1)

    updateTrack <- track

    runPlayer := func(track byte, actions chan nes.NSFActions) (context.Context, context.CancelFunc) {
        playQuit, playCancel := context.WithCancel(quit)
        go func(){
            err := nes.PlayNSF(nsf, track, audioOut, sampleRate, actions, playQuit)
            if err != nil {
                log.Printf("Unable to play: %v", err)
            }
        }()

        return playQuit, playCancel
    }

    nsfActions := make(chan nes.NSFActions)

    paused := false

    playQuit, playCancel := runPlayer(track, nsfActions)
    defer playCancel()
    for quit.Err() == nil {
        select {
            case action := <-playerActions:
                trackDelta := 0
                switch action {
                    case PlayerPreviousTrack:
                        trackDelta = -1
                    case PlayerPrevious5Track:
                        trackDelta = -5
                    case PlayerNextTrack:
                        trackDelta = 1
                    case PlayerNext5Track:
                        trackDelta = 5
                    case PlayerTogglePause:
                        paused = !paused
                        nsfActions <- nes.NSFActionTogglePause
                        pauseChannel <- paused
                }

                if trackDelta != 0 {
                    oldTrack := track
                    newTrack := int(track) + trackDelta
                    if newTrack < 0 {
                        newTrack = 0
                    }
                    if newTrack >= int(nsf.TotalSongs) {
                        newTrack = int(nsf.TotalSongs) - 1
                    }

                    track = byte(newTrack)

                    if oldTrack != track {
                        paused = false
                        playCancel()
                        playQuit, playCancel = runPlayer(track, nsfActions)
                        updateTrack <- track
                    }
                }
            case <-quit.Done():
        }
    }

    <-playQuit.Done()

    return nil
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

    var nesPath string

    if len(os.Args) == 1 {
        fmt.Printf("Give a .nsf file to play\n")
        return
    }

    nesPath = os.Args[1]
    err := run(nesPath)
    if err != nil {
        log.Printf("Error: %v", err)
    } else {
        log.Printf("Bye")
    }
}

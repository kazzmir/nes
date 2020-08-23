package main

import (
    "log"
    "fmt"
    "os"
    "io"
    "context"
    "time"
    "strings"
    "sync"
    "path/filepath"
    "strconv"
    "os/exec"
    "syscall"
    "bytes"
    "encoding/binary"

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

    audioOut := make(chan []float32, 2)

    quit, cancel := context.WithCancel(context.Background())

    go func(){
        var audioBuffer bytes.Buffer
        for quit.Err() == nil {
            select {
                case <-quit.Done():
                case audio := <-audioOut:
                    audioBuffer.Reset()
                    /* convert []float32 into []byte */
                    for _, sample := range audio {
                        binary.Write(&audioBuffer, binary.LittleEndian, sample)
                    }

                    err := sdl.QueueAudio(audioDevice, audioBuffer.Bytes())
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

func help(){
    fmt.Println("nsf [-mp3 <path> <track> <time>] <nsf file>")
    fmt.Println()
    fmt.Println("With no other arguments, launch the terminal app that plays the given <nsf file>")
    fmt.Println("With -mp3 <path> <track> <time> the <nsf file> will be transcoded to an mp3 file (needs ffmpeg installed)")
    fmt.Println("  The <time> argument refers to the amount of time to play the track for, and can be")
    fmt.Println("  either a plain number or can be suffixed with s for seconds or m for minutes. Without a")
    fmt.Println("  suffix the <time> is interpreted as seconds.")
    fmt.Println()
    fmt.Println("Jon Rafkind <jon@rafkind.com>")
}

func isNumber(value string) bool {
    _, err := strconv.Atoi(value)
    return err == nil
}

func convertTime(data string) (uint64, error) {
    if len(data) == 0 {
        return 0, fmt.Errorf("No time given")
    }

    last := strings.ToLower(data[len(data)-1:])
    hasSuffix := !isNumber(last)

    end := len(data)

    multiple := 1
    if hasSuffix {
        switch last {
            case "s":
            case "m": multiple = 60
            default:
                return 0, fmt.Errorf("Unknown suffix '%v'", last)
        }

        end = len(data)-1
    }

    rest := data[0:end]
    number, err := strconv.ParseUint(rest, 10, 64)
    if err != nil {
        return 0, err
    }

    return number * uint64(multiple), nil
}

/* FIXME: share this between the nes program */
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

func saveMp3(nsfPath string, mp3out string, track int, renderTime uint64) error {
    nsf, err := nes.LoadNSF(nsfPath)
    if err != nil {
        return err
    }

    if track < 0 || track >= int(nsf.TotalSongs) {
        return fmt.Errorf("Invalid track %v. Must be between 1 and %v", track+1, nsf.TotalSongs+1)
    }

    sampleRate := float32(44100)
    audioOut := make(chan []float32, 2)
    actions := make(chan nes.NSFActions)

    quit, cancel := context.WithCancel(context.Background())
    defer cancel()

    var waiter sync.WaitGroup

    go func(){
        time.Sleep(time.Duration(renderTime) * time.Second)
        log.Printf("Done")
        cancel()
    }()

    waiter.Add(1)
    go func(){
        ffmpeg_binary_path, err := findFfmpegBinary()
        if err != nil {
            log.Printf("Could not find ffmpeg: %v", err)
            return
        }

        audio_reader, audio_writer, err := os.Pipe()
        if err != nil {
            return
        }

        log.Printf("Launching ffmpeg")
        ffmpeg_process := exec.Command(ffmpeg_binary_path,
            "-use_wallclock_as_timestamps", "1", // treat the incoming data as a live stream
            "-f", "f32le", // audio is uncompressed pcm in float32 format
            "-ar", strconv.Itoa(int(sampleRate)), // sample rate
            "-ac", "1",
            "-i", "pipe:3", // audio is passed as fd 3

            "-tune", "zerolatency", // fast encoding
            "-acodec", "mp3", // mp3 for audio
            // "-filter:a", "volume=10dB", // increase volume a bit
            "-y", // overwrite output if the file already exists
            mp3out)

        // ffmpeg_process.Stdin = reader

        /* FIXME: figure out a solution for windows */
        ffmpeg_process.ExtraFiles = []*os.File{audio_reader}

        stdout, err := ffmpeg_process.StdoutPipe()
        if err != nil {
            log.Printf("Could not get ffmpeg stdout: %v", err)
            return
        }

        stderr, err := ffmpeg_process.StderrPipe()
        if err != nil {
            log.Printf("Could not get ffmpeg stderr: %v", err)
            return
        }

        err = ffmpeg_process.Start()
        if err != nil {
            log.Printf("Could not start ffmpeg: %v", err)
            return
        }

        go func(stdout io.ReadCloser){
            buffer := make([]byte, 4096)
            for {
                count, err := stdout.Read(buffer)
                if err != nil {
                    log.Printf("Could not read ffmpeg stdout: %v", err)
                    return
                }
                _ = count
                // log.Printf("ffmpeg: %v", string(buffer[0:count]))
            }
        }(stdout)

        go func(stdout io.ReadCloser){
            buffer := make([]byte, 4096)
            for {
                count, err := stdout.Read(buffer)
                if err != nil {
                    log.Printf("Could not read ffmpeg stdout: %v", err)
                    return
                }
                _ = count
                // log.Printf("ffmpeg: %v", string(buffer[0:count]))
            }
        }(stderr)

        log.Printf("Recording to %v", mp3out)

        go func(){
            defer waiter.Done()
            startTime := time.Now()
            <-quit.Done()
            /* ffmpeg will normally close on its own if its input is closed */
            audio_writer.Close()
            /* we can send SIGINT to it as well, which also usually stops it */
            ffmpeg_process.Process.Signal(os.Interrupt)
            waitForProcess(ffmpeg_process.Process, 10)
            log.Printf("Recording has ended. Saved '%v' for %v size %v", mp3out, time.Now().Sub(startTime), niceSize(mp3out))
        }()

        go func(){
            defer audio_reader.Close()

            var audioBuffer bytes.Buffer

            for {
                select {
                    case <-quit.Done():
                        return
                    case audio := <-audioOut:
                        audioBuffer.Reset()
                        /* convert []float32 into []byte */
                        for _, sample := range audio {
                            binary.Write(&audioBuffer, binary.LittleEndian, sample)
                        }
                        // log.Printf("Enqueue audio")
                        audio_writer.Write(audioBuffer.Bytes())
                }
            }
        }()
    }()

    log.Printf("Rendering track %v of %v to '%v' for %d:%02d", track+1, filepath.Base(nsfPath), mp3out, renderTime/60, renderTime % 60)

    err = nes.PlayNSF(nsf, byte(track), audioOut, sampleRate, actions, quit)

    waiter.Wait()

    return err
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

    var nesPath string

    if len(os.Args) == 1 {
        fmt.Printf("Give a .nsf file to play\n\n")
        help()
        return
    }

    var mp3Out string
    var mp3track int
    var mp3time uint64

    for i := 1; i < len(os.Args); i++ {
        switch os.Args[i] {
            case "-mp3":
                i = i + 1
                if i < len(os.Args) {
                    mp3Out = os.Args[i]
                    i += 1
                    if i < len(os.Args) {
                        var err error
                        mp3track, err = strconv.Atoi(os.Args[i])
                        if err != nil {
                            fmt.Printf("Error: %v\n", err)
                            return
                        }

                        i += 1
                        if i < len(os.Args) {
                            mp3time, err = convertTime(os.Args[i])
                            if err != nil {
                                fmt.Printf("Error: %v\n", err)
                                return
                            }
                        } else {
                            fmt.Printf("-mp3 needs a <time> argument\n")
                            return
                        }
                    } else {
                        fmt.Printf("-mp3 needs a <track> argument\n")
                        return
                    }
                } else {
                    fmt.Printf("-mp3 needs three more arguments\n")
                    help()
                    return
                }
            case "-h", "--help":
                help()
                return
            default:
                nesPath = os.Args[i]
        }
    }

    if mp3Out != "" {
        err := saveMp3(nesPath, mp3Out, mp3track - 1, mp3time)
        if err != nil {
            log.Printf("Error: %v", err)
        }
    } else {
        err := run(nesPath)
        if err != nil {
            log.Printf("Error: %v", err)
        } else {
            log.Printf("Bye")
        }
    }
}

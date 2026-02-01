//go:build !windows && !js

package util

import (
    "os/exec"
    "os"
    "io"
    "log"
    "fmt"
    "time"
    "bytes"
    "syscall"
    "context"
    "strconv"

    nes "github.com/kazzmir/nes/lib"
)

func FindFfmpegBinary() (string, error) {
    return exec.LookPath("ffmpeg")
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

type SignalTimeout struct {
    Signal syscall.Signal
    Timeout int
}

func processExists(process *os.Process) bool {
    // on linux sending signal 0 will have no impact, but will fail
    // if the process doesn't exist (or we don't own it)
    return process.Signal(syscall.Signal(0)) == nil
}

func isAlive(process *os.Process) bool {
    if !processExists(process) {
        return false
    }

    var status syscall.WaitStatus
    pid, err := syscall.Wait4(process.Pid, &status, syscall.WNOHANG, nil)
    if err != nil {
        log.Printf("Unable to call wait4 on pid %v: %v", process.Pid, err)
        return false
    }
    if pid != process.Pid {
        return true
    }
    return !status.Exited()
}

/* send a series of signals to a process hoping to kill it. if none of the signals
 * manage to kill the process then use SIGKILL ultimately.
 */
func waitForProcess(process *os.Process, signals []SignalTimeout){
    dead := isAlive(process)
    for _, use := range signals {
        if dead {
            break
        }
        process.Signal(use.Signal) // send signal to process, hoping to kill it
        log.Printf("Sent signal %v to pid %v", process.Pid, use.Signal)
        done := time.Now().Add(time.Second * time.Duration(use.Timeout))
        // wait for the process to go away
        for time.Now().Before(done) {
            if isAlive(process){
                time.Sleep(time.Millisecond * 100)
            } else {
                dead = true
                break
            }
        }
    }
    if !dead {
        /* Didn't die on its own, so we forcifully kill it */
        log.Printf("Killing pid %v", process.Pid)
        process.Kill()
    }
    process.Wait()
}

func waitForProcessDefault(process *os.Process){
    waitForProcess(process, []SignalTimeout{
        SignalTimeout{
            Signal: syscall.SIGINT,
            Timeout: 2,
        },
        SignalTimeout{
            Signal: syscall.SIGTERM,
            Timeout: 3,
        },
    })
}

/* Audio */
func EncodeMp3(mp3out string, mainQuit context.Context, sampleRate int, audio_input io.Reader) error {
    ffmpeg_binary_path, err := FindFfmpegBinary()
    if err != nil {
        return fmt.Errorf("Could not find ffmpeg: %v", err)
    }

    audio_reader, audio_writer, err := os.Pipe()
    if err != nil {
        return err
    }

    log.Printf("Launching ffmpeg")
    ffmpeg_process := exec.Command(ffmpeg_binary_path,
    "-use_wallclock_as_timestamps", "1", // treat the incoming data as a live stream
    "-f", "f32le", // audio is uncompressed pcm in float32 format
    "-ar", strconv.Itoa(int(sampleRate)), // sample rate
    "-ac", "2",
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
        return err
    }

    stderr, err := ffmpeg_process.StderrPipe()
    if err != nil {
        log.Printf("Could not get ffmpeg stderr: %v", err)
        return err
    }

    err = ffmpeg_process.Start()
    if err != nil {
        log.Printf("Could not start ffmpeg: %v", err)
        return err
    }

    go io.Copy(io.Discard, stdout)
    go io.Copy(io.Discard, stderr)

    log.Printf("Recording to %v", mp3out)

    quit, cancel := context.WithCancel(mainQuit)
    _ = cancel

    go func(){
        startTime := time.Now()
        <-quit.Done()
        /* ffmpeg will normally close on its own if its input is closed */
        audio_writer.Close()
        waitForProcessDefault(ffmpeg_process.Process)
        log.Printf("Recording has ended. Saved '%v' for %v size %v", mp3out, time.Since(startTime), niceSize(mp3out))
    }()

    go func(){
        defer audio_reader.Close()
        io.Copy(audio_writer, audio_input)
        /*
        for quit.Err() == nil {
            n, _ := io.CopyN(audio_writer, audio_input, int64(sampleRate * 4 * 2)) // 1 second worth of audio
            log.Printf("Wrote %v bytes of audio data to ffmpeg", n)
        }
        */
    }()

    <-quit.Done()

    return nil
}

func videoWriter(out io.Writer, overscanPixels int, video_channel chan nes.VirtualScreen, stop context.Context){
    var output bytes.Buffer
    for {
        select {
            case <-stop.Done():
                return
            case buffer := <-video_channel:
                output.Reset()
                for y := overscanPixels; y < buffer.Height - overscanPixels; y++ {
                    for x := 0; x < buffer.Width; x++ {
                        r, g, b, _ := buffer.GetRGBA(x, y)
                        output.Write([]byte{r, g, b})
                    }
                }

                out.Write(output.Bytes())
            }
    }
}

/* Audio+Video */
func RecordMp4(mainQuit context.Context, mp4Path string, overscanPixels int, sampleRate int, video_channel chan nes.VirtualScreen, audio_input io.Reader) error {
    ffmpeg_binary_path, err := FindFfmpegBinary()

    if err != nil {
        return err
    }

    video_reader, video_writer, err := os.Pipe()
    if err != nil {
        return err
    }

    audio_reader, audio_writer, err := os.Pipe()
    if err != nil {
        return err
    }

    scaleFactor := 3

    log.Printf("Launching ffmpeg")
    ffmpeg_process := exec.Command(ffmpeg_binary_path,
        "-use_wallclock_as_timestamps", "1", // treat the incoming data as a live stream
        "-f", "rawvideo", // video is raw pixels, as opposed to compressed like jpg/png
        "-pixel_format", "rgb24", // 3 bytes per pixel, 1 byte per color
        "-video_size", "256x224", // size of the nes screen
        "-i", "pipe:3", // video is passed as fd 3
        "-f", "f32le", // audio is uncompressed pcm in float32 format
        "-ar", strconv.Itoa(sampleRate), // sample rate
        "-ac", "2", // 2 channel stereo
        "-i", "pipe:4", // audio is passed as fd 4
        "-vf", fmt.Sprintf("scale=iw*%v:ih*%v", scaleFactor, scaleFactor), // upscale the video
        "-vsync", "vfr", // allow for variable frame rate video
        "-r", "60", // maximum of 60fps

        "-movflags", "empty_moov", // write an empty moov frame at the start
        "-frag_duration", "100000", // write moov atoms every 100ms
        "-profile:v", "main", // h264 main profile
        "-pix_fmt", "yuv420p", // default is yuv444 that some decoders can't handle

        "-tune", "zerolatency", // fast encoding
        "-acodec", "mp3", // mp3 for audio. this is arbitrary but aac sounds bad
        // "-filter:a", "volume=10dB", // increase volume a bit
        "-y", // overwrite output if the file already exists
        mp4Path)

    // ffmpeg_process.Stdin = reader

    /* FIXME: figure out a solution for windows */
    ffmpeg_process.ExtraFiles = []*os.File{video_reader, audio_reader}

    stdout, err := ffmpeg_process.StdoutPipe()
    if err != nil {
        log.Printf("Could not get ffmpeg stdout: %v", err)
        return err
    }

    stderr, err := ffmpeg_process.StderrPipe()
    if err != nil {
        log.Printf("Could not get ffmpeg stderr: %v", err)
        return err
    }

    err = ffmpeg_process.Start()
    if err != nil {
        return err
    }

    go io.Copy(io.Discard, stdout)
    go io.Copy(io.Discard, stderr)

    log.Printf("Recording to %v", mp4Path)

    stop, cancel := context.WithCancel(mainQuit)
    _ = cancel

    go func(){
        startTime := time.Now()
        <-stop.Done()
        /* ffmpeg will normally close on its own if its input is closed */
        video_writer.Close()
        audio_writer.Close()
        waitForProcessDefault(ffmpeg_process.Process)
        log.Printf("Recording has ended. Saved '%v', length=%v size=%v", mp4Path, time.Since(startTime), niceSize(mp4Path))
    }()

    go func(){
        defer audio_reader.Close()
        io.Copy(audio_writer, audio_input)
    }()

    /* video reader */
    go func(){
        defer video_reader.Close()
        videoWriter(video_writer, overscanPixels, video_channel, stop)
    }()

    <-stop.Done()

    return nil
}

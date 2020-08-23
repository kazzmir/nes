package util

import (
    "os/exec"
    "os"
    "io"
    "log"
    "fmt"
    "time"
    "bytes"
    "encoding/binary"
    "syscall"
    "context"
    "strconv"
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

func EncodeMp3(mp3out string, mainQuit context.Context, sampleRate int, audioOut chan []float32) error {
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

    quit, cancel := context.WithCancel(mainQuit)
    _ = cancel

    go func(){
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

    <-quit.Done()

    return nil
}

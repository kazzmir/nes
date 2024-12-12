package util

import (
    "errors"
    "context"

    nes "github.com/kazzmir/nes/lib"
)

var UnsupportedError = errors.New("Unsupported")

func RecordMp4(mainQuit context.Context, mp4Path string, overscanPixels int, sampleRate int, video_channel chan nes.VirtualScreen, audio_channel chan []float32) error {
    return UnsupportedError
}

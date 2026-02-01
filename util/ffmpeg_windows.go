//go:build windows && !js
package util

import (
    "errors"
    "context"

    nes "github.com/kazzmir/nes/lib"
)

var UnsupportedError = errors.New("Unsupported")

func RecordMp4(mainQuit context.Context, mp4Path string, overscanPixels int, sampleRate int, video_channel chan nes.VirtualScreen, audio_channel *nes.AudioStream) error {
    return UnsupportedError
}

func EncodeMp3(mp3out string, mainQuit context.Context, sampleRate int, audioOut chan []float32) error {
    return UnsupportedError
}

package data

import (
    "embed"
    "io/fs"
)

//go:embed data/*
var dataFS embed.FS

func OpenFile(path string) (fs.File, error) {
    return dataFS.Open("data/" + path)
}

//go:embed roms/*
var RomsFS embed.FS

//go:embed screenshots/*
var ScreenshotFS embed.FS

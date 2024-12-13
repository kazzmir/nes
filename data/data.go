package data

import (
    "embed"
    "io"
    "path/filepath"
)

//go:embed data/*
var fs embed.FS

func OpenFile(path string) (io.Reader, error) {
    return fs.Open(filepath.Join("data", path))
}

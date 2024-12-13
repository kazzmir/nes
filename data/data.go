package data

import (
    "embed"
    "io/fs"
    "path/filepath"
)

//go:embed data/*
var dataFS embed.FS

func OpenFile(path string) (fs.File, error) {
    return dataFS.Open(filepath.Join("data", path))
}

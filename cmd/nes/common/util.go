package common

import (
    "os"
)

func FileExists(path string) bool {
    info, err := os.Stat(path)
    if os.IsNotExist(err) {
        // file does not exists return false
        return false
    }

    // return true if exist and is not a directory
    return !info.IsDir()
}


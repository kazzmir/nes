package common

import (
    "os"
    "io"
    "fmt"
    "crypto/sha256"
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

/* return the sha256 hash of a file given by the path */
func GetSha256(path string) (string, error){
    hash := sha256.New()
    data, err := os.Open(path)
    if err != nil {
        return "", err
    }
    _, err = io.Copy(hash, data)
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("%x", hash.Sum(nil)), nil
}


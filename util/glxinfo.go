package util

import (
    "os/exec"
)

func HasGlxinfo() bool {
    glxinfo_path, err := exec.LookPath("glxinfo")
    if err != nil {
        return true
    }
    glxinfo := exec.Command(glxinfo_path)
    err = glxinfo.Run()
    return err == nil
}

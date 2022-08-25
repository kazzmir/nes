package debug

import (
    nes "github.com/kazzmir/nes/lib"
)

type Debugger interface {
    Handle(*nes.CPUState)
}

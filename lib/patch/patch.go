package patch

import (
    "bytes"
    "errors"
)

var ErrUnsupportedPatchFormat = errors.New("unsupported patch format")

func ApplyPatch(nesData []byte, patchData []byte) ([]byte, error) {

    ips := []byte("PATCH")
    bps := []byte("BPS1")

    if len(patchData) >= len(ips) && bytes.Equal(patchData[:len(ips)], ips) {
        return ApplyIPSPatch(nesData, patchData)
    }

    if len(patchData) >= len(bps) && bytes.Equal(patchData[:len(bps)], bps) {
        return ApplyBPSPatch(nesData, patchData)
    }

    return nil, ErrUnsupportedPatchFormat
}

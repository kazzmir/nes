package patch

import (
    "bytes"
    "errors"
)

var ErrInvalidPatch = errors.New("invalid patch file")

// Apply an IPS patch file to a ROM

func bigEndian(b []byte) uint32 {
    var result uint32 = 0
    for _, v := range b {
        result = (result << 8) | uint32(v)
    }
    return result
}

func readUint16(r *bytes.Reader) (uint16, error) {
    b1, err := r.ReadByte()
    if err != nil {
        return 0, err
    }
    b2, err := r.ReadByte()
    if err != nil {
        return 0, err
    }
    return uint16(bigEndian([]byte{b1, b2})), nil
}

func ApplyIPSPatch(rom []byte, patch []byte) ([]byte, error) {
    // first 5 bytes should be PATCH

    byteReader := bytes.NewReader(patch)

    data := make([]byte, 5)
    _, err := byteReader.Read(data)
    if err != nil {
        return nil, err
    }

    if !bytes.Equal(data, []byte("PATCH")) {
        return nil, ErrInvalidPatch
    }

    offsetBytes := make([]byte, 3)
    sizeBytes := make([]byte, 2)

    newBytes := make([]byte, len(rom))
    copy(newBytes, rom)

    for {
        n, err := byteReader.Read(offsetBytes)
        if err != nil {
            return nil, err
        }
        if n != 3 {
            return nil, ErrInvalidPatch
        }

        if bytes.Equal(offsetBytes, []byte("EOF")) {
            newSize := make([]byte, 3)
            n, err = byteReader.Read(newSize)
            if err == nil && n == 3 {
                newSizeInt := bigEndian(newSize)
                if newSizeInt > uint32(len(newBytes)) {
                    newBytes = append(newBytes, make([]byte, newSizeInt - uint32(len(newBytes)))...)
                } else {
                    newBytes = newBytes[:newSizeInt]
                }
            }

            return newBytes, nil
        }

        n, err = byteReader.Read(sizeBytes)
        if err != nil {
            return nil, err
        }

        if n != 2 {
            return nil, ErrInvalidPatch
        }

        offset := bigEndian(offsetBytes)
        size := bigEndian(sizeBytes)

        if size > 0 {
            for i := range size {
                next, err := byteReader.ReadByte()
                if err != nil {
                    return nil, err
                }
                if i + offset >= uint32(len(newBytes)) {
                    return nil, ErrInvalidPatch
                }
                newBytes[i + offset] = next
            }
        } else {
            // RLE encoded
            runLength, err := readUint16(byteReader)
            if err != nil {
                return nil, err
            }
            value, err := byteReader.ReadByte()
            if err != nil {
                return nil, err
            }

            for i := range uint32(runLength) {
                if i + offset >= uint32(len(newBytes)) {
                    return nil, ErrInvalidPatch
                }

                newBytes[i + offset] = value
            }
        }
    }
}

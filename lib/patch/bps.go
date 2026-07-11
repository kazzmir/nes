package patch

// https://www.romhacking.net/documents/746/

import (
    "bytes"
    "io"
    "errors"
)

var ErrSourceCRC = errors.New("source CRC mismatch")
var ErrTargetCRC = errors.New("target CRC mismatch")
var ErrPatchCRC = errors.New("patch CRC mismatch")

func littleEndian(b []byte) uint32 {
    return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func readUint32(r *bytes.Reader) (uint32, error) {
    d := make([]byte, 4)
    _, err := io.ReadFull(r, d)
    if err != nil {
        return 0, err
    }

    return littleEndian(d), nil
}

func readBPSVarInt(r *bytes.Reader) (uint64, error) {
    value := uint64(0)
    shift := uint64(1)

    for {
        x, err := r.ReadByte()
        if err != nil {
            return 0, err
        }

        value += uint64(x&0x7F) * shift
        if x&0x80 > 0 {
            break
        }

        shift <<= 7
        value += shift
    }

    return value, nil
}

func computeCRC32(data []byte) uint32 {
    crc := uint32(0xFFFFFFFF)

    for _, b := range data {
        crc ^= uint32(b)
        for range 8 {
            if crc&1 > 0 {
                crc = (crc >> 1) ^ 0xEDB88320
            } else {
                crc >>= 1
            }
        }
    }

    return ^crc
}

func ApplyBPSPatch(nesData []byte, patchData []byte) ([]byte, error) {

    bps := []byte("BPS1")

    if len(patchData) < 12 {
        return nil, ErrInvalidPatch
    }

    // skip CRC section
    reader := bytes.NewReader(patchData[:len(patchData) - 12])

    data := make([]byte, 4)
    n, err := reader.Read(data)
    if err != nil {
        return nil, err
    }

    if n != len(bps) {
        return nil, ErrInvalidPatch
    }

    sourceSize, err := readBPSVarInt(reader)
    if err != nil {
        return nil, err
    }

    if sourceSize != uint64(len(nesData)) {
        return nil, ErrInvalidPatch
    }

    targetSize, err := readBPSVarInt(reader)
    if err != nil {
        return nil, err
    }

    target := make([]byte, targetSize)
    sourcePointer := uint64(0)
    targetPointer := uint64(0)

    metadataSize, err := readBPSVarInt(reader)
    if err != nil {
        return nil, err
    }
    
    if metadataSize > 0 {
        metadata := make([]byte, metadataSize)
        n, err = io.ReadFull(reader, metadata)
        if err != nil {
            return nil, err
        }

        if n != int(metadataSize) {
            return nil, ErrInvalidPatch
        }
        // Process metadata if needed
    }

    sourceOffset := int64(0)
    targetOffset := int64(0)

    for {
        action, err := readBPSVarInt(reader)
        if err != nil {
            if errors.Is(err, io.EOF) {
                break
            }
            return nil, err
        }

        command := action & 3
        length := (action >> 2) + 1

        switch command {
            // source read
            case 0:
                if targetPointer + length > uint64(len(target)) {
                    return nil, ErrInvalidPatch
                }

                if sourcePointer + length > uint64(len(nesData)) {
                    return nil, ErrInvalidPatch
                }

                copy(target[targetPointer:targetPointer+length], nesData[sourcePointer:sourcePointer+length])

                targetPointer += length
                sourcePointer += length
            // target read
            case 1:

                if targetPointer + length > uint64(len(target)) {
                    return nil, ErrInvalidPatch
                }

                for range length {
                    value, err := reader.ReadByte()
                    if err != nil {
                        return nil, err
                    }

                    target[targetPointer] = value
                    targetPointer += 1
                }

            // source copy
            case 2:
                data, err := readBPSVarInt(reader)
                if err != nil {
                    return nil, err
                }

                value := int64(data >> 1)
                // odd values indicate negative offsets
                if data & 1 == 1 {
                    value = -value
                }

                sourceOffset += value

                if sourceOffset < 0 || sourceOffset + int64(length) > int64(len(nesData)) {
                    return nil, ErrInvalidPatch
                }

                if targetPointer + length > uint64(len(target)) {
                    return nil, ErrInvalidPatch
                }

                copy(target[targetPointer:targetPointer+length], nesData[sourceOffset:sourceOffset+int64(length)])
                
                targetPointer += length
                sourceOffset += int64(length)

            // target copy
            case 3:
                data, err := readBPSVarInt(reader)
                if err != nil {
                    return nil, err
                }

                value := int64(data >> 1)
                // odd values indicate negative offsets
                if data & 1 == 1 {
                    value = -value
                }

                targetOffset += value
                
                if targetOffset < 0 || targetOffset + int64(length) > int64(len(target)) {
                    return nil, ErrInvalidPatch
                }

                if targetPointer + length > uint64(len(target)) {
                    return nil, ErrInvalidPatch
                }

                for range length {
                    target[targetPointer] = target[targetOffset]
                    targetPointer += 1
                    targetOffset += 1
                }
        }
    }

    crcBytes := make([]byte, 12)
    copy(crcBytes, patchData[len(patchData)-12:])

    crcReader := bytes.NewReader(crcBytes)

    sourceCRC, err := readUint32(crcReader)
    if err != nil {
        return nil, err
    }

    targetCRC, err := readUint32(crcReader)
    if err != nil {
        return nil, err
    }

    patchCRC, err := readUint32(crcReader)
    if err != nil {
        return nil, err
    }

    if computeCRC32(nesData) != sourceCRC {
        return nil, ErrSourceCRC
    }

    if computeCRC32(target) != targetCRC {
        return nil, ErrTargetCRC
    }

    if computeCRC32(patchData[:len(patchData)-4]) != patchCRC {
        return nil, ErrPatchCRC
    }

    return target, nil
}

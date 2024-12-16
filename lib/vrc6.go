package lib

// https://www.nesdev.org/wiki/VRC6_audio

import (
    "log"
)

// memory addresses to control the VRC6 audio chip
const VRC6FrequencyControl = 0x9003
const VRC6Pulse1Control = 0x9000
const VRC6Pulse1FrequencyLow = 0x9001
const VRC6Pulse1FrequencyHigh = 0x9002

const VRC6Pulse2Control = 0xA000
const VRC6Pulse2FrequencyLow = 0xA001
const VRC6Pulse2FrequencyHigh = 0xA002

const VRC6SawVolume = 0xb000
const VRC6SawFrequencyLow = 0xB001
const VRC6SawFrequencyHigh = 0xB002

type VRC6Audio struct {
    Pulse1 VRC6Pulse
    Pulse2 VRC6Pulse
    Saw VRC6Saw
}

// returns true if the address is a VRC6 audio address
func (vrc6 *VRC6Audio) HandleWrite(address uint16, value uint8) bool {
    switch address {
        case VRC6FrequencyControl:
            log.Printf("vrc6 frequency control: %x\n", value)

            halt := (value & 0x1) == 0x1
            x16 := (value & 0x2) == 0x2
            x256 := (value & 0x4) == 0x4

            vrc6.FrequencyControl(halt, x16, x256)

            return true
        case VRC6Pulse1Control:
            log.Printf("vrc6 pulse1 control: %x\n", value)

            volume := int(value & 0xf)
            duty := int((value >> 4) & 0x7)
            mode := int((value >> 7) & 0x1)

            vrc6.Pulse1Control(volume, duty, mode)

            return true
        case VRC6Pulse1FrequencyLow:
            log.Printf("vrc6 pulse1 frequency low: %x\n", value)

            frequency := int(value)
            vrc6.Pulse1FrequencyLow(frequency)

            return true
        case VRC6Pulse1FrequencyHigh:
            log.Printf("vrc6 pulse1 frequency high: %x\n", value)

            frequency := int(value & 0xf)
            enable := ((value >> 7) & 0x1) == 0x1

            vrc6.Pulse1FrequencyHigh(frequency)
            vrc6.Pulse1SetEnable(enable)

            return true

        case VRC6Pulse2Control:
            log.Printf("vrc6 pulse2 control: %x\n", value)

            volume := int(value & 0xf)
            duty := int((value >> 4) & 0x7)
            mode := int((value >> 7) & 0x1)

            vrc6.Pulse2Control(volume, duty, mode)

            return true
        case VRC6Pulse2FrequencyLow:
            log.Printf("vrc6 pulse2 frequency low: %x\n", value)

            frequency := int(value)
            vrc6.Pulse2FrequencyLow(frequency)

            return true
        case VRC6Pulse2FrequencyHigh:
            log.Printf("vrc6 pulse2 frequency high: %x\n", value)

            frequency := int(value & 0xf)
            enable := ((value >> 7) & 0x1) == 0x1

            vrc6.Pulse2FrequencyHigh(frequency)
            vrc6.Pulse2SetEnable(enable)

            return true

        case VRC6SawVolume:
            log.Printf("vrc6 saw volume: %x\n", value)

            mask := uint8((1 << 6) - 1)
            volume := int(value & mask)
            vrc6.SawVolume(volume)

            return true
        case VRC6SawFrequencyLow:
            log.Printf("vrc6 saw frequency low: %x\n", value)

            frequency := int(value)
            vrc6.SawFrequencyLow(frequency)

            return true
        case VRC6SawFrequencyHigh:
            log.Printf("vrc6 saw frequency high: %x\n", value)

            frequency := int(value & 0xf)
            enable := ((value >> 7) & 0x1) == 0x1

            vrc6.SawFrequencyHigh(frequency)
            vrc6.SawSetEnable(enable)

            return true
    }

    return false
}

func (vrc6 *VRC6Audio) FrequencyControl(halt bool, x16 bool, x256 bool) {
    // TODO
}

func (vrc6 *VRC6Audio) Pulse1Control(volume int, duty int, mode int) {
    // TODO
}

func (vrc6 *VRC6Audio) Pulse2Control(volume int, duty int, mode int) {
    // TODO
}

func (vrc6 *VRC6Audio) Pulse1FrequencyLow(frequency int) {
    // TODO
}

func (vrc6 *VRC6Audio) Pulse1FrequencyHigh(frequency int) {
    // TODO
}

func (vrc6 *VRC6Audio) Pulse1SetEnable(enable bool) {
    // TODO
}   

func (vrc6 *VRC6Audio) Pulse2FrequencyLow(frequency int) {
    // TODO
}

func (vrc6 *VRC6Audio) Pulse2FrequencyHigh(frequency int) {
    // TODO
}

func (vrc6 *VRC6Audio) Pulse2SetEnable(enable bool) {
    // TODO
}   

func (vrc6 *VRC6Audio) SawFrequencyLow(frequency int) {
    // TODO
}

func (vrc6 *VRC6Audio) SawFrequencyHigh(frequency int) {
    // TODO
}

func (vrc6 *VRC6Audio) SawVolume(volume int) {
    // TODO
}

func (vrc6 *VRC6Audio) SawSetEnable(enable bool) {
    // TODO
}

type VRC6Pulse struct {
}

type VRC6Saw struct {
}

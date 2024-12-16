package lib

// https://www.nesdev.org/wiki/VRC6_audio

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

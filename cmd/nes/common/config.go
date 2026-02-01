package common

import (
    "os"
    "log"
    "encoding/json"
    "path/filepath"
)

const CurrentVersion = 2

type ConfigJoystickData struct {
    A string `json:"a,omitempty"`
    B string `json:"b,omitempty"`
    TurboA string `json:"turbo-a,omitempty"`
    TurboB string `json:"turbo-b,omitempty"`
    Select string `json:"select,omitempty"`
    Start string `json:"start,omitempty"`
    Up string `json:"up,omitempty"`
    Down string `json:"down,omitempty"`
    Left string `json:"left,omitempty"`
    Right string `json:"right,omitempty"`
    Guid string `json:"guid,omitempty"`
    Name string `json:"name,omitempty"`
}

type ConfigKeys struct {
    Turbo string
    Pause string
    HardReset string
    PPUDebug string
    SlowDown string
    SpeedUp string
    Normal string
    StepFrame string
    Record string
    SaveState string
    LoadState string
    Console string

    ButtonA string
    ButtonB string
    ButtonTurboA string
    ButtonTurboB string
    ButtonSelect string
    ButtonStart string
    ButtonUp string
    ButtonDown string
    ButtonLeft string
    ButtonRight string
}

type ConfigData struct {
    Version int `json:"version,omitempty"`
    Player1Joystick ConfigJoystickData `json:"player1-joystick,omitempty"`
    Player1Keys ConfigKeys `json:"player1-keys,omitempty"`
}

/* make the directory where the config file lives, which is ~/.config/jon-nes on linux */
func GetOrCreateConfigDir() (string, error) {
    configDir, err := os.UserConfigDir()
    if err != nil {
        return "", err
    }
    configPath := filepath.Join(configDir, "jon-nes")
    err = os.MkdirAll(configPath, 0755)
    if err != nil {
        return "", err
    }

    return configPath, nil
}

func DefaultConfigData() ConfigData {
    return ConfigData{
        Version: CurrentVersion,
    }
}

func LoadConfigData() (ConfigData, error) {
    configPath, err := GetOrCreateConfigDir()
    if err != nil {
        return DefaultConfigData(), err
    }
    config := filepath.Join(configPath, "config.json")
    file, err := os.Open(config)
    if err != nil {
        return DefaultConfigData(), err
    }
    defer file.Close()

    var data ConfigData
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&data)
    if err != nil {
        log.Printf("Could not load config data: %v", err)
        return DefaultConfigData(), err
    }

    if data.Version != CurrentVersion {
        return DefaultConfigData(), nil
    }

    return data, nil
}

/* create the config.json file in the config dir and return the opened file, or an error */
func SaveConfigData(data ConfigData) error {
    configPath, err := GetOrCreateConfigDir()
    if err != nil {
        return err
    }
    config := filepath.Join(configPath, "config.json")

    file, err := os.Create(config)
    if err != nil {
        return err
    }
    defer file.Close()

    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")
    return encoder.Encode(data)
}

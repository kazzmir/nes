package main

import (
    "io/fs"
    "math/rand/v2"
    "image/png"

    "github.com/kazzmir/nes/data"

    "github.com/hajimehoshi/ebiten/v2"
)

const MAX_LIFE = 300
const FADE_MARGIN = 10

type ScreenshotsBackground struct {
    Files []string
    Images map[string]*ebiten.Image

    Life int
    Current string
    X float64
    Y float64
    X_dir float64
    Y_dir float64
}

func MakeScreenshotsBackground() *ScreenshotsBackground {
    entries, err := fs.ReadDir(data.ScreenshotFS, "screenshots")

    var files []string

    if err == nil {
        for _, entry := range entries {
            if entry.IsDir() {
                continue
            }

            files = append(files, entry.Name())
        }
    }

    return &ScreenshotsBackground{
        Files:  files,
        Images: make(map[string]*ebiten.Image),
    }
}

func (background *ScreenshotsBackground) GetImage() *ebiten.Image {
    img, ok := background.Images[background.Current]
    if ok {
        return img
    }

    file, err := data.ScreenshotFS.Open("screenshots/" + background.Current)
    if err == nil {
        defer file.Close()
        decoded, err := png.Decode(file)
        if err == nil {
            img = ebiten.NewImageFromImage(decoded)
            background.Images[background.Current] = img
            return img
        }
    }

    return nil
}

func (background *ScreenshotsBackground) Update() {
    if background.Life > 0 {
        background.Life -= 1
        // background.X += 0.001 + rand.Float64() / 500
        background.X += background.X_dir
        background.Y += background.Y_dir
    } else {
        background.Life = MAX_LIFE
        if len(background.Files) > 0 {
            background.Current = background.Files[rand.N(len(background.Files))]
            background.GetImage()
        }

        switch rand.N(4) {
            // start on left side, move right
            case 0:
                background.X = -rand.Float64() / 4
                background.Y = rand.Float64() / 2
                background.X_dir = 0.001 + rand.Float64() / 500
                background.Y_dir = 0
            // start on right side, move left
            case 1:
                background.X = 1.01
                background.Y = rand.Float64() / 2
                background.X_dir = -0.001 - rand.Float64() / 500
                background.Y_dir = 0
            // start on top, move down
            case 2:
                background.X = rand.Float64() / 2 + 0.1
                background.Y = -rand.Float64() / 4
                background.X_dir = 0
                background.Y_dir = 0.001 + rand.Float64() / 500
            // start on bottom, move up
            case 3:
                background.X = rand.Float64() / 2 + 0.1
                background.Y = 1.01
                background.X_dir = 0
                background.Y_dir = -0.001 - rand.Float64() / 500
        }

    }
}

func (background *ScreenshotsBackground) Draw(screen *ebiten.Image) {
    img := background.GetImage()
    if img == nil {
        return
    }

    bounds := screen.Bounds()

    var options ebiten.DrawImageOptions
    options.GeoM.Scale(2, 2)

    x := float64(bounds.Dx()) * background.X
    y := float64(bounds.Dy()) * background.Y

    options.GeoM.Translate(x, y)
    // options.GeoM.Translate(float64(bounds.Min.X + bounds.Max.X) / 2, float64(bounds.Min.Y + bounds.Max.Y) / 2)
    // options.GeoM.Translate(-float64(img.Bounds().Dx()) / 2, -float64(img.Bounds().Dy()) / 2)

    var alpha float32 = 1.0

    if background.Life > MAX_LIFE - FADE_MARGIN {
        alpha = float32(MAX_LIFE - background.Life) / FADE_MARGIN
    } else if background.Life < FADE_MARGIN {
        alpha = float32(background.Life) / FADE_MARGIN
    }

    options.ColorScale.ScaleAlpha(alpha)
    
    screen.DrawImage(img, &options)
}

package screenshot

import (
    get_screenshot "github.com/kazzmir/nes/test/screenshot"
    "image/png"
    "path/filepath"
    "fmt"
    "regexp"
    "strconv"
    "os"
    "log"

    "image"

    test_utils "github.com/kazzmir/nes/test/all-test/utils"
)

func findPngFiles(dirPath string) []string {
    dir, err := os.Open(dirPath)
    if err != nil {
        return nil
    }
    defer dir.Close()
    all, err := dir.Readdirnames(0)
    if err != nil {
        return nil
    }

    var out []string
    for _, path := range all {
        if filepath.Ext(path) == ".png" {
            out = append(out, filepath.Join(dirPath, path))
        }
    }

    return out
}

func loadPng(pngPath string) (image.Image, error) {
    file, err := os.Open(pngPath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    return png.Decode(file)
}

func compareImage(image1 image.Image, image2 image.Image) bool {
    if !image1.Bounds().Eq(image2.Bounds()) {
        return false
    }

    width := image1.Bounds().Dx()
    height := image1.Bounds().Dy()

    for x := 0; x < width; x++ {
        for y := 0; y < height; y++ {
            color1 := image1.At(x, y)
            color2 := image2.At(x, y)

            r1, g1, b1, a1 := color1.RGBA()
            r2, g2, b2, a2 := color2.RGBA()

            if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
                return false
            }
        }
    }

    return true
}

func testPng(png string) error {
    pattern := "(.*)-(\\d+)\\.png"
    regex := regexp.MustCompile(pattern)
    matches := regex.FindSubmatch([]byte(png))
    if matches == nil {
        return nil
    }

    if len(matches) != 3 {
        return nil
    }

    romName := filepath.Base(string(matches[1]))
    cycles := string(matches[2])
    cyclesI, err := strconv.ParseInt(cycles, 10, 64)
    if err != nil {
        return err
    }

    expectedImage, err := loadPng(png)
    if err != nil {
        return err
    }

    log.Printf("Test rom %v cycles %v", romName, cycles)

    screen, err := get_screenshot.Run(fmt.Sprintf("roms/%v.nes", romName), cyclesI)
    if err != nil {
        return err
    }


    if !compareImage(expectedImage, get_screenshot.ScreenToImage(screen)) {
        log.Printf(test_utils.Failure(png))
    } else {
        log.Printf(test_utils.Success(png))
    }

    return nil
}

func Run(debug bool) (bool, error) {
    pngFiles := findPngFiles("images")

    for _, png := range pngFiles {
        err := testPng(png)
        if err != nil {
            return false, err
        }
    }

    return true, nil
}

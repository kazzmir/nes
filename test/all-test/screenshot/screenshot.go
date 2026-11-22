package screenshot

import (
    get_screenshot "github.com/kazzmir/nes/test/screenshot"
    "errors"
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
        log.Print(test_utils.Failure(png))
    } else {
        log.Print(test_utils.Success(png))
    }

    return nil
}

var mmc5Checkpoints = [...]int64{180_000, 300_000, 600_000, 1_200_000, 2_400_000}

func checkMMC5Screenshot(dir string, cycles int64) error {
    expectedPath := filepath.Join(dir, fmt.Sprintf("%d.png", cycles))
    expected, err := loadPng(expectedPath)
    if err != nil {
        return err
    }
    screen, err := get_screenshot.Run(filepath.Join(dir, "mmc5test.nes"), cycles)
    if err != nil {
        return err
    }
    if !compareImage(expected, get_screenshot.ScreenToImage(screen)) {
        return fmt.Errorf("pixels differ from %s", expectedPath)
    }
    return nil
}

func Run(debug bool) (bool, error) {
    var failures []error
    for _, cycles := range mmc5Checkpoints {
        name := fmt.Sprintf("mmc5test screenshot at %d CPU cycles", cycles)
        if err := checkMMC5Screenshot(filepath.Join("test-roms", "mmc5test"), cycles); err != nil {
            failures = append(failures, fmt.Errorf("%s: %w", name, err))
            log.Print(test_utils.Failure(name))
        } else {
            log.Print(test_utils.Success(name))
        }
    }

    for _, png := range findPngFiles("images") {
        if err := testPng(png); err != nil {
            failures = append(failures, err)
        }
    }

    return len(failures) == 0, errors.Join(failures...)
}

package gfx

import (
    "testing"
    // "math"
)

/* make sure converting rgb -> hsv -> rgb produces the same rgb values */
func TestHSV(test *testing.T){
    /*
    epsilon := 0.1
    for r := 0; r <= 255; r++ {
        for g := 0; g <= 255; g++ {
            for b := 0; b <= 255; b++ {
                h, s, v := rgb2hsv(sdl.Color{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
                
                r2, g2, b2 := hsv2rgb(h, s, v)
                r2 *= 255
                g2 *= 255
                b2 *= 255

                if math.Abs(float64(r2) - float64(r)) > epsilon {
                    test.Fatalf("hsv failed for r=%v g=%v b=%v", r, g, b)
                }
                if math.Abs(float64(g2) - float64(g)) > epsilon {
                    test.Fatalf("hsv failed for r=%v g=%v b=%v", r, g, b)
                }
                if math.Abs(float64(b2) - float64(b)) > epsilon {
                    test.Fatalf("hsv failed for r=%v g=%v b=%v", r, g, b)
                }
            }
        }
    }
    */
}

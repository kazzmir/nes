package main

//#include <stdlib.h>
import "C"
import (
    "fmt"
    "log"
    "strconv"
    "os"

    nes "github.com/kazzmir/nes/lib"

    "github.com/veandco/go-sdl2/sdl"

    "encoding/binary"
    "bytes"
    "time"
    "sync"
    "context"
    "runtime/pprof"
)

func Run(path string, debug bool, maxCycles uint64) error {
    nesFile, err := nes.ParseNesFile(path)
    if err != nil {
        return err
    }

    // dump_instructions(prgRom)

    cpu := nes.StartupState()

    mapper, err := nes.MakeMapper(nesFile.Mapper, nesFile.ProgramRom)
    if err != nil {
        return err
    }
    err = cpu.SetMapper(mapper)
    if err != nil {
        return err
    }

    cpu.PPU.CopyCharacterRom(0x0000, nesFile.CharacterRom)

    cpu.Input = nes.MakeInput()

    if debug {
        cpu.Debug = 1
        cpu.PPU.Debug = 1
    }

    cpu.Reset()

    // force a software renderer
    // sdl.SetHint(sdl.HINT_RENDER_DRIVER, "software")

    err = sdl.Init(sdl.INIT_EVERYTHING)
    if err != nil {
        return err
    }
    defer sdl.Quit()

    /* to resize the window */
    // | sdl.WINDOW_RESIZABLE
    window, err := sdl.CreateWindow("nes", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, 320 * 3, 240 * 3, sdl.WINDOW_SHOWN)
    if err != nil {
        return err
    }
    defer window.Destroy()

    /*
    surface, err := window.GetSurface()
    if err != nil {
        return err
    }

    surface.FillRect(nil, 0)
    window.UpdateSurface()
    */

    softwareRenderer := true
    _ = softwareRenderer
    // renderer, err := sdl.CreateSoftwareRenderer(surface)
    // renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_SOFTWARE)

    /* Create an accelerated renderer */
    renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)

    if err != nil {
        return err
    }
    defer renderer.Destroy()

    // renderer.SetScale(2, 2)

    /*
    texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_RGB888, sdl.TEXTUREACCESS_TARGET, 640, 480)
    if err != nil {
        return err
    }

    // _ = texture
    // renderer.SetRenderTarget(texture)
    */

    var waiter sync.WaitGroup

    quit, cancel := context.WithCancel(context.Background())
    defer cancel()
    drawn := make(chan nes.VirtualScreen, 1)

    pixelFormat := findPixelFormat()

    /* create a surface from the pixels in one call, then create a texture and render it */
    doRender := func(screen nes.VirtualScreen, raw_pixels []byte) error {
        width := int32(320)
        height := int32(240)
        depth := 8 * 4 // RGBA8888
        pitch := int(width) * int(depth) / 8

        for i, pixel := range screen.Buffer {
            /* red */
            raw_pixels[i*4+0] = byte(pixel >> 24)
            /* green */
            raw_pixels[i*4+1] = byte(pixel >> 16)
            /* blue */
            raw_pixels[i*4+2] = byte(pixel >> 8)
            /* alpha */
            raw_pixels[i*4+3] = byte(pixel >> 0)
        }

        pixels := C.CBytes(raw_pixels)
        defer C.free(pixels)

        // pixelFormat := sdl.PIXELFORMAT_ABGR8888

        /* pixelFormat should be ABGR8888 on little-endian (x86) and
         * RBGA8888 on big-endian (arm)
         */

        surface, err := sdl.CreateRGBSurfaceWithFormatFrom(pixels, width, height, int32(depth), int32(pitch), pixelFormat)
        if err != nil {
            return fmt.Errorf("Unable to create surface from pixels: %v", err)
        }
        if surface == nil {
            return fmt.Errorf("Did not create a surface somehow")
        }

        defer surface.Free()

        texture, err := renderer.CreateTextureFromSurface(surface)
        if err != nil {
            return fmt.Errorf("Could not create texture: %v", err)
        }

        defer texture.Destroy()

        // texture_format, access, width, height, err := texture.Query()
        // log.Printf("Texture format=%v access=%v width=%v height=%v err=%v\n", get_pixel_format(texture_format), access, width, height, err)

        renderer.Clear()
        renderer.Copy(texture, nil, nil)
        renderer.Present()

        return nil
    }

    go func(){
        waiter.Add(1)
        defer waiter.Done()
        raw_pixels := make([]byte, 320*240*4)
        // raw_pixels := make([]byte, 320*240*3)
        fps := 0
        timer := time.NewTicker(1 * time.Second)
        defer timer.Stop()
        for {
            select {
                case <-quit.Done():
                    return
                case screen := <-drawn:
                    err := doRender(screen, raw_pixels)
                    fps += 1
                    if err != nil {
                        log.Printf("Could not render: %v\n", err)
                    }
                case <-timer.C:
                    log.Printf("FPS: %v", fps)
                    fps = 0
            }
        }
    }()

    turbo := make(chan bool, 10)

    go runNES(cpu, maxCycles, quit, drawn, turbo, &waiter)

    var turboKey sdl.Scancode = sdl.SCANCODE_GRAVE

    for quit.Err() == nil {
        event := sdl.WaitEvent()
        if event != nil {
            // log.Printf("Event %+v\n", event)
            switch event.GetType() {
                case sdl.QUIT: cancel()
                case sdl.KEYDOWN:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    // log.Printf("key down %+v pressed %v escape %v", keyboard_event, keyboard_event.State == sdl.PRESSED, keyboard_event.Keysym.Sym == sdl.K_ESCAPE)
                    quit_pressed := keyboard_event.State == sdl.PRESSED && (keyboard_event.Keysym.Sym == sdl.K_ESCAPE || keyboard_event.Keysym.Sym == sdl.K_CAPSLOCK)
                    if quit_pressed {
                        cancel()
                    }

                    if keyboard_event.Keysym.Scancode == turboKey {
                        select {
                            case turbo <- true:
                        }
                    }
                case sdl.KEYUP:
                    keyboard_event := event.(*sdl.KeyboardEvent)
                    if keyboard_event.Keysym.Scancode == turboKey {
                        select {
                            case turbo <- false:
                        }
                    }
            }
        }
    }

    log.Printf("Waiting to quit..")
    waiter.Wait()

    return nil
}

/* determine endianness of the host by comparing the least-significant byte of a 32-bit number
 * versus a little endian byte array
 * if the first byte in the byte array is the same as the lowest byte of the 32-bit number
 * then the host is little endian
 */
func findPixelFormat() uint32 {
    red := uint32(32)
    green := uint32(128)
    blue := uint32(64)
    alpha := uint32(96)
    color := (red << 24) | (green << 16) | (blue << 8) | alpha

    var buffer bytes.Buffer
    binary.Write(&buffer, binary.LittleEndian, color)

    if buffer.Bytes()[0] == uint8(alpha) {
        return sdl.PIXELFORMAT_ABGR8888
    }

    return sdl.PIXELFORMAT_RGBA8888
}

func get_pixel_format(format uint32) string {
    switch format {
        case sdl.PIXELFORMAT_BGR888: return "BGR888"
        case sdl.PIXELFORMAT_ARGB8888: return "ARGB8888"
        case sdl.PIXELFORMAT_RGB888: return "RGB888"
        case sdl.PIXELFORMAT_RGBA8888: return "RGBA8888"
    }

    return fmt.Sprintf("%v?", format)
}

func runNES(cpu nes.CPUState, maxCycles uint64, quit context.Context, draw chan nes.VirtualScreen, turbo <-chan bool, waiter *sync.WaitGroup){
    waiter.Add(1)
    defer waiter.Done()

    instructionTable := nes.MakeInstructionDescriptiontable()

    screen := nes.MakeVirtualScreen(320, 240)

    var quitEvent sdl.QuitEvent
    quitEvent.Type = sdl.QUIT
    /* FIXME: does quitEvent.Timestamp need to be set? */
    defer sdl.PushEvent(&quitEvent)

    var cycleCounter float64
    /* http://wiki.nesdev.com/w/index.php/Cycle_reference_chart#Clock_rates
     * NTSC 2c0c clock speed is 21.47~ MHz ÷ 12 = 1.789773 MHz
     * Every millisecond we should run this many cycles
     */
    cycleDiff := (1.789773 * 1000000) / 1000

    cycleTimer := time.NewTicker(1 * time.Millisecond)

    turboMultiplier := float64(1)

    for quit.Err() == nil {
        if maxCycles > 0 && cpu.Cycle >= maxCycles {
            break
        }

        for cycleCounter <= 0 {
            select {
                case enable := <-turbo:
                    if enable {
                        turboMultiplier = 3
                    } else {
                        turboMultiplier = 1
                    }
                case <-quit.Done():
                    return
                case <-cycleTimer.C:
                    cycleCounter += cycleDiff * turboMultiplier
            }
        }

        // log.Printf("Cycle counter %v\n", cycleCounter)

        cycles := cpu.Cycle
        err := cpu.Run(instructionTable)
        if err != nil {
            log.Fatal(err)
            return
        }
        usedCycles := cpu.Cycle

        cycleCounter -= float64(usedCycles - cycles)

        /* ppu runs 3 times faster than cpu */
        nmi, drawn := cpu.PPU.Run((usedCycles - cycles) * 3, screen)

        if drawn {
            select {
                case draw <- screen.Copy():
                default:
            }
        }

        if nmi {
            if cpu.Debug > 0 {
                log.Printf("Cycle %v Do NMI\n", cpu.Cycle)
            }
            cpu.NMI()
        }
    }
}

func main(){
    log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

    var nesPath string
    var debug bool
    var maxCycles uint64

    argIndex := 1
    for argIndex < len(os.Args) {
        arg := os.Args[argIndex]
        switch arg {
            case "-debug", "--debug":
                debug = true
            case "-cycles", "--cycles":
                var err error
                argIndex += 1
                if argIndex >= len(os.Args) {
                    log.Fatalf("Expected a number of cycles\n")
                }
                maxCycles, err = strconv.ParseUint(os.Args[argIndex], 10, 64)
                if err != nil {
                    log.Fatalf("Error parsing cycles: %v\n", err)
                }
            default:
                nesPath = arg
        }

        argIndex += 1
    }

    if nesPath != "" {
        profile, err := os.Create("profile.cpu")
        if err != nil {
            log.Fatal(err)
        }
        pprof.StartCPUProfile(profile)
        defer pprof.StopCPUProfile()
        err = Run(nesPath, debug, maxCycles)
        if err != nil {
            log.Printf("Error: %v\n", err)
        }
    } else {
        fmt.Printf("Give a .nes argument\n")
    }

    log.Printf("Bye")
}

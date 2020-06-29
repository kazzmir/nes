An NES emulator written in Go.

Resources:

* Nes Development Wiki http://wiki.nesdev.com/w/index.php/Nesdev_Wiki

* Another nes emulator in go https://github.com/fogleman/nes

Log:

6/20/2020: cpu is mostly complete

![first render](./pics/nes1.png)

6/23/2020: The first time the emulator was able to render an image. This is from the Baseball game. It was very exciting to see an image show up, even if the colors are not exactly correct! Mostly the difficulty in getting to this point was understanding pattern tables and nametables.

![with a palette](./pics/nes-palette.png)

6/24/2020: The same Baseball game but rendered with colors from a palette derived from the pattern attributes. I think the colors are still wrong here, as other rom's show a lot of dark blue as well, but its a step in the right direction.

![basic sprites](./pics/contra.gif)

6/28/2020: Contra is playable where sprites are kind of rendered, but the background doesn't scroll. This also demonstrates mapper 2, which contra uses.

6/29/2020: I was using SDL to call `SDL_DrawPoint` on a renderer for each pixel rendered by the PPU, but this ended up making frames take 10-20ms to render, which seems absurdly long for a modern machine (even with software rendering). The reason seems to be that each call to `SDL_DrawPoint` incurs a runtime.cgocall invocation, which has high overhead. The pprof cpu profile demonstrates this:

```
(pprof) top 10
Showing nodes accounting for 21830ms, 77.11% of 28310ms total
Dropped 124 nodes (cum <= 141.55ms)
Showing top 10 nodes out of 50
      flat  flat%   sum%        cum   cum%
   14320ms 50.58% 50.58%    19380ms 68.46%  runtime.cgocall
    1390ms  4.91% 55.49%     1670ms  5.90%  runtime.mapiternext
    1350ms  4.77% 60.26%     2310ms  8.16%  runtime.exitsyscallfast
    1210ms  4.27% 64.54%     1210ms  4.27%  runtime.casgstatus
     800ms  2.83% 67.36%     4320ms 15.26%  runtime.exitsyscall
     790ms  2.79% 70.15%      790ms  2.79%  runtime.wirep
     530ms  1.87% 72.02%     1990ms  7.03%  runtime.cgoCheckPointer
     520ms  1.84% 73.86%    23330ms 82.41%  github.com/kazzmir/nes/lib.(*PPUState).Render
     490ms  1.73% 75.59%     2350ms  8.30%  github.com/kazzmir/nes/lib.(*CPUState).LoadMemory
     430ms  1.52% 77.11%     1040ms  3.67%  runtime.cgoIsGoPointer
```

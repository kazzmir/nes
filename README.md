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

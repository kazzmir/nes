#!/bin/sh

# Installs the cross compilation libraries to build mingw (windows) binaries on linux

ttf_url=https://github.com/libsdl-org/SDL_ttf/releases/download/release-2.22.0/SDL2_ttf-devel-2.22.0-mingw.tar.gz
mixer_url=https://github.com/libsdl-org/SDL_mixer/releases/download/release-2.8.0/SDL2_mixer-devel-2.8.0-mingw.tar.gz
sdl_url=https://github.com/libsdl-org/SDL/releases/download/release-2.30.10/SDL2-devel-2.30.10-mingw.tar.gz

apt install -y mingw-w64

rm -rf sdl_libs
mkdir sdl_libs
cd sdl_libs

for url in $ttf_url $mixer_url $sdl_url
do
    echo "Downloading $url"
    curl -L $url | tar -xz
done

echo "Copying x86_64-w64-mingw32 directories to /usr"
sudo find . -name "x86_64-w64-mingw32" -type d -exec cp -r {} /usr

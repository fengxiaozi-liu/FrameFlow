// Generate the Windows icon from the same teal F mark used in the app header.
// Run from the repository root: go run scripts/windows-icon/main.go
// Then run from backend/cmd/server:
// go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --out icon --manifest none --icon frameflow.ico --file-description FrameFlow --product-name FrameFlow --original-filename FrameFlow.exe
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	var images [][]byte
	for _, size := range []int{16, 32, 48, 256} {
		var data bytes.Buffer
		if err := png.Encode(&data, drawIcon(size)); err != nil {
			panic(err)
		}
		images = append(images, data.Bytes())
	}
	var icon bytes.Buffer
	write := func(v any) {
		if err := binary.Write(&icon, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	write(uint16(0))
	write(uint16(1))
	write(uint16(len(images)))
	offset := uint32(6 + len(images)*16)
	for i, data := range images {
		size := []int{16, 32, 48, 256}[i]
		write(uint8(size % 256))
		write(uint8(size % 256))
		write(uint8(0))
		write(uint8(0))
		write(uint16(1))
		write(uint16(32))
		write(uint32(len(data)))
		write(offset)
		offset += uint32(len(data))
	}
	for _, data := range images {
		if _, err := icon.Write(data); err != nil {
			panic(err)
		}
	}
	if err := os.WriteFile("backend/cmd/server/frameflow.ico", icon.Bytes(), 0644); err != nil {
		panic(err)
	}
	if err := os.MkdirAll("frontend/public", 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile("frontend/public/favicon.png", images[len(images)-1], 0644); err != nil {
		panic(err)
	}
}

func drawIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a int
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/4) / float64(size)
					py := (float64(y) + (float64(sy)+0.5)/4) / float64(size)
					if !roundedSquare(px, py) {
						continue
					}
					cr, cg, cb := 6, 112, 103
					if px >= .31 && px <= .45 && py >= .25 && py <= .76 ||
						px >= .31 && px <= .72 && py >= .25 && py <= .38 ||
						px >= .31 && px <= .63 && py >= .47 && py <= .59 {
						cr, cg, cb = 255, 255, 255
					}
					r += cr
					g += cg
					b += cb
					a += 255
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(r / 16), G: uint8(g / 16), B: uint8(b / 16), A: uint8(a / 16)})
		}
	}
	return img
}

func roundedSquare(x, y float64) bool {
	if x < .05 || x > .95 || y < .05 || y > .95 {
		return false
	}
	cx, cy := x, y
	if x < .23 {
		cx = .23
	} else if x > .77 {
		cx = .77
	}
	if y < .23 {
		cy = .23
	} else if y > .77 {
		cy = .77
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= .18*.18
}

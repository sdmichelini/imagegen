package imageconv

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/kolesa-team/go-webp/decoder"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
)

func ToJPG(data []byte) ([]byte, error) {
	img, err := decodeImage(data)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func ToWEBP(data []byte) ([]byte, error) {
	img, err := decodeImage(data)
	if err != nil {
		return nil, err
	}

	opts, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 85)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := webp.Encode(&out, img, opts); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func ToPNG(data []byte) ([]byte, error) {
	img, err := decodeImage(data)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func ToICO(data []byte) ([]byte, error) {
	return ToICOWithSizes(data, []int{16, 32, 48})
}

func ToICOWithSizes(data []byte, sizes []int) ([]byte, error) {
	img, err := decodeImage(data)
	if err != nil {
		return nil, err
	}

	icons := make([]icoImage, 0, len(sizes))
	for _, size := range sizes {
		resized := resizeNearest(img, size, size)
		var pngBuf bytes.Buffer
		if err := png.Encode(&pngBuf, resized); err != nil {
			return nil, err
		}
		icons = append(icons, icoImage{
			width:  size,
			height: size,
			data:   pngBuf.Bytes(),
		})
	}

	return wrapPNGsAsICO(icons), nil
}

func decodeImage(data []byte) (image.Image, error) {
	if isWEBP(data) {
		return webp.Decode(bytes.NewReader(data), &decoder.Options{})
	}
	if isICO(data) {
		return decodeICO(data)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func isWEBP(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	return string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func isICO(data []byte) bool {
	if len(data) < 6 {
		return false
	}
	return data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0
}

func decodeICO(data []byte) (image.Image, error) {
	if len(data) < 6 {
		return nil, errors.New("invalid ico: header too short")
	}
	imageCount := int(binary.LittleEndian.Uint16(data[4:6]))
	if imageCount < 1 {
		return nil, errors.New("invalid ico: no image entries")
	}

	const (
		headerSize = 6
		entrySize  = 16
	)
	if len(data) < headerSize+imageCount*entrySize {
		return nil, errors.New("invalid ico: truncated entry table")
	}

	bestArea := -1
	var bestPayload []byte
	for i := 0; i < imageCount; i++ {
		entryOffset := headerSize + i*entrySize
		w := int(data[entryOffset+0])
		h := int(data[entryOffset+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}

		imgSize := int(binary.LittleEndian.Uint32(data[entryOffset+8 : entryOffset+12]))
		imgOffset := int(binary.LittleEndian.Uint32(data[entryOffset+12 : entryOffset+16]))
		if imgSize <= 0 || imgOffset < 0 || imgOffset+imgSize > len(data) {
			continue
		}

		payload := data[imgOffset : imgOffset+imgSize]
		if !isPNG(payload) {
			continue
		}

		area := w * h
		if area > bestArea {
			bestArea = area
			bestPayload = payload
		}
	}

	if len(bestPayload) == 0 {
		return nil, fmt.Errorf("unsupported ico payload format (non-png)")
	}

	return png.Decode(bytes.NewReader(bestPayload))
}

func isPNG(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	return bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
}

func resizeNearest(src image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	b := src.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return dst
	}

	for y := 0; y < height; y++ {
		srcY := b.Min.Y + (y*srcH)/height
		for x := 0; x < width; x++ {
			srcX := b.Min.X + (x*srcW)/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

type icoImage struct {
	width  int
	height int
	data   []byte
}

func wrapPNGsAsICO(images []icoImage) []byte {
	const (
		headerSize = 6
		entrySize  = 16
	)

	entriesSize := entrySize * len(images)
	totalImageDataSize := 0
	for _, img := range images {
		totalImageDataSize += len(img.data)
	}

	ico := make([]byte, headerSize+entriesSize+totalImageDataSize)
	binary.LittleEndian.PutUint16(ico[0:2], 0) // reserved
	binary.LittleEndian.PutUint16(ico[2:4], 1) // type: icon
	binary.LittleEndian.PutUint16(ico[4:6], uint16(len(images)))

	nextDataOffset := headerSize + entriesSize
	for i, img := range images {
		entryOffset := headerSize + i*entrySize
		ico[entryOffset+0] = byte(img.width)
		ico[entryOffset+1] = byte(img.height)
		ico[entryOffset+2] = 0 // palette colors
		ico[entryOffset+3] = 0 // reserved
		binary.LittleEndian.PutUint16(ico[entryOffset+4:entryOffset+6], 1)
		binary.LittleEndian.PutUint16(ico[entryOffset+6:entryOffset+8], 32)
		binary.LittleEndian.PutUint32(ico[entryOffset+8:entryOffset+12], uint32(len(img.data)))
		binary.LittleEndian.PutUint32(ico[entryOffset+12:entryOffset+16], uint32(nextDataOffset))

		copy(ico[nextDataOffset:nextDataOffset+len(img.data)], img.data)
		nextDataOffset += len(img.data)
	}

	return ico
}

package imageconv

import (
	"bytes"
	"encoding/binary"
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

func ToICO(data []byte) ([]byte, error) {
	img, err := decodeImage(data)
	if err != nil {
		return nil, err
	}

	const iconSize = 48
	resized := resizeNearest(img, iconSize, iconSize)

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, resized); err != nil {
		return nil, err
	}

	return wrapPNGAsICO(pngBuf.Bytes(), iconSize, iconSize), nil
}

func decodeImage(data []byte) (image.Image, error) {
	if isWEBP(data) {
		return webp.Decode(bytes.NewReader(data), &decoder.Options{})
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

func wrapPNGAsICO(pngData []byte, width, height int) []byte {
	const (
		headerSize = 6
		entrySize  = 16
	)

	ico := make([]byte, headerSize+entrySize+len(pngData))
	binary.LittleEndian.PutUint16(ico[0:2], 0) // reserved
	binary.LittleEndian.PutUint16(ico[2:4], 1) // type: icon
	binary.LittleEndian.PutUint16(ico[4:6], 1) // one image

	offset := headerSize
	ico[offset+0] = byte(width)
	ico[offset+1] = byte(height)
	ico[offset+2] = 0 // palette colors
	ico[offset+3] = 0 // reserved
	binary.LittleEndian.PutUint16(ico[offset+4:offset+6], 1)
	binary.LittleEndian.PutUint16(ico[offset+6:offset+8], 32)
	binary.LittleEndian.PutUint32(ico[offset+8:offset+12], uint32(len(pngData)))
	binary.LittleEndian.PutUint32(ico[offset+12:offset+16], uint32(headerSize+entrySize))

	copy(ico[headerSize+entrySize:], pngData)
	return ico
}

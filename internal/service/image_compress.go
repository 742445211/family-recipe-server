package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// ErrImageCompressFailed 服务端无法将图片压缩到微信检测上限内。
var ErrImageCompressFailed = errors.New("图片压缩失败，请换一张较小的图片")

// PrepareImageForSecCheck 若超过微信 img_sec_check 上限则在服务端压缩，否则原样返回。
func PrepareImageForSecCheck(data []byte, filename string) ([]byte, string, error) {
	if len(data) <= maxImgSecCheckBytes {
		return data, filename, nil
	}
	return compressImageToMaxBytes(data, filename, maxImgSecCheckBytes)
}

func compressImageToMaxBytes(data []byte, filename string, maxBytes int) ([]byte, string, error) {
	img, err := decodeUploadImage(data)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrImageCompressFailed, err)
	}
	img = flattenForJPEG(img)

	scales := []float64{1.0, 0.85, 0.7, 0.55, 0.4, 0.3, 0.2}
	qualities := []int{90, 85, 80, 75, 70, 65, 60, 55, 50, 45, 40, 35}

	for _, scale := range scales {
		scaled := img
		if scale < 1.0 {
			scaled = scaleImage(img, scale)
		}
		for _, q := range qualities {
			buf, err := encodeJPEG(scaled, q)
			if err != nil {
				continue
			}
			if len(buf) <= maxBytes {
				return buf, secCheckJPEGName(filename), nil
			}
		}
	}
	return nil, "", ErrImageCompressFailed
}

func decodeUploadImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil
	}
	return webp.Decode(bytes.NewReader(data))
}

func flattenForJPEG(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, b, src, b.Min, draw.Over)
	return dst
}

func scaleImage(src image.Image, scale float64) image.Image {
	b := src.Bounds()
	w := int(float64(b.Dx()) * scale)
	h := int(float64(b.Dy()) * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func secCheckJPEGName(filename string) string {
	base := filepath.Base(filename)
	if base == "" || base == "." {
		return "image.jpg"
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return base + ".jpg"
	}
	return strings.TrimSuffix(base, ext) + ".jpg"
}

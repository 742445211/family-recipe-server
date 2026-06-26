package service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func makeLargeJPEG(t *testing.T, w, h, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPrepareImageForSecCheckSmallUnchanged(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	out, name, err := PrepareImageForSecCheck(data, "a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) || name != "a.jpg" {
		t.Fatalf("got name=%q len=%d", name, len(out))
	}
}

func TestPrepareImageForSecCheckLargeJPEG(t *testing.T) {
	data := makeLargeJPEG(t, 4000, 3000, 98)
	if len(data) <= maxImgSecCheckBytes {
		t.Fatalf("fixture too small: %d", len(data))
	}
	out, name, err := PrepareImageForSecCheck(data, "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > maxImgSecCheckBytes {
		t.Fatalf("compressed still too large: %d", len(out))
	}
	if name != "photo.jpg" {
		t.Fatalf("name=%q", name)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("invalid jpeg: %v", err)
	}
}

func TestPrepareImageForSecCheckInvalidData(t *testing.T) {
	data := make([]byte, maxImgSecCheckBytes+1)
	_, _, err := PrepareImageForSecCheck(data, "bad.jpg")
	if err == nil {
		t.Fatal("expected error")
	}
}

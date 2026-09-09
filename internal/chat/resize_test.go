package chat

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{R: 255, A: 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decodeSize(t *testing.T, data []byte) (int, int, string) {
	t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height, format
}

func TestResizeImageLeavesSmallImagesUntouched(t *testing.T) {
	data := encodePNG(t, 1200, 800)
	out, mime := ResizeImage(data, "image/png")
	if !bytes.Equal(out, data) || mime != "image/png" {
		t.Fatalf("small image must pass through unchanged")
	}
}

func TestResizeImageShrinksLongestEdgeTo2000(t *testing.T) {
	out, mime := ResizeImage(encodePNG(t, 1170, 2532), "image/png") // phone screenshot
	w, h, format := decodeSize(t, out)
	if mime != "image/png" || format != "png" {
		t.Fatalf("mime=%s format=%s", mime, format)
	}
	if h != 2000 || w != 924 {
		t.Fatalf("got %dx%d, want 924x2000", w, h)
	}
}

func TestResizeImageKeepsJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3000, 1500))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	out, mime := ResizeImage(buf.Bytes(), "image/jpeg")
	w, h, format := decodeSize(t, out)
	if mime != "image/jpeg" || format != "jpeg" || w != 2000 || h != 1000 {
		t.Fatalf("mime=%s format=%s size=%dx%d", mime, format, w, h)
	}
}

func TestResizeImageReturnsGarbageUnchanged(t *testing.T) {
	junk := []byte("not an image")
	out, mime := ResizeImage(junk, "image/png")
	if !bytes.Equal(out, junk) || mime != "image/png" {
		t.Fatalf("undecodable input must pass through")
	}
}

func TestParseRequestResizesOversizedUpload(t *testing.T) {
	req := multipartRequest(t, "look", map[string]testUpload{
		"images": {name: "big.png", body: string(encodePNG(t, 2600, 2600))},
	})
	parsed, err := ParseRequest(req, DefaultMaxImageBytes, DefaultMaxRequestBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Images) != 1 {
		t.Fatalf("want 1 inline image, got %d", len(parsed.Images))
	}
	raw, err := base64.StdEncoding.DecodeString(parsed.Images[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if w, h, _ := decodeSize(t, raw); w != 2000 || h != 2000 {
		t.Fatalf("inline image %dx%d, want 2000x2000", w, h)
	}
	// The on-disk upload keeps the original bytes.
	if len(parsed.Files) != 1 || len(parsed.Files[0].Data) == len(raw) {
		t.Fatalf("saved upload should be the original file")
	}
}

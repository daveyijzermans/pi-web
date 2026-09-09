package chat

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// MaxImageEdge is the longest edge an inline image may have. Anthropic rejects
// many-image requests whose images exceed 2000px on either side, and pi only
// auto-resizes @file/read/tool images — not images handed to it via RPC — so
// pi-web must shrink browser uploads itself before they land in the session.
const MaxImageEdge = 2000

// ResizeImage returns data unchanged (and the original mimeType) when the image
// already fits within MaxImageEdge. Otherwise it decodes, scales the longest
// edge down to MaxImageEdge and re-encodes: JPEG stays JPEG; PNG, GIF (first
// frame) and WebP become PNG so transparency survives. Undecodable input is
// returned as-is so the caller's existing size/type checks still apply.
func ResizeImage(data []byte, mimeType string) ([]byte, string) {
	var src image.Image
	var err error
	switch mimeType {
	case "image/jpeg":
		src, err = jpeg.Decode(bytes.NewReader(data))
	case "image/png":
		src, err = png.Decode(bytes.NewReader(data))
	case "image/gif":
		src, err = gif.Decode(bytes.NewReader(data))
	case "image/webp":
		src, err = webp.Decode(bytes.NewReader(data))
	default:
		return data, mimeType
	}
	if err != nil {
		return data, mimeType
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= MaxImageEdge && h <= MaxImageEdge {
		return data, mimeType
	}
	scale := float64(MaxImageEdge) / float64(max(w, h))
	dw := max(1, int(float64(w)*scale))
	dh := max(1, int(float64(h)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var out bytes.Buffer
	if mimeType == "image/jpeg" {
		if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 88}); err != nil {
			return data, mimeType
		}
		return out.Bytes(), "image/jpeg"
	}
	if err := png.Encode(&out, dst); err != nil {
		return data, mimeType
	}
	return out.Bytes(), "image/png"
}

package chat

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
)

const DefaultMaxImageBytes int64 = 10 << 20
const DefaultMaxRequestBytes int64 = 32 << 20

var ErrEmptyRequest = errors.New("message or image required")
var ErrImageTooLarge = errors.New("image attachment too large")

var inlineImageMimes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

type Image struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

type Request struct {
	Message string
	Images  []Image
	Files   []UploadedFile
}

// stripCharset removes a "; charset=..." suffix from a MIME type.
func stripCharset(mimeType string) string {
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		return mimeType[:idx]
	}
	return mimeType
}

func ParseRequest(r *http.Request, maxImageBytes, maxRequestBytes int64) (Request, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBytes)
	if err := r.ParseMultipartForm(maxRequestBytes); err != nil {
		return Request{}, err
	}

	chat := Request{Message: strings.TrimSpace(r.FormValue("message"))}
	if r.MultipartForm == nil {
		if chat.Message == "" {
			return Request{}, ErrEmptyRequest
		}
		return chat, nil
	}

	for _, fh := range r.MultipartForm.File["images"] {
		file, err := fh.Open()
		if err != nil {
			return Request{}, err
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return Request{}, readErr
		}
		if closeErr != nil {
			return Request{}, closeErr
		}
		mimeType := stripCharset(http.DetectContentType(data))

		// Every uploaded file is saved to disk
		chat.Files = append(chat.Files, UploadedFile{Name: fh.Filename, MimeType: mimeType, Data: data})

		// Only whitelisted image types are sent inline to the model
		if inlineImageMimes[mimeType] {
			// Shrink oversized images first (see ResizeImage), then cap the
			// inline payload at maxImageBytes.
			inline, inlineMime := ResizeImage(data, mimeType)
			if int64(len(inline)) > maxImageBytes {
				return Request{}, ErrImageTooLarge
			}
			chat.Images = append(chat.Images, Image{Type: "image", Data: base64.StdEncoding.EncodeToString(inline), MimeType: inlineMime})
		}
	}

	if chat.Message == "" && len(chat.Files) == 0 {
		return Request{}, ErrEmptyRequest
	}
	return chat, nil
}

package api

import (
	"bytes"
	"io"

	"github.com/ledongthuc/pdf"       // go get github.com/ledongthuc/pdf
	"github.com/otiai10/gosseract/v2" // go get github.com/otiai10/gosseract/v2
)

// PDF
func ExtractTextFromPDF(r io.Reader) (string, error) {
	f, err := pdf.NewReader(r, int64(r.(io.Seeker).Seek(0, io.SeekEnd)))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	b, err := f.GetPlainText()
	if err != nil {
		return "", err
	}
	_, err = buf.ReadFrom(b)
	return buf.String(), err
}

// Markdown
func ExtractTextFromMarkdown(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// 图片 OCR
func ExtractTextFromImage(r io.Reader) (string, error) {
	client := gosseract.NewClient()
	defer client.Close()
	imgBytes, _ := io.ReadAll(r)
	client.SetImageFromBytes(imgBytes)
	return client.Text()
}

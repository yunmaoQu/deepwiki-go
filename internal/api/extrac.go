package api

import (
	"bytes"
	"io"

	"github.com/ledongthuc/pdf"       
	"github.com/otiai10/gosseract/v2" 
)

// PDF
func ExtractTextFromPDF(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	bytesReader := bytes.NewReader(data)
	f, err := pdf.NewReader(bytesReader, int64(len(data)))
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

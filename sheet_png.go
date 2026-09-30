package yfgo

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
)

// Store the company URL in a standard uncompressed UTF-8 iTXt chunk. A consumer
// can show a clickable link without another API request or changing CLI stdout.
func encodeSheetPNG(w io.Writer, img image.Image, website string) error {
	website = sheetWebsite(website)
	if website == "" {
		return png.Encode(w, img)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return err
	}
	// Keyword, compression flag/method, empty language and translated keyword.
	data := []byte("CompanyURL\x00\x00\x00\x00\x00" + website)
	chunk := make([]byte, len(data)+12)
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], "iTXt")
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	// PNG signature (8 bytes) and IHDR (25 bytes) always precede other chunks.
	for _, part := range [][]byte{encoded.Bytes()[:33], chunk, encoded.Bytes()[33:]} {
		n, err := w.Write(part)
		if err != nil {
			return err
		}
		if n != len(part) {
			return io.ErrShortWrite
		}
	}
	return nil
}

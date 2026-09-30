package yfgo

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"testing"
)

func TestSheetPNGWebsiteMetadata(t *testing.T) {
	for _, tt := range []struct{ website, want string }{
		{"https://global.toyota", "https://global.toyota"},
		{"http://example.com/a(b)?x=1&y=2", "http://example.com/a(b)?x=1&y=2"},
		{"https://日本.example", "https://%E6%97%A5%E6%9C%AC.example"},
		{"", ""}, {"not a URL", ""}, {"javascript:alert(1)", ""},
	} {
		var encoded bytes.Buffer
		if err := encodeSheetPNG(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2)), tt.website); err != nil {
			t.Fatal(err)
		}
		if _, err := png.Decode(bytes.NewReader(encoded.Bytes())); err != nil {
			t.Fatalf("metadata broke PNG decoding: %v", err)
		}
		data := encoded.Bytes()
		found := ""
		for offset := 8; offset+12 <= len(data); {
			length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
			end := offset + length + 12
			if end > len(data) {
				t.Fatal("truncated PNG chunk")
			}
			if got, want := binary.BigEndian.Uint32(data[end-4:end]), crc32.ChecksumIEEE(data[offset+4:end-4]); got != want {
				t.Fatal("invalid PNG chunk CRC")
			}
			if string(data[offset+4:offset+8]) == "iTXt" {
				content := data[offset+8 : end-4]
				prefix := []byte("CompanyURL\x00\x00\x00\x00\x00")
				if !bytes.HasPrefix(content, prefix) {
					t.Fatal("wrong metadata fields")
				}
				found = string(content[len(prefix):])
			}
			offset = end
		}
		if found != tt.want {
			t.Errorf("website %q: got %q want %q", tt.website, found, tt.want)
		}
	}
}

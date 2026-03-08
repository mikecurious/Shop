package barcode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
	"github.com/google/uuid"
)

// Generate creates a unique barcode string value
func Generate() (string, error) {
	// Format: KSK-{timestamp}-{random}
	ts := time.Now().UnixMilli()
	uid := uuid.New().String()[:6]
	return fmt.Sprintf("KSK%d%s", ts%1000000, strings.ToUpper(uid)), nil
}

// GenerateCode128PNG returns a base64-encoded PNG of a Code128 barcode
func GenerateCode128PNG(value string, width, height int) (string, error) {
	bc, err := code128.Encode(value)
	if err != nil {
		return "", fmt.Errorf("encode barcode: %w", err)
	}

	scaled, err := barcode.Scale(bc, width, height)
	if err != nil {
		return "", fmt.Errorf("scale barcode: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// GenerateQRPNG returns a base64-encoded PNG of a QR code
func GenerateQRPNG(value string, size int) (string, error) {
	bc, err := qr.Encode(value, qr.M, qr.Auto)
	if err != nil {
		return "", fmt.Errorf("encode qr: %w", err)
	}

	scaled, err := barcode.Scale(bc, size, size)
	if err != nil {
		return "", fmt.Errorf("scale qr: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

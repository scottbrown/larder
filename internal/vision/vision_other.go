//go:build !darwin

// Package vision recognises text in receipt PDFs using the macOS Vision
// framework.
package vision

import (
	"errors"

	"github.com/scottbrown/larder"
)

// DefaultDPI is the resolution receipts are rendered at before recognition.
const DefaultDPI = 200

// Recognizer is unavailable outside macOS.
type Recognizer struct {
	DPI int
}

// Recognize always fails outside macOS.
func (Recognizer) Recognize(string) (larder.Document, error) {
	return nil, errors.New("text recognition requires macOS")
}

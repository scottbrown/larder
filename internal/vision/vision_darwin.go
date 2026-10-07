//go:build darwin

// Package vision recognises text in receipt PDFs using the macOS Vision
// framework.
package vision

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework Vision -framework CoreGraphics
#include <stdlib.h>
#include "vision.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"

	"github.com/scottbrown/larder"
)

// DefaultDPI is the resolution receipts are rendered at before recognition
// unless a Recognizer says otherwise.
const DefaultDPI = 200

// Recognizer implements larder.Recognizer with Apple's Vision framework.
type Recognizer struct {
	// DPI is the rendering resolution; zero means DefaultDPI.
	DPI int
}

// Recognize renders each page of the PDF at path and returns the text
// observations found on it.
func (r Recognizer) Recognize(path string) (larder.Document, error) {
	dpi := r.DPI
	if dpi == 0 {
		dpi = DefaultDPI
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var cerr *C.char
	out := C.larder_recognize_pdf(cpath, C.double(dpi), &cerr)
	if out == nil {
		msg := "unknown error"
		if cerr != nil {
			msg = C.GoString(cerr)
			C.free(unsafe.Pointer(cerr))
		}
		return nil, fmt.Errorf("recognising %s: %w", path, errors.New(msg))
	}
	defer C.free(unsafe.Pointer(out))

	var doc larder.Document
	if err := json.Unmarshal([]byte(C.GoString(out)), &doc); err != nil {
		return nil, fmt.Errorf("decoding recognition of %s: %w", path, err)
	}
	return doc, nil
}

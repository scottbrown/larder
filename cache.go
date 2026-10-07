package larder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// cacheVersion changes whenever recognition output changes shape or quality,
// invalidating earlier cache entries.
const cacheVersion = "v2"

// CachedRecognizer memoises another Recognizer on disk, keyed by the content
// of each receipt so renamed or moved files are not recognised twice.
type CachedRecognizer struct {
	Inner Recognizer
	Dir   string
	// Variant distinguishes caches for differently configured recognisers,
	// such as other rendering resolutions.
	Variant string
}

// Recognize returns the cached document for path, recognising and caching it
// first if needed.
func (c CachedRecognizer) Recognize(path string) (Document, error) {
	key, err := fileDigest(path)
	if err != nil {
		return nil, err
	}
	cached := filepath.Join(c.Dir, cacheVersion, c.Variant, key[:2], key+".json")

	if data, err := os.ReadFile(cached); err == nil {
		var doc Document
		if err := json.Unmarshal(data, &doc); err == nil {
			return doc, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	doc, err := c.Inner.Recognize(path)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		return nil, fmt.Errorf("creating cache: %w", err)
	}
	if err := os.WriteFile(cached, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing cache: %w", err)
	}
	return doc, nil
}

func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

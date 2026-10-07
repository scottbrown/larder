package larder

import (
	"fmt"
	"runtime"
	"sync"
)

// LoadReceipts recognises and parses every file, skipping files whose
// content duplicates one already loaded. Results keep the order of files.
// When a receipt does not reconcile, each fallback recogniser is tried in
// turn and the first reading that does reconcile is kept.
func LoadReceipts(files []ReceiptFile, rec Recognizer, fallbacks ...Recognizer) ([]Receipt, error) {
	out := make([]Receipt, len(files))
	digests := make([]string, len(files))
	errs := make([]error, len(files))

	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, runtime.NumCPU()))
	for i, f := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			digest, err := fileDigest(f.Path)
			if err != nil {
				errs[i] = err
				return
			}
			doc, err := rec.Recognize(f.Path)
			if err != nil {
				errs[i] = err
				return
			}
			digests[i] = digest
			out[i] = ParseReceipt(f, doc)
			for _, fb := range fallbacks {
				if out[i].Status() == Reconciled {
					break
				}
				doc, err := fb.Recognize(f.Path)
				if err != nil {
					continue
				}
				if r := ParseReceipt(f, doc); r.Status() == Reconciled {
					out[i] = r
				}
			}
		}()
	}
	wg.Wait()

	seen := map[string]bool{}
	var receipts []Receipt
	for i := range files {
		if errs[i] != nil {
			return nil, fmt.Errorf("loading %s: %w", files[i].Path, errs[i])
		}
		if seen[digests[i]] {
			continue
		}
		seen[digests[i]] = true
		receipts = append(receipts, out[i])
	}
	return receipts, nil
}

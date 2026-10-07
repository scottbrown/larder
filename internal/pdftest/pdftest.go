// Package pdftest writes minimal receipt-like PDFs for tests.
package pdftest

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// Row is one printed receipt line: text on the left and an optional amount
// right-aligned in a second column, as a till prints them.
type Row struct {
	Text   string
	Amount string
}

const (
	pageWidth  = 300
	lineHeight = 22
	fontSize   = 13
)

// Receipt renders rows in a monospaced font on a narrow page.
func Receipt(rows ...Row) []byte {
	height := 60 + lineHeight*len(rows)
	var content strings.Builder
	for i, r := range rows {
		y := height - 30 - i*lineHeight
		fmt.Fprintf(&content, "BT /F1 %d Tf 16 %d Td (%s) Tj ET\n", fontSize, y, escape(r.Text))
		if r.Amount != "" {
			x := pageWidth - 16 - len(r.Amount)*8
			fmt.Fprintf(&content, "BT /F1 %d Tf %d %d Td (%s) Tj ET\n", fontSize, x, y, escape(r.Amount))
		}
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>", pageWidth, height),
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier-Bold >>",
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return buf.Bytes()
}

// WriteReceipt renders rows to path.
func WriteReceipt(path string, rows ...Row) error {
	return os.WriteFile(path, Receipt(rows...), 0o644)
}

func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(s)
}

package larder

import "testing"

func TestLinesGroupsCellsIntoRows(t *testing.T) {
	doc := Document{{Width: 600, Height: 600, Observations: []Observation{
		{Text: "8.99", X: 0.8, Y: 0.80, W: 0.1, H: 0.02},
		{Text: "39036 ROMAINE", X: 0.1, Y: 0.801, W: 0.4, H: 0.02},
		{Text: "SUBTOTAL", X: 0.1, Y: 0.70, W: 0.3, H: 0.02},
		{Text: "8.99", X: 0.8, Y: 0.70, W: 0.1, H: 0.02},
	}}}
	lines := doc.Lines()
	if len(lines) != 2 {
		t.Fatalf("got %d lines", len(lines))
	}
	if lines[0].Text() != "39036 ROMAINE 8.99" || lines[0].Last() != "8.99" {
		t.Errorf("first line = %q", lines[0].Text())
	}
	if (Line{}).Last() != "" {
		t.Error("empty line has a last cell")
	}
}

func TestLinesStraightenSkewedScans(t *testing.T) {
	// Each row falls by three text-heights from left to right, so a naive
	// grouping would pair each price with the row below its description.
	const slope = -0.06 // pixels of rise per pixel of run
	page := Page{Width: 600, Height: 600}
	for i, text := range []string{"ROMAINE", "TOMATO", "CUCUMBER"} {
		y := 0.8 - float64(i)*0.04
		page.Observations = append(page.Observations,
			Observation{Text: text, X: 0.05, Y: y, W: 0.3, H: 0.02, Slope: slope},
			Observation{Text: "1.99", X: 0.85, Y: y + slope*0.8, W: 0.1, H: 0.02, Slope: slope},
		)
	}
	lines := Document{page}.Lines()
	if len(lines) != 3 {
		t.Fatalf("got %d lines: %v", len(lines), lines)
	}
	for i, want := range []string{"ROMAINE 1.99", "TOMATO 1.99", "CUCUMBER 1.99"} {
		if lines[i].Text() != want {
			t.Errorf("line %d = %q, want %q", i, lines[i].Text(), want)
		}
	}
}

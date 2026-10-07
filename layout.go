package larder

import (
	"math"
	"sort"
	"strings"
)

// Cell is one recognised run of text within a Line.
type Cell struct {
	Text string
	X    float64
}

// Line is a row of text on a receipt, read left to right.
type Line struct {
	Cells []Cell
}

// Text joins the line's cells with single spaces.
func (l Line) Text() string {
	parts := make([]string, len(l.Cells))
	for i, c := range l.Cells {
		parts[i] = c.Text
	}
	return strings.Join(parts, " ")
}

// Last returns the right-most cell's text, or "" for an empty line.
func (l Line) Last() string {
	if len(l.Cells) == 0 {
		return ""
	}
	return l.Cells[len(l.Cells)-1].Text
}

// Lines arranges a document's observations into rows, top to bottom, page by
// page. Observations whose vertical centres lie within half a text height of
// each other share a row.
func (d Document) Lines() []Line {
	var out []Line
	for _, page := range d {
		out = append(out, page.lines()...)
	}
	return out
}

type row struct {
	mid    float64
	height float64
	cells  []Cell
}

// skew estimates the slope of the page's text lines in normalised units as
// the median slope of the wider runs of text, which Vision measures more
// reliably than short tokens.
func (p Page) skew() float64 {
	if p.Width == 0 || p.Height == 0 {
		return 0
	}
	var slopes []float64
	for _, o := range p.Observations {
		if o.W*p.Width >= 120 {
			slopes = append(slopes, o.Slope)
		}
	}
	if len(slopes) == 0 {
		return 0
	}
	sort.Float64s(slopes)
	return slopes[len(slopes)/2] * p.Width / p.Height
}

func (p Page) lines() []Line {
	obs := make([]Observation, len(p.Observations))
	copy(obs, p.Observations)
	slope := p.skew()
	// level returns the height an observation would sit at if the page were
	// straight, measured at the horizontal centre of the page.
	level := func(o Observation) float64 {
		return o.Y + o.H/2 - slope*(o.X+o.W/2-0.5)
	}
	sort.SliceStable(obs, func(i, j int) bool { return level(obs[i]) > level(obs[j]) })

	var rows []*row
	for _, o := range obs {
		mid := level(o)
		var best *row
		bestDist := math.Inf(1)
		// Only recent rows can match since observations arrive top-down.
		for i := len(rows) - 1; i >= 0 && i >= len(rows)-3; i-- {
			r := rows[i]
			dist := math.Abs(r.mid - mid)
			if dist < 0.5*math.Max(r.height, o.H) && dist < bestDist {
				best, bestDist = r, dist
			}
		}
		if best == nil {
			best = &row{mid: mid, height: o.H}
			rows = append(rows, best)
		}
		best.cells = append(best.cells, Cell{Text: strings.TrimSpace(o.Text), X: o.X})
	}

	out := make([]Line, len(rows))
	for i, r := range rows {
		sort.SliceStable(r.cells, func(a, b int) bool { return r.cells[a].X < r.cells[b].X })
		out[i] = Line{Cells: r.cells}
	}
	return out
}

package larder

// Observation is a run of text recognised on a page. Coordinates are
// normalised to the page with the origin at the bottom-left; Slope is the
// rise over run of the text's baseline in pixels, revealing skewed scans.
type Observation struct {
	Text  string  `json:"t"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Slope float64 `json:"s"`
}

// Page holds every observation recognised on one page, along with the
// page's rendered size in pixels.
type Page struct {
	Width        float64       `json:"width"`
	Height       float64       `json:"height"`
	Observations []Observation `json:"observations"`
}

// Document is the recognised text of a whole receipt.
type Document []Page

// Recognizer extracts text observations from a receipt file.
type Recognizer interface {
	Recognize(path string) (Document, error)
}

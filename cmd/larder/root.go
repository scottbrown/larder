package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/scottbrown/larder"
	"github.com/scottbrown/larder/internal/vision"
)

type globalOptions struct {
	receipts  string
	cacheDir  string
	catalogue string
	fromYear  int
	toYear    int
}

func newRootCommand() *cobra.Command {
	opts := &globalOptions{}
	cmd := &cobra.Command{
		Use:           "larder",
		Short:         "Find the groceries you buy regularly and track what they cost",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	cmd.PersistentFlags().StringVarP(&opts.receipts, "receipts", "r", os.Getenv("LARDER_RECEIPTS"), "directory of scanned receipts (env LARDER_RECEIPTS)")
	cmd.PersistentFlags().StringVar(&opts.cacheDir, "cache", defaultCacheDir(), "directory for cached text recognition")
	cmd.PersistentFlags().StringVarP(&opts.catalogue, "catalogue", "c", defaultCataloguePath(), "YAML catalogue naming and grouping products (env LARDER_CATALOGUE)")
	cmd.PersistentFlags().IntVar(&opts.fromYear, "from", 2019, "first year of receipts to include")
	cmd.PersistentFlags().IntVar(&opts.toYear, "to", time.Now().Year(), "last year of receipts to include")

	cmd.AddCommand(
		newScanCommand(opts),
		newOCRCommand(opts),
		newAuditCommand(opts),
		newItemsCommand(opts),
		newRegularsCommand(opts),
		newCatalogueCommand(opts),
		newDashboardCommand(opts),
	)
	return cmd
}

func defaultCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ".larder-cache"
	}
	return filepath.Join(dir, "larder")
}

func defaultCataloguePath() string {
	if p := os.Getenv("LARDER_CATALOGUE"); p != "" {
		return p
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "larder", "catalogue.yml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "catalogue.yml"
	}
	return filepath.Join(home, ".config", "larder", "catalogue.yml")
}

func (o *globalOptions) recognizer() larder.Recognizer {
	return larder.CachedRecognizer{Inner: vision.Recognizer{}, Dir: o.cacheDir}
}

func (o *globalOptions) findReceipts() ([]larder.ReceiptFile, error) {
	if o.receipts == "" {
		return nil, fmt.Errorf("no receipts directory: pass --receipts or set LARDER_RECEIPTS")
	}
	return larder.FindReceipts(o.receipts, larder.YearRange(o.fromYear, o.toYear))
}

func (o *globalOptions) loadReceipts() ([]larder.Receipt, error) {
	files, err := o.findReceipts()
	if err != nil {
		return nil, err
	}
	return larder.LoadReceipts(files, o.recognizer(), o.fallbacks()...)
}

// fallbacks re-read receipts that do not reconcile at other resolutions,
// since faint thermal print often reads correctly at one but not another.
func (o *globalOptions) fallbacks() []larder.Recognizer {
	var out []larder.Recognizer
	for _, dpi := range []int{300, 150} {
		out = append(out, larder.CachedRecognizer{
			Inner:   vision.Recognizer{DPI: dpi},
			Dir:     o.cacheDir,
			Variant: fmt.Sprintf("%ddpi", dpi),
		})
	}
	return out
}

// analysis is the parsed receipts and the products found in them.
type analysis struct {
	receipts  []larder.Receipt
	stats     []larder.ProductStats
	catalogue *larder.Catalogue
	today     time.Time
}

func (o *globalOptions) analyse() (analysis, error) {
	receipts, err := o.loadReceipts()
	if err != nil {
		return analysis{}, err
	}
	cat, err := larder.LoadCatalogue(o.catalogue)
	if err != nil {
		return analysis{}, err
	}
	today := time.Now()
	if last := lastReceiptDate(receipts); !last.IsZero() && o.toYear < today.Year() {
		today = last
	}
	stats := larder.Analyse(larder.Purchases(receipts), cat, larder.DefaultAnalysisOptions(today))
	return analysis{receipts: receipts, stats: stats, catalogue: cat, today: today}, nil
}

func lastReceiptDate(receipts []larder.Receipt) time.Time {
	var last time.Time
	for _, r := range receipts {
		if r.File.Date.After(last) {
			last = r.File.Date
		}
	}
	return last
}

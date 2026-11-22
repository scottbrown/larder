package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/scottbrown/larder"
)

func newDashboardCommand(opts *globalOptions) *cobra.Command {
	var out string
	var public bool
	var assets string
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Write a self-contained HTML dashboard of regular purchases",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := opts.analyse()
			if err != nil {
				return err
			}
			stats := a.stats
			if public {
				stats = larder.PublicStats(stats, a.catalogue)
			}
			data := larder.NewDashboardData(a.receipts, stats, larder.YearRange(opts.fromYear, opts.toYear), a.today, time.Now(), public)
			if cmd.Flags().Changed("assets") {
				err = writeDashboardFiles(out, data, larder.AssetPrefix(assets))
			} else {
				err = writeFile(out, func(w io.Writer) error { return larder.RenderDashboard(w, data) })
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%d regular products from %d receipts)\n", out, len(data.Products), data.Receipts)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "larder.html", "file to write the dashboard to")
	cmd.Flags().BoolVar(&public, "public", false, "leave out private categories and sensitive products, store codes and exact dates, for sharing publicly")
	cmd.Flags().StringVar(&assets, "assets", "", "write the data and script to "+larder.DashboardDataFile+" and "+larder.DashboardScriptFile+" next to the output, loaded from this URL prefix (such as /larder/), for sites whose Content Security Policy blocks inline scripts")
	return cmd
}

// writeDashboardFiles writes the page to out and its data and script files
// alongside it.
func writeDashboardFiles(out string, data larder.DashboardData, prefix larder.AssetPrefix) error {
	dir := filepath.Dir(out)
	return writeFile(out, func(page io.Writer) error {
		return writeFile(filepath.Join(dir, larder.DashboardDataFile), func(dataJS io.Writer) error {
			return writeFile(filepath.Join(dir, larder.DashboardScriptFile), func(script io.Writer) error {
				return larder.RenderDashboardFiles(larder.DashboardFiles{Page: page, Data: dataJS, Script: script}, data, prefix)
			})
		})
	})
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/scottbrown/larder"
)

func newDashboardCommand(opts *globalOptions) *cobra.Command {
	var out string
	var public bool
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
			f, err := os.Create(out)
			if err != nil {
				return err
			}
			if err := larder.RenderDashboard(f, data); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%d regular products from %d receipts)\n", out, len(data.Products), data.Receipts)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "larder.html", "file to write the dashboard to")
	cmd.Flags().BoolVar(&public, "public", false, "leave out private categories and sensitive products, store codes and exact dates, for sharing publicly")
	return cmd
}

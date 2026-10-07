package main

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/scottbrown/larder"
)

func newRegularsCommand(opts *globalOptions) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "regulars",
		Short: "List the products bought regularly, with cadence and price change",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := opts.analyse()
			if err != nil {
				return err
			}
			stats := a.stats
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PRODUCT\tCATEGORY\tSTORES\tTRIPS\tEVERY\tLAST\tSTATUS\tON SALE\tPRICE\tSHELF CHANGE\tPAID CHANGE")
			shown := 0
			for _, s := range stats {
				if !s.Regular {
					continue
				}
				if limit > 0 && shown >= limit {
					break
				}
				shown++
				price, change := "", ""
				if n := len(s.Yearly); n > 0 {
					price = s.Yearly[n-1].Shelf.String()
					if s.Unit == larder.Kilogram {
						price += "/kg"
					}
				}
				if s.HasPriceChange {
					change = fmt.Sprintf("%+.0f%% since %d", 100*s.PriceChange, s.PriceFrom.Year)
				}
				paid := ""
				if s.HasPaidChange {
					paid = fmt.Sprintf("%+.0f%% since %d", 100*s.PaidChange, s.PaidFrom.Year)
				}
				stores := make([]string, len(s.Stores))
				for i, st := range s.Stores {
					stores[i] = string(st)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%.0fd\t%s\t%s\t%.0f%%\t%s\t%s\t%s\n",
					s.Name, s.Category, strings.Join(stores, ","), s.Trips(), s.CadenceDays,
					s.Last.Format(time.DateOnly), s.Status, 100*s.SaleShare(), price, change, paid)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "show at most this many products")
	return cmd
}

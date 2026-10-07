package main

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/scottbrown/larder"
)

func newAuditCommand(opts *globalOptions) *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report how many receipts parse cleanly, by store",
		Long: "Compares the items parsed from each receipt with its printed subtotal.\n" +
			"A receipt is reconciled when they agree, a mismatch when they do not, and\n" +
			"unverified when no items or no subtotal could be read.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			receipts, err := opts.loadReceipts()
			if err != nil {
				return err
			}
			type tally struct{ total, reconciled, mismatch, unverified, items int }
			byStore := map[larder.StoreID]*tally{}
			for _, r := range receipts {
				t := byStore[r.File.Store.ID]
				if t == nil {
					t = &tally{}
					byStore[r.File.Store.ID] = t
				}
				t.total++
				t.items += len(r.Items)
				switch r.Status() {
				case larder.Reconciled:
					t.reconciled++
				case larder.Mismatch:
					t.mismatch++
				default:
					t.unverified++
				}
				if verbose && r.Status() != larder.Reconciled {
					fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s items=%s fees=%s adj=%s subtotal=%s\n",
						r.Status(), r.File.Path, r.ItemTotal(), r.Fees, r.Adjustments, r.Subtotal)
				}
			}
			ids := make([]string, 0, len(byStore))
			for id := range byStore {
				ids = append(ids, string(id))
			}
			sort.Strings(ids)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', tabwriter.AlignRight)
			fmt.Fprintln(w, "store\treceipts\treconciled\tmismatch\tunverified\titems\t")
			for _, id := range ids {
				t := byStore[larder.StoreID(id)]
				fmt.Fprintf(w, "%s\t%d\t%d (%.0f%%)\t%d\t%d\t%d\t\n", id, t.total, t.reconciled,
					100*float64(t.reconciled)/float64(t.total), t.mismatch, t.unverified, t.items)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "list every receipt that does not reconcile")
	return cmd
}

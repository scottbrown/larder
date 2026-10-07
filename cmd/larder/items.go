package main

import (
	"encoding/csv"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newItemsCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "items",
		Short: "Export every parsed line item as CSV",
		RunE: func(cmd *cobra.Command, _ []string) error {
			receipts, err := opts.loadReceipts()
			if err != nil {
				return err
			}
			w := csv.NewWriter(cmd.OutOrStdout())
			_ = w.Write([]string{"date", "store", "code", "description", "department",
				"quantity", "unit", "price", "discount", "paid", "on_sale", "status", "receipt"})
			for _, r := range receipts {
				for _, i := range r.Items {
					_ = w.Write([]string{
						r.File.Date.Format("2006-01-02"), string(r.File.Store.ID), string(i.Code),
						i.Description, i.Department,
						strconv.FormatFloat(i.Quantity.Amount, 'f', -1, 64), string(i.Quantity.Unit),
						fmt.Sprintf("%.2f", i.Price.Dollars()), fmt.Sprintf("%.2f", i.Discount.Dollars()),
						fmt.Sprintf("%.2f", i.Paid().Dollars()), strconv.FormatBool(i.OnSale()),
						string(r.Status()), r.File.Path,
					})
				}
			}
			w.Flush()
			return w.Error()
		},
	}
}

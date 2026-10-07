package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newOCRCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "ocr <receipt.pdf>...",
		Short: "Print the recognised text of receipts, one row per line",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rec := opts.recognizer()
			for _, path := range args {
				doc, err := rec.Recognize(path)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "== %s\n", path)
				for _, l := range doc.Lines() {
					cells := make([]string, len(l.Cells))
					for i, c := range l.Cells {
						cells[i] = c.Text
					}
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cells, " | "))
				}
			}
			return nil
		},
	}
}

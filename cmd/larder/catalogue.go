package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/scottbrown/larder"
)

func newCatalogueCommand(opts *globalOptions) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "catalogue",
		Short: "Print catalogue stubs for regular products not yet catalogued",
		Long: "Prints YAML entries for regular products the catalogue does not yet name,\n" +
			"most frequent first, ready to be edited and pasted into the catalogue file.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := opts.analyse()
			if err != nil {
				return err
			}
			stats := a.stats
			var stubs []larder.CatalogueEntry
			for _, s := range stats {
				if !s.Regular || s.Catalogued {
					continue
				}
				if limit > 0 && len(stubs) >= limit {
					break
				}
				stubs = append(stubs, larder.StubEntry(s.Name, s.Category, s.Keys...))
			}
			data, err := larder.MarshalCatalogue(stubs)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), string(data))
			return err
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "print at most this many stubs")
	return cmd
}

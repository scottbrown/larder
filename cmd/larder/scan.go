package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/spf13/cobra"
)

func newScanCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Recognise the text of every receipt, caching the results",
		RunE: func(cmd *cobra.Command, _ []string) error {
			files, err := opts.findReceipts()
			if err != nil {
				return err
			}
			rec := opts.recognizer()
			out := cmd.ErrOrStderr()
			live := isTerminal(out)

			paths := make(chan string)
			var mu sync.Mutex
			done, failed := 0, 0
			var wg sync.WaitGroup
			for range max(1, runtime.NumCPU()/2) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for p := range paths {
						_, err := rec.Recognize(p)
						mu.Lock()
						done++
						if err != nil {
							failed++
							fmt.Fprintf(out, "\n%v\n", err)
						}
						if live {
							fmt.Fprintf(out, "\rscanned %d/%d", done, len(files))
						}
						mu.Unlock()
					}
				}()
			}
			for _, f := range files {
				paths <- f.Path
			}
			close(paths)
			wg.Wait()
			if live {
				fmt.Fprintln(out)
			}
			fmt.Fprintf(out, "%d receipts scanned, %d failed\n", done, failed)
			return nil
		},
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

package main

import (
	"fmt"
	"os"

	"github.com/StatPan/gira/internal/gira"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	report, err := gira.RefreshDocsContract(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "refresh docs contract: %v\n", err)
		os.Exit(1)
	}
	for _, path := range report.Updated {
		fmt.Printf("refreshed %s\n", path)
	}
}

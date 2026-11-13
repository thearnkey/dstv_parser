package main

import (
	"encoding/json"
	"fmt"
	"os"

	dstv "github.com/thearnkey/dstv_parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dstv_parser FILE.nc1 [FILE.nc1 ...]")
		os.Exit(2)
	}
	type result struct {
		File string `json:"file"`
		*dstv.Part
		Cutting dstv.CuttingAssessment `json:"cutting"`
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, path := range os.Args[1:] {
		part, err := dstv.ParseFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
		if err := enc.Encode(result{path, part, part.Cutting()}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

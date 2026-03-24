package main

import (
	"flag"
	"log"
	"os"

	"bss/internal/places"
)

func main() {
	input := flag.String("input", "data/source/places.csv", "path to source CSV (;-separated) or JSON array of place objects")
	output := flag.String("output", "data/places.db", "path to output SQLite database (overwritten)")
	flag.Parse()

	logger := log.New(os.Stdout, "[build_db] ", log.LstdFlags)
	if err := places.BuildDatabase(*input, *output, logger); err != nil {
		logger.Fatal(err)
	}
}

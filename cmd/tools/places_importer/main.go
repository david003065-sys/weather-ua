package main

import (
	"flag"
	"log"
	"os"

	"bss/internal/places"
)

func main() {
	var inputPath string
	var outputPath string

	flag.StringVar(&inputPath, "input", "data/source/places.csv", "path to source CSV with settlements")
	flag.StringVar(&outputPath, "output", "data/places.db", "path to output SQLite database")
	flag.Parse()

	logger := log.New(os.Stdout, "[places_importer] ", log.LstdFlags)
	if err := places.BuildDatabase(inputPath, outputPath, logger); err != nil {
		log.Fatalf("places_importer: %v", err)
	}
}

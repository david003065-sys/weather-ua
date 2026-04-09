#!/bin/bash
set -e
# places.db збирається на старті сервера (bootstrap.EnsureData) або вручну:
#   go run ./cmd/tools/build_ua_cities_csv -geonames-dir data/geonames -out-dir data/out
#   go run ./cmd/build_db -input data/out/cities_ua.csv -output data/places.db
echo "Running tests..."
go test ./... -v
echo "Tests passed. Building..."
go build -o app ./cmd/server

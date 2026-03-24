#!/bin/sh
set -e
if [ ! -f data/places.db ]; then
  go run ./cmd/build_db -input "${PLACES_CSV:-data/source/places.csv}" -output data/places.db
fi
go build -o app ./cmd/server


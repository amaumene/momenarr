#!/bin/bash

# Script to recreate go.mod and go.sum files
# This ensures proper dependency resolution without creating go.mod in dependency repos

echo "Removing existing go.mod and go.sum..."
rm -f go.mod go.sum

echo "Initializing go.mod..."
go mod init github.com/amaumene/momenarr

echo "Adding replace directive for local momenarr..."
go mod edit -replace github.com/amaumene/momenarr=./

echo "Getting dependencies..."
# Add gostremiofr from latest commit
go get github.com/amaumene/gostremiofr@main

# Add other dependencies
go get github.com/cyruzin/golang-tmdb
go get github.com/sirupsen/logrus
go get go.etcd.io/bbolt

echo "Running go mod tidy..."
go mod tidy

echo "Done! go.mod and go.sum have been recreated."
echo ""
echo "Current dependencies:"
go list -m all | grep -E "gostremiofr|cyruzin|logrus|bbolt" | head -5
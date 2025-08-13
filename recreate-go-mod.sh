#!/bin/bash

echo "Removing existing go.mod and go.sum..."
rm -f go.mod go.sum

echo "Initializing go.mod..."
go mod init github.com/amaumene/momenarr

echo "Adding replace directive for local momenarr..."
go mod edit -replace github.com/amaumene/momenarr=./

echo "Getting dependencies..."
# Add gostremiofr from latest commit
go get github.com/amaumene/gostremiofr@main

echo "Running go mod tidy..."
go mod tidy

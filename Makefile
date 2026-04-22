SHELL := /usr/bin/env bash

.PHONY: build test unit integration

build:
	go build -o ./comot ./cmd/comot

unit:
	go test ./...

integration: build
	./scripts/test.sh

test:
	./scripts/test.sh

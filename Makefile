SHELL := /usr/bin/env bash

.PHONY: build test unit integration

build:
	go build -o ./comot ./cmd/comot

unit:
	go test ./...

integration: build
	bash ./scripts/test.sh

test:
	bash ./scripts/test.sh

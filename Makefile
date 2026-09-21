.DEFAULT_GOAL := test

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...

## tidy: tidy modfiles and modernize and format .go files
.PHONY: tidy
tidy:
	go mod tidy -v
	go fix ./...
	go fmt ./...

## run: run the command
.PHONY: run
run:
	go run ./

## build: build all binaries
.PHONY: build
build: tidy
	go build -o bin/hmm ./cmd/hmm

## ci: run all tests and build project
.PHONY: ci
ci: tidy test build

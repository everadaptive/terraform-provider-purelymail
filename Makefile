BINARY := terraform-provider-purelymail
GOBIN  := $(shell go env GOPATH)/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build install test lint fmt acceptance clean

build:
	go build -o bin/$(BINARY) .

install:
	go build -o $(GOBIN)/$(BINARY) .

test:
	go test ./...

lint:
	$(GOBIN)/golangci-lint run

fmt:
	gofmt -w .

acceptance:
	TF_ACC=1 go test ./internal/provider/ -v -run TestAcc

clean:
	rm -rf bin dist

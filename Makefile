PREFIX=github.com/kwkoo
PACKAGE=nutclient
GOOS=linux
GOARCH=arm
GOARM=5

GOPATH:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

build:
	@echo "Building..."
	@GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) go build -o $(PACKAGE) $(GOPATH)/.

PREFIX=github.com/kwkoo
PACKAGE=nutclient
GOOS=linux
GOARCH=arm
GOARM=5

GOPATH:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
GOBIN=$(GOPATH)/bin

build:
	@test -f $(GOPATH)/src/github.com/robbiet480/go.nut/nut.go || (echo "Initializing submodule..." && cd $(GOPATH) && git submodule init && git submodule update)
	@echo "Building..."
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) go build -o $(GOBIN)/$(PACKAGE) $(PREFIX)/$(PACKAGE)

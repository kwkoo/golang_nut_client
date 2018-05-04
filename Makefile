PACKAGE=nutclient
GOOS=linux
GOARCH=arm
GOARM=5

GOPATH:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
GOBIN=$(GOPATH)/bin

build:
	cd $(GOPATH) && git submodule init && git submodule update
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) go build -o $(GOBIN)/$(PACKAGE) $(PACKAGE)

SOURCEDIR="."
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')

BINARY=usage
BINARY_RELEASE=bin/${BINARY}
BINARY_VERSIONED_RELEASE=${BINARY_RELEASE}_${VERSION}

VERSION=$(shell cat VERSION)
OS=$(shell uname -s)
ARCH=$(shell uname -m)
OS_AND_ARCH="${OS}_${ARCH}"

.DEFAULT_GOAL: $(BINARY)

$(BINARY): bin_dir deps $(SOURCES)
	go build -o bin/${BINARY}

release: deps
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${BINARY_RELEASE}

releases: deps release_linux_x86_64 release_linux_armv7l release_darwin_x86_64

release_linux_x86_64: bin_dir
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${BINARY_VERSIONED_RELEASE}_Linux_x86_64

release_linux_armv7l: bin_dir
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -a -installsuffix cgo -o ${BINARY_VERSIONED_RELEASE}_armv7l

release_darwin_x86_64: bin_dir
	CGO_ENABLED=0 GOOS=darwin go build -a -installsuffix cgo -o ${BINARY_VERSIONED_RELEASE}_Darwin_x86_64

.PHONY: deps
deps:
	go get -d ./...

.PHONY: update_deps
update_deps:
	go get -u -d ./...

.PHONY: bin_dir
bin_dir:
	mkdir -p bin

.PHONY: run
run: deps
	go run main.go $(filter-out $@, $(MAKECMDGOALS))

.PHONY: clean
clean:
	rm -f ${BINARY} ${BINARY}_*

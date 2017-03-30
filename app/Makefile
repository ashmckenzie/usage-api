BINARY=usage_api
BINARY_RELEASE=release/${BINARY}
BINARY_VERSIONED_RELEASE=${BINARY_RELEASE}_${VERSION}

BUILD_IMAGE_NAME=usage-api-build
BUILD_DOCKERFILE=Dockerfile

RELEASE_DIR_NAME=release
RELEASE_DIR_REMOTE=/${RELEASE_DIR_NAME}
RELEASE_DIR_LOCAL=${PWD}/${RELEASE_DIR_NAME}
GOPATH_DIR=${PWD}/gopath
BUILD_ARGS=-i -ldflags="-X main.version=${VERSION}"

BUILD_USER=builder
CURRENT_USER_UID=1000

VERSION=$(shell cat VERSION)

default: bin_dir deps
	go build ${BUILD_ARGS} -o bin/${BINARY}

static: release_dir deps
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo ${BUILD_ARGS} -o ${BINARY_RELEASE}

release: create_build_image
	mkdir -p ${RELEASE_DIR_LOCAL} ${GOPATH_DIR}
	docker run --rm -v ${GOPATH_DIR}:/gopath -v ${RELEASE_DIR_LOCAL}:${RELEASE_DIR_REMOTE} -e BUILD_USER=${BUILD_USER} ${BUILD_IMAGE_NAME} make extract_releases

.PHONY: create_build_image
create_build_image:
	docker build --build-arg CURRENT_USER_UID=${CURRENT_USER_UID} --build-arg BUILD_USER=${BUILD_USER} -f ${BUILD_DOCKERFILE} -t ${BUILD_IMAGE_NAME} .	

.PHONY: deps
deps:
	go get -d ./...

.PHONY: update_deps
update_deps:
	go get -u -d ./...

.PHONY: release_dir
release_dir:
	mkdir -p release

.PHONY: bin_dir
bin_dir:
	mkdir -p bin

.PHONY: run
run: deps
	go run main.go $(filter-out $@, $(MAKECMDGOALS))

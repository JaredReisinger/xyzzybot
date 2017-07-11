.DEFAULT_GOAL := build

IMAGE_NAME := xyzzybot
IMAGE_VERSION := 0.1

build: build-dependencies generate
	go build -v .
.PHONY: build

build-dependencies:
	@# We really want to download and install dependencies, but *not*
	@# install xyzzybot itself.  'go get' doesn't give us that option
	@# easily, but we can use `-d` to download-but-don't-install, and then
	@# issue a monstrous `go list` to install *just* the dependencies.
	go get -v $$(go list -f '{{range .Deps}}{{printf "%s\n" .}}{{end}}' ./... | sort -u | grep -v "github.com/JaredReisinger/xyzzybot")
.PHONY: build-dependencies

generate:
	@# Sadly, the util sub-package has to be *installed* for go generate
	@# to work.  This seems unfortunate.
	go install -v ./util
	go generate -v ./interpreter
	go clean -i ./util
.PHONY: generate

acquire-external-tools:
	go get -u golang.org/x/tools/cmd/stringer
	go get -u github.com/kardianos/govendor
.PHONY: acquire-external-tools

install: build
	go install -v .
.PHONY: install

image:
	docker build \
		-t ${IMAGE_NAME}:${IMAGE_VERSION} \
		-t ${IMAGE_NAME}:latest \
		.
.PHONY: image

try: build
	./xyzzybot -config ./config/development.json
.PHONY: try

shell:
	docker run --rm \
		--tty \
		--interactive \
		--volume ${PWD}/config:/usr/local/etc/xyzzybot \
		--volume ${PWD}/sample-games:/usr/local/games \
		${IMAGE_NAME}:${IMAGE_VERSION}

# lint:
# 	go lint -v ./...
# .PHONY: lint

.DEFAULT_GOAL := build

build:
	go generate -v ./...
	@# go build -v ./...
	go build -v .
.PHONY: build

try: build
	./fizmo-slack -config ./config/development.json
.PHONY: try

# lint:
# 	go lint -v ./...
# .PHONY: lint

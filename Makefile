.DEFAULT_GOAL := foo

foo:
	go build -v .
.PHONY: foo

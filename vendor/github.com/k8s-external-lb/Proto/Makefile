all: build-go build-python build-example

build-go:
	./docker.sh sebassch/golang-protoc

build-python:
	./docker.sh sebassch/python-protoc

build-example:
	./create-server.sh

.PHONY: all

#!/bin/bash

export PATH=$PATH:$GOPATH/bin

protoc -I. external-loadbalancer.proto --go_out=plugins=grpc:
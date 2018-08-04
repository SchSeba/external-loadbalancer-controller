#!/bin/bash
go get github.com/golang/mock/gomock
go install github.com/golang/mock/mockgen
make test
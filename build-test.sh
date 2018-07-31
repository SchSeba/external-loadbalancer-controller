#!/bin/bash
goLocation=/go/src/github.com/k8s-external-lb/external-loadbalancer-controller/
docker run -it --rm -v $(pwd):${goLocation} -w ${goLocation} golang:1.10.3 ./run-test.sh
FROM golang

RUN go get -u google.golang.org/grpc
RUN go get -u github.com/golang/protobuf/protoc-gen-go

RUN apt-get update && apt-get install unzip

COPY install.sh /install.sh
COPY entrypoint.sh /entrypoint.sh

RUN /install.sh

CMD ["protoc", "-I.", "external-loadbalancer.proto", "--go_out=plugins=grpc:."]
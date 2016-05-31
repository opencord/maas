FROM golang:alpine

RUN apk --update add git

WORKDIR /go
RUN go get github.com/tools/godep
ADD . /go/src/github.com/ciena/cord-ip-allocator

WORKDIR /go/src/github.com/ciena/cord-ip-allocator
RUN /go/bin/godep restore

WORKDIR /go

RUN go install github.com/ciena/cord-ip-allocator

ENTRYPOINT ["/go/bin/cord-ip-allocator"]

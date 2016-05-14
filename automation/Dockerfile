FROM golang:alpine

RUN apk --update add git

WORKDIR /go
RUN go get github.com/ciena/cord-maas-automation

ENTRYPOINT ["/go/bin/cord-maas-automation"]

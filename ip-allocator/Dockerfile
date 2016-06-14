FROM golang:alpine

RUN apk --update add git

WORKDIR /go
RUN go get github.com/tools/godep
ADD . /go/src/gerrit.opencord.org/maas/cord-ip-allocator

WORKDIR /go/src/gerrit.opencord.org/maas/cord-ip-allocator
RUN /go/bin/godep restore

WORKDIR /go

RUN go install gerrit.opencord.org/maas/cord-ip-allocator

ENTRYPOINT ["/go/bin/cord-ip-allocator"]

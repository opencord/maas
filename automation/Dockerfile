FROM golang:1.6-alpine

RUN apk --update add openssh-client git

WORKDIR /go
RUN go get github.com/tools/godep
ADD . /go/src/gerrit.opencord.org/maas/cord-maas-automation

WORKDIR /go/src/gerrit.opencord.org/maas/cord-maas-automation
RUN /go/bin/godep restore || true

WORKDIR /go
RUN go install gerrit.opencord.org/maas/cord-maas-automation

ENTRYPOINT ["/go/bin/cord-maas-automation"]

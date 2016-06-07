FROM golang:alpine

RUN apk --update add openssh-client git

WORKDIR /go
RUN go get github.com/tools/godep
ADD . /go/src/github.com/ciena/cord-maas-automation

WORKDIR /go/src/github.com/ciena/cord-maas-automation
RUN /go/bin/godep restore

WORKDIR /go

RUN go install github.com/ciena/cord-maas-automation

RUN mkdir -p /root/.ssh
COPY ssh-config /root/.ssh/config

ENTRYPOINT ["/go/bin/cord-maas-automation"]

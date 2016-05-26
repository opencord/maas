FROM ubuntu:14.04

RUN apt-get update -y && \
    sudo apt-get install -y  software-properties-common && \
    apt-add-repository ppa:ansible/ansible && \
    apt-get update -y && \
    apt-get install -y git ansible golang

RUN mkdir /go
WORKDIR /go
ENV GOPATH=/go
RUN go get github.com/tools/godep
ADD . /go/src/github.com/ciena/cord-maas-automation

WORKDIR /go/src/github.com/ciena/cord-maas-automation
RUN /go/bin/godep restore

WORKDIR /go
RUN go install github.com/ciena/cord-maas-automation

ENTRYPOINT ["/go/bin/cord-maas-automation"]

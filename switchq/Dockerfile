## Copyright 2016 Open Networking Laboratory
##
## Licensed under the Apache License, Version 2.0 (the "License");
## you may not use this file except in compliance with the License.
## You may obtain a copy of the License at
##
## http://www.apache.org/licenses/LICENSE-2.0
##
## Unless required by applicable law or agreed to in writing, software
## distributed under the License is distributed on an "AS IS" BASIS,
## WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
## See the License for the specific language governing permissions and
## limitations under the License.
FROM ubuntu:14.04
MAINTAINER Open Networking Labratory <info@onlab.us>

# Base image information borrowed by official golang wheezy Dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
		g++ \
		gcc \
		libc6-dev \
		make \
                curl \
	&& rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.6.2
ENV GOLANG_DOWNLOAD_URL https://golang.org/dl/go$GOLANG_VERSION.linux-amd64.tar.gz
ENV GOLANG_DOWNLOAD_SHA256 e40c36ae71756198478624ed1bb4ce17597b3c19d243f3f0899bb5740d56212a

RUN curl -kfsSL "$GOLANG_DOWNLOAD_URL" -o golang.tar.gz \
	&& echo "$GOLANG_DOWNLOAD_SHA256  golang.tar.gz" | sha256sum -c - \
	&& tar -C /usr/local -xzf golang.tar.gz \
	&& rm golang.tar.gz

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

# CORD Provisioner Dockerfile
WORKDIR $GOPATH

RUN apt-get update && \
	apt-get install -y  software-properties-common && \
	apt-add-repository ppa:ansible/ansible && \
	apt-get update -y  -m && \
	apt-get install -y git ansible

RUN mkdir -p /root/.ssh
COPY ssh-config /root/.ssh/config

RUN mkdir -p /switchq
COPY vendors.json /switchq/vendors.json

RUN go get github.com/tools/godep
ADD . $GOPATH/src/gerrit.opencord.com/maas/switchq

WORKDIR $GOPATH/src/gerrit.opencord.com/maas/switchq
RUN $GOPATH/bin/godep restore || true

WORKDIR $GOPATH
RUN go install gerrit.opencord.com/maas/switchq

ENTRYPOINT ["/go/bin/switchq"]

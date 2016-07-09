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
FROM golang:alpine

MAINTAINER Open Networking Labratory <info@onlab.us>

RUN apk --update add git

# install godep
WORKDIR /go
RUN go get github.com/tools/godep
ADD . /go/src/gerrit.opencord.org/maas/config-generator

# restore dependancies
WORKDIR /go/src/gerrit.opencord.org/maas/config-generator
RUN /go/bin/godep restore || true

# generate the binary
WORKDIR /go/
RUN go install gerrit.opencord.org/maas/config-generator

# copy templates to the image
COPY hosts.tpl /go/bin/
COPY links.tpl /go/bin/
COPY ports.tpl /go/bin/
COPY devices.tpl /go/bin/

EXPOSE 1337

ENTRYPOINT ["/go/bin/config-generator"]

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
MAINTAINER Open Networking Laboratory <info@onlab.us>

RUN apk --update add git

WORKDIR /go
RUN go get github.com/tools/godep
ADD . /go/src/gerrit.opencord.org/maas/cord-ip-allocator

WORKDIR /go/src/gerrit.opencord.org/maas/cord-ip-allocator
RUN /go/bin/godep restore || true

WORKDIR /go

RUN go install gerrit.opencord.org/maas/cord-ip-allocator

LABEL org.label-schema.name="allocator" \
      org.label-schema.description="Provides IP address allocation for fabric interfaces" \
      org.label-schema.vcs-url="https://gerrit.opencord.org/maas" \
      org.label-schema.vendor="Open Networking Labratory" \
      org.label-schema.schema-version="1.0"

ENTRYPOINT ["/go/bin/cord-ip-allocator"]

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
MAINTAINER Open Networking Laboratory <info@onlab.us>

RUN	apt-get update -y && \
	apt-get install -y python-pip

RUN	pip install maasclient==0.3 && \
	pip install requests_oauthlib && \
        pip install ipaddress

ADD bootstrap.py /bootstrap.py

LABEL org.label-schema.name="bootstrap" \
      org.label-schema.description="Provides bootstrap configuration of MAAS for the CORD deployment" \
      org.label-schema.vcs-url="https://gerrit.opencord.org/maas" \
      org.label-schema.vendor="Open Networking Labratory" \
      org.label-schema.schema-version="1.0"

ENTRYPOINT [ "/bootstrap.py" ]

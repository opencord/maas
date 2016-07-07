FROM ubuntu:14.04

RUN apt-get update && apt-get install --no-install-recommends -y \
    ca-certificates \
    curl \
    mercurial \
    git-core
RUN curl -s https://storage.googleapis.com/golang/go1.6.2.linux-amd64.tar.gz | tar -v -C /usr/local -xz

ENV GOPATH /go
ENV GOROOT /usr/local/go
ENV PATH /usr/local/go/bin:/go/bin:/usr/local/bin:$PATH

WORKDIR /app

# copy binary into image
COPY app /app/
COPY hosts.tpl /app/
COPY links.tpl /app/
COPY ports.tpl /app/
COPY devices.tpl /app/

VOLUME /app


# ENTRYPOINT ["./app"]
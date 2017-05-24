FROM golang:alpine
MAINTAINER Gerry Gleason && Christopher Reay

RUN apk add --update \
      git \
      make \
      sudo \
    && rm -rf /var/cache/apk/*
# UID argument specified by docker build --build-arg uid=<uid> and defaults to 1000
ARG uid=${UID_MIN:-1000}
RUN adduser -D -u $uid holochain \
&& addgroup holochain wheel \
&& sed -i -e'/%wheel ALL=(ALL) NOPASSWD/s/# //' /etc/sudoers

# Cache dependencies of holochain
  RUN go get -v -d github.com/metacurrency/holochain
  WORKDIR ${GOPATH}/src/github.com/metacurrency/holochain
  RUN make deps \
  && rm -rf ${GOPATH}/src/github.com/metacurrency/holochain

# Use checked out version of holochain
COPY . ${GOPATH}/src/github.com/metacurrency/holochain
RUN make \
&& rm -rf ${GOPATH}/pkg ${GOPATH}/src/* ${GOPATH}/bin/gx*

EXPOSE 3141

USER holochain

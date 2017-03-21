FROM golang:1.7.5-alpine
MAINTAINER Duke Dorje && DayZee

RUN apk add --update \
      ca-certificates \
      curl wget \
      curl-dev \
      procps \
      openrc \
      git \ 
      make \
    && rm -rf /var/cache/apk/* \
    && addgroup holochain -g 868 \
    && adduser -G holochain -u 868 -D holochain \
    && mv /etc/profile.d/color_prompt /etc/profile.d/color_prompt.sh

ENV GOPATH=/app/golang
ENV PATH=$GOPATH/bin:$PATH

RUN go get -v -d github.com/metacurrency/holochain \
    && cd /app/golang/src/github.com/metacurrency/holochain \
    && make deps \
    && chown -R holochain /app

WORKDIR /app/golang/src/github.com/metacurrency/holochain

USER holochain

COPY . /app/golang/src/github.com/metacurrency/holochain

CMD ["make", "test" ]

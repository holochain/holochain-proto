FROM ubuntu
MAINTAINER Duke Dorje && Dayz

RUN apt-get update
RUN apt-get install -y build-essential software-properties-common python curl wget git-core 

RUN wget -q https://storage.googleapis.com/golang/go1.8.linux-amd64.tar.gz -O golang.tar.gz
RUN tar -zxvf golang.tar.gz -C /usr/local/
RUN mkdir /golang
ENV GOPATH /golang
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

RUN go get -v -u github.com/whyrusleeping/gx
RUN rm $GOPATH/src/github.com/ethereum/tests -r

RUN go get -v -u github.com/metacurrency/holochain


WORKDIR $GOPATH/src/github.com/metacurrency/holochain

RUN make
RUN make bs

RUN make test

#ADD . $GOPATH/src/github.com/metacurrency/holochain

#CMD ["/usr/bin/node", "/var/www/app.js"]

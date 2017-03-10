FROM metacurrency/holochain

WORKDIR $GOPATH/src/github.com/metacurrency/holochain

ADD . $GOPATH/src/github.com/metacurrency/holochain

RUN make
RUN make bs

RUN make test

EXPOSE 3142
CMD ["bs"]
#CMD ["/usr/bin/node", "/var/www/app.js"]

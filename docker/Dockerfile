#
# Dockerfile using a multi-stage build
#

# Stage 0 (build) - Build the holochain tools

FROM golang as build

run go get -d -v github.com/metacurrency/holochain
run cd $GOPATH/src/github.com/metacurrency/holochain && make

# Stage 1 - Copy holochain tools into a minimal image

FROM debian:buster-slim

COPY --from=build /go/bin/bs /usr/local/bin/bs
COPY --from=build /go/bin/hcadmin /usr/local/bin/hcadmin
COPY --from=build /go/bin/hccore /usr/local/bin/hccore
COPY --from=build /go/bin/hcd /usr/local/bin/hcd
COPY --from=build /go/bin/hcdev /usr/local/bin/hcdev

CMD ["/bin/bash"]

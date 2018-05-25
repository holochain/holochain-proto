FROM golang
# UID argument specified by docker build --build-arg uid=<uid> and defaults to 1000
ARG uid=${UID_MIN:-1000}

# Install packages, cache dependencies of holochain
RUN apt-get update && apt-get install -y \
  git \
  make \
  sudo \
  && go get -v -d github.com/holochain/holochain-proto \
  && make -C "${GOPATH}/src/github.com/holochain/holochain-proto" deps \
  && rm -rf ${GOPATH}/src/github.com/holochain/holochain-proto
# Use checked out version of holochain
COPY . ${GOPATH}/src/github.com/holochain/holochain-proto
RUN make -C "${GOPATH}/src/github.com/holochain/holochain-proto"

RUN useradd -mUu $uid holochain \
&& adduser holochain sudo \
&& sed -i -e'/%sudo/s/(ALL:ALL) ALL/(ALL:ALL) NOPASSWD: ALL/' /etc/sudoers
USER holochain

EXPOSE 3141

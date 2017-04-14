FROM golang:1.7.5-alpine
MAINTAINER Gerry Gleason && Christopher Reay

# Install dependencies required for the make process but not needed afterwards
RUN apk update && apk add --update -t .build-deps \
    git \
    make \
# Install programs needed for using the container
&& apk add --update \
    sudo \
    su-exec \
# Clear the apk cache to reduce image size
&& rm -rf /var/cache/apk/*
RUN addgroup holochain -g 868 \
&& adduser -G holochain -u 868 -D holochain \
&& addgroup holochain wheel \
&& passwd -u holochain \
&& sed -i~orig -e'/ALL) ALL/s/# %wheel/%wheel/' /etc/sudoers

# Copy the host's holochain repo into the container
COPY . /go/src/github.com/metacurrency/holochain

# Install the hc command
RUN make -C /go/src/github.com/metacurrency/holochain hc \
# Install the bs command
&& make -C /go/src/github.com/metacurrency/holochain bs \
# Uninstall the now unecessary dependencies
&& apk del -r .build-deps \
# Uninstall go dependencies
&& rm -rf /go/src/* && rm -rf /go/pkg \
# Uninstall gx and gx-go
&& rm -rf /go/bin/gx && rm -rf /go/bin/gx-go

EXPOSE 3141

# sort out dynamic user login
COPY docker/entrypoint.sh.addHostUserToContainer /usr/local/bin/entrypoint.sh.addHostUserToContainer
ENTRYPOINT ["/usr/local/bin/entrypoint.sh.addHostUserToContainer"]


#CMD ["su", "holochain", "-c", "source /etc/holochain.env_vars; make test" ]
#CMD ["/bin/sh"]

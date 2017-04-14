FROM golang:1.7.5-alpine
MAINTAINER Gerry Gleason && Christopher Reay

RUN apk update && apk add --update \
    ca-certificates \
    curl wget \
    curl-dev \
    procps \
    openrc \
    git \
    make \
    sudo \
    su-exec \
&& rm -rf /var/cache/apk/* \
&& chmod +s /usr/bin/passwd
RUN addgroup holochain -g 868 \
&& adduser -G holochain -u 868 -D holochain \
&& addgroup holochain wheel \
&& passwd -u holochain \
&& sed -i~orig -e'/ALL) ALL/s/# %wheel/%wheel/' /etc/sudoers \
&& mv /etc/profile.d/color_prompt /etc/profile.d/color_prompt.sh

# Copy the host's holochain repo into the container
COPY . /go/src/github.com/metacurrency/holochain

# Install the hc command
RUN make -C /go/src/github.com/metacurrency/holochain hc \
# Install the bs command
&& make -C /go/src/github.com/metacurrency/holochain bs

EXPOSE 3141

# sort out dynamic user login
COPY docker/entrypoint.sh.addHostUserToContainer /usr/local/bin/entrypoint.sh.addHostUserToContainer
ENTRYPOINT ["/usr/local/bin/entrypoint.sh.addHostUserToContainer"]


#CMD ["su", "holochain", "-c", "source /etc/holochain.env_vars; make test" ]
#CMD ["/bin/sh"]

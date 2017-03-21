FROM golang:1.7.5-alpine
MAINTAINER Gerry Gleason

WORKDIR /work

  RUN apk add --update \
      ca-certificates \
      curl wget \
      curl-dev \
      procps \
      openrc \
      git \ 
      make \
    && rm -rf /var/cache/apk/* \
    \
    && addgroup developer -g 868 \
    && adduser -g 'Standard developer user.' -u 868 -G developer -h /home/developer -s /bin/sh -D developer \
    && mkdir -p ~developer/.ssh \
    && touch /tmp/id_rsa.developer  \
    && mv /tmp/id_rsa.developer ~developer/.ssh/id_rsa \
    && chown -R developer. ~developer/ /work \ 
    && chmod 700 ~developer/.ssh \
    && chmod 600 ~developer/.ssh/id_rsa \
    && mv /etc/profile.d/color_prompt /etc/profile.d/color_prompt.sh \
    && sed -i -e 's#/bin/ash#/bin/sh#' /etc/passwd \
    && touch /etc/developer.env_vars \
    && mkdir /apps \
    && ln -s /home/developer/holochain /apps/holochain \
    && mkdir /home/developer/golang \
    && mkdir /home/developer/bin \
    && chown developer ~developer/golang ~developer/bin; \
    \
    echo 'export GOPATH=/work/golang' >> /etc/developer.env_vars; \
    echo 'export GOBIN=/home/developer/bin' >> /etc/developer.env_vars; \
    echo 'export PATH=$GOPATH/bin:/usr/local/go/bin:$GOBIN:$PATH' >> /etc/developer.env_vars; \
    su - developer -c 'source /etc/developer.env_vars; \
    git clone https://github.com/metacurrency/holochain ~developer/holochain \
    && go get -v -u github.com/whyrusleeping/gx \
    && go get -v -u github.com/metacurrency/holochain; \
    cd ~developer/holochain \
    && (make; true) && make bs'

# note that on xhyve (MacOs and Windows), volume access can be slow
VOLUME /home
# If you do have it on a volume, but not one exported from xhyve, you can bind mount and
# backup data from a container's volume like this:
#sudo docker run --rm --volumes-from my_container -v ~/container_backups/:/backup busybox tar cvzf /backup/contentservices_back.tgz home/webapp

# run tests in a container:
# docker run -t holochain:developer su - developer -c "source /etc/developer.env_vars; cd ~developer/holochain; time go get -t; time go test -v ./... || exit 1"

##CMD ["/usr/bin/node", "/var/www/app.js"]
#
CMD ["/bin/sh"]

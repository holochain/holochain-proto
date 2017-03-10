FROM metacurrency/holochain

ADD . /clutter

RUN hc init $HOSTNAME
RUN hc clone /clutter clutter
RUN hc gen chain clutter

EXPOSE 3141
CMD ["hc", "serve", "clutter"]


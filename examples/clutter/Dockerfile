FROM metacurrency/holochain

ADD . /clutter
WORKDIR /clutter

#RUN hc init $HOSTNAME
#RUN hc join /clutter clutter
#RUN hc gen chain clutter



EXPOSE 3141
CMD ["/clutter/Scripts/chain.joinAndServe"]


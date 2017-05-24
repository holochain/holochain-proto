#!/bin/sh
if id -nG "$USER" | grep -qw "docker"; then
        docker="docker"
else
        docker="sudo docker"
fi
$docker run --rm -p 3141 -it metacurrency/holochain sh

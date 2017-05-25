#!/bin/sh
if id -nG "$USER" | grep -qw "docker"; then
        docker="docker"
else
        docker="sudo docker"
fi
$docker run --rm -p 3141:3141 -it hc-dev-tools:jessie bash

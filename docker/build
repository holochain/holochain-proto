#!/bin/sh
if id -nG "$USER" | grep -qw "docker"; then
        docker="docker"
else
        docker="sudo docker"
fi
$docker build -f docker/Dockerfile -t metacurrency/holochain .

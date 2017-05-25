#!/bin/sh
name="hc-dev-tools"
tag="jessie"
if id -nG "$USER" | grep -qw "docker"; then
        docker="docker"
else
        docker="sudo docker"
fi
# If id command is installed then use it to mimick the id, otherwise don't.
if [ -n $(command -v id) ]; then
        $docker build -f "docker/$name/Dockerfile.$tag" -t "$name:$tag" --build-arg uid=$(id -u) .
else
        $docker build -f "docker/$name/Dockerfile.$tag" -t "$name:$tag" .
fi

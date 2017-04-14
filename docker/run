#!/bin/sh
if id -nG "$USER" | grep -qw "docker"; then
        docker="docker"
else
        docker="sudo docker"
fi
$docker run -p=3141:3141 -v ~/.holochain:/home/user/.holochain -e LOCAL_USER_ID=`id -u $USER`  -Pit metacurrency/holochain /bin/sh

# Holochain Docker Usage
This document provides a description and usage instructions for the different use-cases of Holochain Docker images.

TODO: This file and the hc-dev-tool scripts are out of data, and need to be updated.

<!-- TOC depthFrom:2 depthTo:6 withLinks:1 updateOnSave:1 orderedList:0 -->

- [Docker use-cases](#docker-use-cases)
	- [Multi-Node Tests](#multi-node-tests)
	- [Development](#development)
	- [Packaging Holochain Apps for End-Users](#packaging-holochain-apps-for-end-users)
- [Images](#images)
	- [metacurrency/holochain](#metacurrencyholochain)
	- [hc-dev-tools](#hc-dev-tools)

<!-- /TOC -->

## Docker use-cases
This is a list of use-cases for Docker in the context of Holochain. Once you have found the Docker image for your needs, scroll down to the relevant image section below.

### Development
Docker can also be used for doing development inside of containers. This can be helpful for developers who are unable to (or don't want to) run Holochain directly on their system.

For this use-case, one of the [hc-dev-tools](#hc-dev-tools) images should be built and run interactively.

### Packaging Holochain Apps for End-Users
Docker can also be used for distributing a Holochain application as an image.

For this use-case one of the [metacurrency/holochain](#metacurrencyholochain) images should be used as the base of a custom image for your project.

## Images
This is a list of Docker images [published by holochain](https://hub.docker.com/u/metacurrency/) as well as instructions on how to use them.

### metacurrency/holochain
These images are based on the official [`golang`](https://hub.docker.com/_/golang/) with the addition of installing the `hc` command.

In order to use these images place their tag in the `FROM` statement of your Dockerfile.

Example of an image which distributes a holochain app:
```Dockerfile
FROM metacurrency/holochain

RUN hcadmin init address@example.org

ENV appdir="/home/holochain/app"

COPY . ${appdir}
# Files when first copied are owned by root.
RUN sudo chown -R holochain:holochain ${appdir}/.. \
\
&& hc clone ${appdir} MYAPPNAMEHERE \
&& rm -rf ${appdir} \
&& hc gen chain MYAPPNAMEHERE

CMD ["hc", "web", "MYAPPNAMEHERE"]
```

### hc-dev-tools
These images are basically the official `golang` images except with the `hc` command and a few other tools installed.

In order to build the latest version of this image, run the following commands from the root of the [Holochain](https://github.com/metacurrency/holochain) repository:
* Build it using the `docker/build`
* Run it using the `docker/run`

# Holochain

[![Code Status](https://img.shields.io/badge/Code-Alpha-yellow.svg)](https://github.com/metacurrency/holochain/milestones?direction=asc&sort=completeness&state=all)
[![Travis](https://img.shields.io/travis/metacurrency/holochain/master.svg)](https://travis-ci.org/metacurrency/holochain/branches)
[![Codecov](https://img.shields.io/codecov/c/github/metacurrency/holochain.svg)](https://codecov.io/gh/metacurrency/holochain/branch/master)
[![Go Report Card](https://goreportcard.com/badge/github.com/metacurrency/holochain)](https://goreportcard.com/report/github.com/metacurrency/holochain)
[![Gitter](https://badges.gitter.im/metacurrency/holochain.svg)](https://gitter.im/metacurrency/holochain?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=body_badge)
[![In Progress](https://img.shields.io/waffle/label/metacurrency/holochain/in%20progress.svg)](http://waffle.io/metacurrency/holochain)
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0)
[![Twitter Follow](https://img.shields.io/twitter/follow/holochain.svg?style=social&label=Follow)](https://twitter.com/holochain)

**Holographic storage for distributed applications.** A holochain is a monotonic distributed hash table (DHT) where every node enforces validation rules on data before publishing that data against the signed chains where the data originated.

In other words, a holochain functions very much **like a blockchain without bottlenecks** when it comes to enforcing validation rules, but is designed to  be fully distributed with each node only needing to hold a small portion of the data instead of everything needing a full copy of a global ledger. This makes it feasible to run blockchain-like applications on devices as lightweight as mobile phones.

**[Code Status:](https://github.com/metacurrency/holochain/milestones?direction=asc&sort=completeness&state=all)** Alpha. Not for production use. The code has not yet undergone a security audit. We expect to destructively restructure code APIs and data chains until Beta. Proof-of-concept was unveiled at our first hackathon (March 2017). Alpha 0 was released (October 2017).
<br/>

| Holochain Links: | [FAQ](https://github.com/metacurrency/holochain/wiki/FAQ) | [Developer Wiki](https://github.com/metacurrency/holochain/wiki) | [White Paper](https://github.com/metacurrency/holochain/blob/whitepaper/holochain.pdf) | [GoDocs](https://godoc.org/github.com/metacurrency/holochain) |
|---|---|---|---|---|

**Table of Contents**

<!-- TOC depthFrom:2 depthTo:4 withLinks:1 updateOnSave:1 orderedList:0 -->

- [Installation](#installation)
	- [Go Based Install](#go-based-install)
		- [Unix](#unix)
		- [Windows](#windows)
	- [Docker Based Install](#docker-based-install)
- [Usage](#usage)
	- [Getting Started](#getting-started)
	  - [Initializing the Holochain environment](#initializing-the-holochain-environment)
	  - [Joining a Holochain](#joining-a-holochain)
	  - [Running a Holochain](#running-a-holochain)
	- [Developing a Holochain](#developing-a-holochain)
	  - [Test-driven Application Development Locations](#test-driven-application-development)
	  - [File Locations](#file-locations)
	  - [Logging](#logging)
	- [Multi-instance Integration Testing](#multi-instance-integration-testing)
- [Architecture Overview and Documentation](#architecture-overview-and-documentation)
- [Holochain Core Development](#development)
	- [Contribute](#contribute)
	- [Dependencies](#dependencies)
	- [Tests](#tests)
- [License](#license)
- [Acknowledgements](#acknowledgements)

<!-- /TOC -->

## Installation
**Developers Only:** At this stage, holochain is only for use by developers (either developers of applications to run on holochains, or developers of the holochain software itself). App developers should bundle their app in an installer as either approach below is not for non-technical folks.

There are two approaches to installing holochain:
1. as a standard Go language application for direct execution on your machine
2. using [docker](https://www.docker.com/) for execution in a container.

Which you choose depends on your preference and your purpose.  If you intend to develop holochain applications, then you should almost certainly use the docker approach as we provide a testing harness for running multiple holochain instances in a docker cluster.  If you will be developing in Go on holochain itself then you will probably end up doing both.

### Go Based Install

#### Unix
(Unix includes macOS and Linux.)

1. [Download Go](https://golang.org/dl/). Download the "Archive" or "Installer" for version 1.8 or later for your CPU and OS. The "Source" download does not contain an executable and step 3 will fail.
2. [Install Go](https://golang.org/doc/install) on your system.  See platform specific instructions and hints below for making this work.
3. Setup your path (Almost all installation problems that have been reported stem from skipping this step.)

    * Export the `$GOPATH` variable in your shell profile.
    * Add `$GOPATH/bin` to your `$PATH` in your shell profile.

    For example, add the following to the end of your shell profile (usually `~/.bashrc` or `~/.bash_profile`):
````bash
        export GOPATH="$HOME/go"
        export PATH="$GOPATH/bin:$PATH"
````

4. Install the command line tool suite with:

```bash
$ go get -d -v github.com/metacurrency/holochain
$ cd $GOPATH/src/github.com/metacurrency/holochain
$ make
```

5. Test that it works (should look something like this):

```bash
$ hcadmin -v
hcadmin version 0.0.x (holochain y)
```

#### Windows
First you'll need to install some necessary programs if you don't already have them.
* [Download Go](https://golang.org/dl/). Download the "Archive" or "Installer" for version 1.8 or later for Windows and your CPU. The "Source" download does not contain an executable.
* [Install Windows git](https://git-scm.com/downloads). Be sure to select the appropriate options so that git is accessible from the Windows command line.
* Optional: [Install GnuWin32 make](http://gnuwin32.sourceforge.net/packages/make.htm#download).

Next, in your Control Panel, select *System>Advanced system settings>Environment Variables...* and under *System Variables* do the following:
1. Add a new entry with the name `GOPATH` and the value `%USERPROFILE%\go` (Or your Go workspace folder).
2. Double-click Path, and in the window that pops up add the following entries:
    - `%GOPATH%\bin`
    - `C:\Go\bin` (Or wherever you installed Go to+`\bin`).
    - `C:\Program Files (x86)\GnuWin32\bin` (Or wherever you installed GnuWin32 make to+`\bin`).

### Docker Based Install
Using docker, you don't have to install Go first. Our docker scripts manage installation of Go, holochain dependencies and holochain. The docker installation can run alongside Local ("Go") installation of holochain, sharing config directories.  See [docker usage](https://github.com/metacurrency/holochain/wiki/Docker-Usage) on our wiki for more on how this works.

1. Install the latest version of Docker on your machine
    1. [Docker Installation](https://docs.docker.com/engine/installation/). The Community edition; stable is sufficient.
    2. See [Docker Getting Started](https://docs.docker.com/engine/getstarted/step_one/) for help.
    3. It is recommended to add your user to the `docker` group as in: [Post Installation Steps](https://docs.docker.com/engine/installation/linux/linux-postinstall/), rather than use `sudo` before all script commands. Holochain Apps cannot exploit the kinds of security concerns mentioned in the Post Installation Steps document.
&nbsp;
2. Confirm that docker installation and permissions are working by running:
```bash
$ docker info
```

3. Get our holochain repository from github:
```bash
$ git clone https://github.com/metacurrency/holochain.git holochain
$ cd holochain
```
4. Build the holochain core with all dependencies
```bash
$ docker/build
```
  * the first time build is run on a machine, it will download Alpine unix, and install all holochain dependencies.
  * During development cycles, docker/build will just compile changes made to the holochain go code, and run tests

5. To run holochain in your new environment, suitable to continue the walkthrough below in [usage](#usage)
```bash
$ docker/run
```
6. This will put you into an new command shell that may behave differently than what you're used to. To exit this holochain (Alpine) shell, press `Ctrl-D` or type `exit`

## Usage
These instructions are for using the holochain command line tool suite: `hcadmin`, `hcdev` and `hcd`.  They should work equally well for Go based or docker based installation.

(Note that since Holochain is intended to be used behind distributed applications, end users should not have to do much through the command or may not have it installed at all, as the application will probably have wrapped up the holochain library internally.)

Each of the tools includes a help command, e.g., run `hcadmin help` or for sub-commands run `hcadmin <COMMAND> help`. For more detailed information, see [the wiki page](https://github.com/metacurrency/holochain/wiki/command-line-tools)

The tool suite include these commands:

- `hcadmin` for administering your installed holochain applications
- `hcd` for running and serving a holochain application
- `hcdev` for developing and testing holochain applications

### Getting Started

The instructions below walk you through the basic steps necessary to run a holochain application.

#### Initializing the Holochain environment

```bash
$ hcadmin init 'your@emailaddress.here'
```
This command creates a `~/.holochain` directory for storing all chain data, along with initial public/private key pairs based on the identity string provided as the second argument.

#### Joining a Holochain

You can use the `hcadmin` tool to join a pre-existing Holochain application by running the following command (replacing SOURCE_PATH with a path to an application's DNA and CHAIN_NAME with the name you'd like it to be stored as).

For example: `hcadmin join ./examples/chat chat`

Note: this command will be replaced by a package management command still in development.

#### Running a Holochain
Holochains run and serve their UI via local web sockets. This let's interface developers have a lot of freedom to build HTML/JavaScript files and drop them in that chain's UI directory. You start a holochain and activate it's UI with the `hcd` command:

```bash
$ hcd <CHAIN_NAME> [PORT]
```

### Developing a Holochain

The `hcdev` tool allows you to:

1. generate new holochain application source files by cloning from an existing application, from the a [package file](https://github.com/metacurrency/hc-scaffold), or a simple empty template.
2. run stand-alone or multi-node scenario tests
3. run a holochain and serve it's UI for testing purposes
4. dump out chain and dht data for inspection

Please see the wiki for more [detailed documentation](https://github.com/metacurrency/holochain/wiki/hcdev-Command).

Note that the `hcdev` command creates a separate ~/.holochaindev directory for serving and managing chains, so your dev work won't interfere with any running holochain apps you may be using.

#### Test-driven Application Development
We have designed Holochain around test-driven development, so the DNA should contain tests to confirm that the rest of the DNA is functional.  Our testing harness includes two types of testing, stand-alone and multi-instance scenarios.  Stand-alone tests allow you to tests the functions you create in your application.  However, testing a distributed application requires being able to spin up many instances of it and have them interact. Our docker cluster testing harness automates that process, and enables app developers to specify scenarios and roles and test instructions to run on multiple docker containers.

Please see the [App-Testing](https://github.com/metacurrency/holochain/wiki/App-Testing) documentation for details.


#### File Locations
By default holochain data and configuration files are assumed to be stored in the `~/.holochain` directory.  You can override this with the `-path` flag or by setting the `HOLOPATH` environment variable, e.g.:
```bash
$ hcadmin -path ~/mychains init '<my@other.identity>'
$ HOLOPATH=~/mychains hcadmin
```
You can use the form: `hcadmin -path=/your/path/here` but you must use the absolute path, as shell substitutions will not happen.

#### Logging
All the commands take a `--debug` flag which will turn on a number of different kinds of debugging. For running chains, you can also control exactly which of these logging types you wish to see in the chain's config.json file. You can also set the DEBUG environment variable to 0 or 1 to temporarily override your settings to turn everything on or off.

## Architecture Overview and Documentation
Architecture information and application developer documentation is in our [Holochain Wiki](https://github.com/metacurrency/holochain/wiki/).

You can also look through auto-generated [reference API on GoDocs](https://godoc.org/github.com/metacurrency/holochain)

## Holochain Core Development
We accept Pull Requests and welcome your participation.

Some helpful links: [![In Progress](https://img.shields.io/waffle/label/metacurrency/holochain/in%20progress.svg)](http://waffle.io/metacurrency/holochain)
* Come [chat with us on gitter](https://gitter.im/metacurrency/holochain)
* View our [Kanban on Waffle](https://waffle.io/metacurrency/holochain).
* View our  [Milestone](https://github.com/metacurrency/holochain/milestones?direction=asc&sort=due_date&state=all) progress.

If you'd like to get involved you can:
* Contact us on [Gitter](https://gitter.im/metacurrency/holochain) to set up a **pair coding session** with one of our developers to learn the lay of the land.
* **join our dev documentation calls** twice weekly on Tuesdays and Fridays.

Current Throughput graph:

[![Throughput Graph](http://graphs.waffle.io/metacurrency/holochain/throughput.svg)](https://waffle.io/metacurrency/holochain/metrics)

### Contribute
Contributors to this project are expected to follow our [development protocols & practices](https://github.com/metacurrency/holochain/wiki/Development-Protocols).

### Dependencies
This project depends on various parts of [libp2p](https://github.com/libp2p/go-libp2p), which uses the [gx](https://github.com/whyrusleeping/gx) package manager. All of which will be automatically installed by make by following the [setup instructions](#installation) above.

### Tests
To compile and run all the tests:
```bash
$ cd $GOPATH/github.com/metacurrency/holochain
$ make test
```
If you want to use `go test` instead of `make test`, you'll need to do a couple extra things because of this project's dependency on `gx`:
* Before running `go test` you need to run `make work` to configure the imports properly.
* If you do this, before commiting you must also run `make pub` to revert the changes it makes.

The docker setup runs tests automatically during builds.

## License
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0)

Copyright (C) 2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)

This program is free software: you can redistribute it and/or modify it under the terms of the license provided in the LICENSE file (GPLv3).  This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

**Note:** We are considering other 'looser' licensing options (like MIT license) but at this stage are using GPL while we're getting the matter sorted out.

## Acknowledgements
* **MetaCurrency & Ceptr**: Holochains are a sub-project of [Ceptr](http://ceptr.org) which is a semantic, distributed computing platform under development by the [MetaCurrency Project](http://metacurrency.org).
&nbsp;
* **Ian Grigg**: Some of our initial plans for this architecture were inspired in 2006 by [his paper about Triple Entry Accounting](http://iang.org/papers/triple_entry.html) and his work on [Ricardian Contracts](http://iang.org/papers/ricardian_contract.html).
&nbsp;
* **Juan Benet & the IPFS team**: For all their work on IPFS, libp2p, and various cool tools like multihash, multiaddress, etc. We use libP2P library for our transport layer and kademlia dht.
&nbsp;
* **Crypto Pioneers** And of course the people who paved the road before us by writing good crypto libraries and *preaching the blockchain gospel*. Back in 2008, nobody understood what we were talking about when we started sharing our designs. The main reason people want it now, is because blockchains have opened their eyes to the power of decentralized architectures.

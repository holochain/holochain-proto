# Holochain
**Holographic storage for distributed applications.** A holochain is a monotonic distributed hash table (DHT) where every node enforces validation rules on data before publishing that data against the signed chains where the data originated.

In other words, a holochain functions very much **like a blockchain without bottlenecks** when it comes to enforcing validation rules, but is designed to  be fully distributed with each node only needing to hold a small portion of the data instead of everything needing a full copy of a global ledger. This makes it feasible to run blockchain-like applications on devices as lightweight as mobile phones.

**[Code Status:](https://github.com/metacurrency/holochain/milestones?direction=asc&sort=due_date&state=all)** Active development for **proof-of-concept stage**. Pre-alpha. Not for production use. We still expect to destructively restructure data chains at this time.

<table style="font-size:150%;"><tr>
<td><b>Holochain Info:</b>
<td><a href="https://github.com/metacurrency/holochain/blob/master/docs/FAQ.md">FAQ</a></td>
<td><a href="http://holochain.org/whitepaper">White Paper</a></td>
<td><a href="https://godoc.org/github.com/metacurrency/holochain">GoDocs</a></td></tr></table>

<br/>
**Table of Contents**
<!-- TOC START min:2 max:4 link:true update:true -->
  - [Installation](#installation)
  - [Usage](#usage)
  - [Architecture](#architecture)
    - [Functional Domains](#functional-domains)
      - [Group DNA / Holochain configuration](#group-dna--holochain-configuration)
      - [Individuals Authoring Content](#individuals-authoring-content)
      - [Application API](#application-api)
    - [Two Separate SubSystems](#two-separate-subsystems)
      - [Authoring your Local Chain](#authoring-your-local-chain)
      - [DHT Node -- Validating and Publishing](#dht-node----validating-and-publishing)
  - [Documentation](#documentation)
  - [Testing](#testing)
  - [Development](#development)
    - [Contributor Guidelines](#contributor-guidelines)
      - [Tech](#tech)
      - [Social](#social)
  - [License](#license)
  - [Acknowledgements](#acknowledgements)

<!-- TOC END -->

## Installation

Make sure you have a working environment set up for the Go language. [See the installation instructions for Go](http://golang.org/doc/install.html). Follow their instructions for setting your $GOPATH too.

To install the holochain command line interface, just run:
```
$ go get github.com/metacurrency/holochain/cmd/hc
```

Make sure your `PATH` includes the `$GOPATH/bin` directory so the program it builds can be easily called:
```
$ export PATH=$PATH:$GOPATH/bin
```

## Usage

Once you've gotten everything working as described above you can execute some basic holochain commands from the command line like this:

    hc help

Since holochain is basically a distributed database engine, you will probably only do some basic maintenance through the command line. To initialize holochain service and build the directories, files, and generates public/private keys.

    hc init "Fred Flinstone" <fred@flintsone.com>

You can use a pre-existing holochain configuration by replacing SOURCE with  path for loading existing DNA for a group's holochain.:

    hc gen from <SOURCE>

If you are a developer and want to build your own group configuration, data schemas, and validation rules for a holochain you can set up the initial scaffolding and files with:

    hc gen dev <NAME>

To aid development, the `gen dev` command also produces a `test` sub-directory with sample chain entries of the format `<index>_<schema-type>.zy`  The command:

    hc test <NAME>

runs validates these data entries against the validation rules.  Thus you can run this command as you make changes to your holochain DNA to confirm that it's all working.

After you have completed development for that chain, you can start the chain (i.e. create the genesis entries) with:

    hc gen chain <NAME>

To view all the chains on your system and their status, use:

    hc status

You can inspect the contents of a particular chain with:

    hc dump <NAME>




## Architecture
### Functional Domains
Holochains, by design, should be used in the context of a group operating by a shared set of agreements. Generally speaking, you don't need a holochain if you are just managing your own data.

These agreements are encoded in the validation rules which are checked before authoring to one's local chain, and are also checked by every DHT node asked to publish the new data.

In essence these ensure holochain participants operate according the same rules. Just like in blockchains, if you collude to break validation rules, you essentially have forked the chain. If you commit things to your chain, or try to publish things which don't comply with the validation rules, the rest of the network/DHT rejects it.

#### Group DNA / Holochain configuration
At this stage, a developer needs to set up the technical configuration of the collective agreements enforced by a holochain. This includes such things as: the holochain name, UUID, address & name spaces, data schemas, validation rules for chain entries and data propagation on the DHT,

#### Individuals Authoring Content
As an individual, you can join a holochain by installing its holochain configuration and configuring your ID, keys, chain, and DHT node in accord with the DNA specs.

#### Application API
Holochains function like a database. They don't have much end-user interface, and are primarily used by an application or program to store data. Unless you're a developer building one of these applications, you're not likely interact directly with a holochains. Hopefully, you install an application that does all that for you and the holochain stays nice and invisible enabling the application to store its information in a decentralized manner.

### Two Distinct SubSystems
There are two modes to participate in a holochain: as a **chain author**, and as a **DHT node**. We expect most installations will be doing both things and acting as full peers in a P2P data system. However, each could be run in a separate
container, communicating only by network interface.

#### 1. Authoring your Local Chain
Your chain is your signed, sequential record of the data you create to share on the holochain. Depending on the holochain's validation rules, this data may also be immutable and non-repudiable. Your local chain/data-store follows this pattern:

1. Validates your new data
2. Stores the data in a new chain entry
3. Signs it to your chain
4. Indexes the content
5. Shares it to the DHT
6. Responds to validation requests from DHT nodes

#### 2. Running a DHT Node
For serving data shared across the network. When your node receives a request from another node to publish DHT data, it will first validate the signatures, chain links, and any other application specific data integrity in the entity's source chain who is publishing the data.

## Documentation

Find additional documentation in the [docs directory](https://github.com/metacurrency/holochain/tree/master/docs).

You can also find the [auto-generated Reference API for Holochain on GoDocs](https://godoc.org/github.com/metacurrency/holochain)

## Testing

To compile and run all the tests:

    cd $GOPATH/github.com/metacurrency/holochain
    go test

## Development
We welcome your participation. See our [milestones for current progress](https://github.com/metacurrency/holochain/milestones?direction=asc&sort=due_date&state=all), or our [GitHub issue tracking kanban in waffle](https://waffle.io/metacurrency/holochain).  Or chat with us on gitter: [![Gitter](https://badges.gitter.im/metacurrency/holochain.svg)](https://gitter.im/metacurrency/holochain?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=body_badge)

[![In Progress](https://badge.waffle.io/metacurrency/holochain.svg?label=in%20progress&title=In%20Progress)](http://waffle.io/metacurrency/holochain)

### Contributor Guidelines

#### Tech
* We use **test driven development**. Adding a new function or feature, should mean you've added the tests that make sure it works.
* Set your editor to automatically use [gofmt](https://blog.golang.org/go-fmt-your-code) on save so there's no wasted discussion on proper indentation of brace style!
* [Contact us](https://gitter.im/metacurrency/holochain) to set up a **pair coding session** with one of our developers to learn the lay of the land
* **join our dev documentation calls** twice weekly on Tuesdays and Fridays.

#### Social
<!-- * Protocols for Inclusion. -->
We are committed to foster a vibrant thriving community, including growing a culture that breaks cycles of marginalization and dominance behavior. In support of this, some open source communities adopt [Codes of Conduct](http://contributor-covenant.org/version/1/3/0/).  We are still working on our social protocols, and empower each team to describe its own <i>Protocols for Inclusion</i>.  Until our teams have published their guidelines, please use the link above as a general guideline.

## License

Copyright (C) 2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)

This program is free software: you can redistribute it and/or modify it under the terms of the license provided in the LICENSE file (GPLv3).

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

## Acknowledgements
* **MetaCurrency & Ceptr**: Holochains are a sub-project of [Ceptr](http://ceptr.org) which is a semantic, distributed computing platform under development by the [MetaCurrency Project](http://metacurrency.org).
&nbsp;
* **Ian Grigg**: Some of our initial plans for this architecture were inspired in 2006 by [his paper about Triple Entry Accounting](http://iang.org/papers/triple_entry.html) and his work on [Ricardian Contracts](http://iang.org/papers/ricardian_contract.html).
<!-- * **Juan Benet**: For all his work on IPFS and being a generally cool guy. We're planning to piggyback a bunch of the networking communications for Holochains on the libP2P libary of IPFS and hopefully leverage their S/Kademlia DHT implementation. -->
* **Crypto Pioneers** And of course the people who paved the road before us by writing good crypto libraries and **preaching the blockchain gospel**. Nobody understood what we were talking about when we started sharing our designs. The main reason people want it now, is because blockchains have opened their eyes to new patterns of power available from decentralized architectures.

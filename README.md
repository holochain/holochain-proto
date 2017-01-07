# Holochain
Holographic storage for distributed applications. A holochain is a monotonic distributed hash table (DHT) where every node enforces validation rules on data before publishing that data against the signed chains where the data originated.

In other words, a holochain functions very much like a blockchain when it comes to enforcing validation rules, but it has no transactional bottlenecks and can be fully distributed with each node only needing to hold a small portion of the data instead of everything needing a full copy of a global ledger. This makes it feasible to run blockchain-like applications on devices as lightweight as mobile phones.

<!-- TOC START min:2 max:4 link:true update:true -->
  - [Installation](#installation)
  - [Usage](#usage)
  - [Architecture](#architecture)
    - [Functional Domains](#functional-domains)
      - [Group DNA / Holochain configuration](#group-dna--holochain-configuration)
      - [Individuals Authoring Content](#individuals-authoring-content)
      - [Application API](#application-api)
    - [Peering Modes](#peering-modes)
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
$ go get github.com/metacurrency/holochain/hc
```

Make sure your `PATH` includes the `$GOPATH/bin` directory so the program it builds can be easily called:
```
$ export PATH=$PATH:$GOPATH/bin
```

## Usage

Once you've gotten everything working as described above you can execute some basic holochain commands from the command line like this:

    hc help

Since holochain is basically a distributed database engine, you will probably only do some basic maintenance through the command line.

You can use an existing group configuration like this where SOURCE is a local file path or URI for retrieving existing DNA for a group's holochain.:

    hc gen chain <SOURCE>

You can generate your private and public keys for signing your local chain with:

    hc gen key

If you are a developer and want to build your own group configuration for a holochain you can set up the initial scaffolding and files with:

    hc gen dev


## Architecture
### Functional Domains
Holochains, by design, should be used in the context of a group operating by a shared set of agreements. Generally speaking, you don't need a holochain if you are just managing your own data.

These agreements are encoded in the validation rules which are checked before authoring to one's local chain, and are also checked by every DHT node asked to publish the new data.

In essence these ensure holochain participants operate according the same rules. Just like in blockchains, if you collude to break validation rules, you essentially have forked the chain. If you commit things to your chain, or try to publish things which don't comply with the validation rules, the rest of the network/DHT rejects it.

#### Group DNA / Holochain configuration
Group/Holochain name, UUID, address/name spaces, data schema, validation rules,

#### Individuals Authoring Content
...Keys, ID, Chain, Node, App-chains

#### Application API
Holochains are kind of like a database. They don't have much end-user interface, but are used by program developers to store data. Unless you're a developer building one of these applications, you're not likely to do much directly with your holochains. Hopefully, they stay nice and invisible just allowing your application to store its information in a decentralized manner.

### Peering Modes
There are two modes to participate in a holochain: as a **chain author**, and as a **DHT node**. We expect most installations will be doing both things and acting as full peers in a P2P data system.

#### Authoring your Local Chain
Your chain is your signed, sequential record or the data you create to share on the holochain. Depending on the holochain's validation rules, this data may also be immutable and non-repudiable. Your local chain/data-store follows this pattern:

1. Validates your new data
2. Stores the data in a new chain entry
3. Signs it to your chain
4. Indexes the content
5. Shares it to the DHT

#### DHT Node -- Validating and Publishing
For serving data shared across the network. When your node receives a request from another node to publish DHT data, it will first validate the signatures, chain links, and any other application specific data integrity in the entity's source chain who is publishing the data.

## Documentation

```
Forthcoming... GoDoc Generated API Docs
```

## Testing

To compile and run all the tests:

    cd $GOPATH/github.com/metacurrency/holochain
    go test

## Development

[![In Progress](https://badge.waffle.io/metacurrency/holochain.svg?label=in%20progress&title=In%20Progress)](http://waffle.io/metacurrency/holochain)

We welcome participation. Check our our waffle for [Roadmap & kanban](https://waffle.io/metacurrency/holochain) or if you prefer you can just use github's [issue tracking](https://github.com/metacurrency/holochain/issues).

Finally, zippy314 does some [livecoding](https://www.livecoding.tv/zippy/)..

### Contributor Guidelines

#### Tech

* We use test driven development.  Adding a new function or feature, should mean you've added a new test!

#### Social

* Protocols for Inclusion.

We recognize the need to actively foster vibrant thriving community, which means among other things, building a culture that breaks cycles of marginalization and dominance behavior.  To that end many open source communities adopt Codes of Conduct like [this one](http://contributor-covenant.org/version/1/3/0/).  We are in the process of addressing the goals of such codes in what we feel is a more general way, by establishing meta requirements for each membrane within our social organism to describe its <i>Protocols for Inclusion</i>.  Until we have done so please operate using the above referenced code as a general protocol for inclusion.

## License

Copyright (C) 2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)

This program is free software: you can redistribute it and/or modify it under the terms of the license provided in the LICENSE file (GPLv3).

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

## Acknowledgements
* **Ian Grigg**: Some of the initial ideas for this approach were inspired in 2006 by [his paper about Triple Entry Accounting](http://iang.org/papers/triple_entry.html) and his work on [Ricardian Contracts](http://iang.org/papers/ricardian_contract.html).

* **Juan Benet**: For all his work on IPFS and being a generally cool guy. We're planning to piggyback a bunch of the networking communications for Holochains on the libP2P libary of IPFS and hopefully leverage their S/Kademlia DHT implementation.

* And of course the people who paved the road before us by **preaching the blockchain gospel**. Nobody understood what we were talking about when we started sharing our designs. The main reason people want it now, is because blockchains have opened their eyes to new patterns of power available from decentralized architectures.

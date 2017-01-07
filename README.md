# Holochain
Holographic storage for distributed applications. A holochain is a monotonic distributed hash table (DHT) where every node enforces validation rules on data before publishing that data against the signed chains where the data originated.

In other words, a holochain functions very much like a blockchain when it comes to enforcing validation rules, but it has no transactional bottlenecks and can be fully distributed with each node only needing to hold a small portion of the data instead of everything needing a full copy of a global ledger. This makes it feasible to run blockchain-like applications on devices as lightweight as mobile phones.

<!-- TOC START min:2 max:4 link:true update:true -->
  - [Installation](#installation)
  - [Usage](#usage)
  - [Documentation](#documentation)
  - [Architecture](#architecture)
    - [Layers](#layers)
      - [Group DNA](#group-dna)
      - [Individual Participants](#individual-participants)
      - [Application UI](#application-ui)
    - [Peering Modes](#peering-modes)
      - [Authoring your Local Chain](#authoring-your-local-chain)
      - [DHT Node -- Validating and Publishing](#dht-node----validating-and-publishing)
  - [Testing](#testing)
  - [Development](#development)
    - [Contributor Guidelines](#contributor-guidelines)
      - [Tech](#tech)
      - [Social](#social)
  - [License](#license)
  - [Acknowledgements](#acknowledgements)
  - [FAQ (this text should move to another doc)](#faq-this-text-should-move-to-another-doc)

<!-- TOC END -->


## Installation

Make sure you have a working environment set up for the Go language. [See the installation instructions for Go](http://golang.org/doc/install.html).

To install the holochain command line interface, just run:
```
$ go get github.com/metacurrency/holochain/hc
```

Make sure your `PATH` includes the `$GOPATH/bin` directory so the program it builds can be easily called:
```
$ export PATH=$PATH:$GOPATH/bin
```

## Usage

Once you've gotten everything working as described above you can run holochain commands from the command line like this:

    hc help

## Documentation

See [FAQ]

## Architecture
### Layers
Holochains, by design, should be used in the context of a group operating by a shared set of agreements. Generally speaking, you don't need a holochain if you are just managing your own data.

These agreements are encoded in the validation rules which are checked before authoring to one's local chain, and are also checked by every DHT node asked to publish the new data.

In essence these ensure holochain participants operate according the same rules. Just like in blockchains, if you collude to break validation rules, you essentially have forked the chain. If you commit things to your chain, or try to publish things which don't comply with the validation rules, the rest of the network/DHT rejects it.

#### Group DNA
Group/Holochain name, UUID, address/name spaces, data schema, validation rules,

#### Individual Participants
...Keys, ID, Chain, Node, App-chains

#### Application UI
Holochains are kind of like a database. They don't have much end-user interface, but are used by program developers to store data. Unless you're a developer building one of these applications, you're not likely to do much directly with your holochains. Hopefully, they stay nice and invisible just allowing your application to store its information in a decentralized manner.

### Peering Modes
There are two modes to participate in a holochain: as a **chain author**, and as a **DHT node**. We expect most installations will be doing both things and acting as full peers in a P2P data system.

#### Authoring your Local Chain
Your chain is your signed, sequential record or the data you create to share on the holochain. Depending on the holochain's validation rules, this data may also be immutable and non-repudiable.

1. Creating new data
2. Validate the data
2. Sign it to your chain
3. Share it to the DHT

#### DHT Node -- Validating and Publishing
For serving data shared across the network. When your node receives a request from another node to publish DHT data, it will first validate the signatures, chain links, and any other application specific data integrity in the entity's source chain who is publishing the data.


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


## FAQ (this text should move to another doc)
```
How is this different from Blockchains?

How is this different from ordinary Distributed Hash Tables?

DHT's don't have validation rules.  This makes it so that they don't have that layer of social coherence.  This was partly a result of those projects not want

Not great for anonymously publishing content.  Great for AUTHORITATIVELY publishing content.  For creating content that is accompanied by authorship (signed provenance) and tamperproof-ness (intrinsic data integrity).

```

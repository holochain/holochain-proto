# FAQ

If you have more questions for our FAQ, please make a suggestion.

<!-- TOC START min:2 max:3 link:true update:true -->
  - [Why do you call it "Holochain?"](#why-do-you-call-it-holochain)
    - [A Unified Cryptographic WHOLE](#a-unified-cryptographic-whole)
    - [Holographic Storage](#holographic-storage)
    - [Holarchy](#holarchy)
  - [How is a Holochain different from a blockchain?](#how-is-a-holochain-different-from-a-blockchain)
  - [How is a Holochain different from a DHT (Distributed Hash Table)?](#how-is-a-holochain-different-from-a-dht-distributed-hash-table)
  - [What kind of projects is Holochain good for?](#what-kind-of-projects-is-holochain-good-for)
  - [What kind of projects is it NOT good for?](#what-kind-of-projects-is-it-not-good-for)
  - [Can you run a cryptocurrency on a Holochain?](#can-you-run-a-cryptocurrency-on-a-holochain)

<!-- TOC END -->

## Why do you call it "Holochain?"

### A Unified Cryptographic WHOLE
**Hashchains + Signing + DHT** --  multiple cryptographic technologies and composed them into a new unity.  
  1. **Independent Hashchains** provide immutable data integrity and definitive time sequence from the vantage point of each node. Truthfully, we're using hash trees -- blockchains do too, but they're not called blocktrees, so we're not calling these holotrees.
  2. **Signing** of chains, messages, and validation confirmations maintain authorship, provenance, and accountability. Countersigning of transactions/interactions between multiple parties provide non-repudiation and "locking" of chains.
  3. **DHT** (Distributed Hash Table) leverages cryptographic hashes for content addressable storage, randomization of interactions to discourage collusion, and performs validation of #1 and #2.

### Holographic Storage
Every node has a resilient sample of the whole. Like cutting a hologram, if you were to cut the network in half (make it so half the nodes were isolated from the other half), you would have two, whole, functioning systems, not two partial, broken systems.

This seems to be the strategy used to create resilience in natural systems. For example, where is your DNA stored? Every cell carries its own copy, with different functions expressed based on the role of that cell.

Where is the English language stored? Every speaker has a different version -- different areas of expertise, or exposure to different slang or specialized vocabularies. Nobody has a complete copy, nor is anyone's version exactly the same as anyone else,  If you disappeared half of the English speakers, it would not degrade the language much.

If you keep cutting a hologram smaller and smaller eventually the image degrades enough to stop being recognizable, and depending on the resiliency rules for DHT neighborhoods, holochains would likely share a similar fate. Although, if the process of killing off the nodes was not instantaneous, the network might be able to keep reshuffling data to keep it alive.

### Holarchy
Holochains are composable with each other into new levels of unification. In other words, holochains can build on decentralized capacities provided by other holochains, making new holistic patterns possible. Like bodies build new unity on holographic storage patterns that cells use for DNA, and a society build new unity on the holographic storage patterns of language, and so on.

*Share examples of how we're bootstrapping using a holochain of holochains, DPKI, neighborhood gossip systems, tagging system for locking top hashes, etc.*



## How is a Holochain different from a blockchain?
Long before blockchains were [hash chains](https://en.wikipedia.org/wiki/Hash_chain) and [hash trees](https://en.wikipedia.org/wiki/Merkle_tree). These structures ensure tamper-proof data integrity as progressive versions or additions to data are made. These kinds of hashes are often used as reference points to ensure data hasn't been messed with -- like making sure you're getting the program you meant to download, not some virus in its place.

Instead of trying to manage global consensus for every change to a huge blockchain ledger, every participant has [their own signed hash chain](https://medium.com/metacurrency-project/perspectives-on-blockchains-and-cryptocurrencies-7ef391605bd1#.kmous6d7z) ([countersigned for transactions](https://medium.com/metacurrency-project/beyond-blockchain-simple-scalable-cryptocurrencies-1eb7aebac6ae#.u1idviscz) involving others). After data is signed to local chains, it is shared to a [DHT](https://en.wikipedia.org/wiki/Distributed_hash_table) where every node runs the same validation rules (like blockchain nodes all run the [same validation rules](https://bitcoin.org/en/bitcoin-core/features/validation)). If someone breaks those rules, the DHT rejects their data -- their chain has forked away from the holochain.

The initial [Bitcoin white paper](https://bitcoin.org/bitcoin.pdf) introduced a blockchain as an architecture for decentralized production of a chain of digital currency transactions. This solved two problems (time/sequence of transactions, and randomizing who writes to the chain) with one main innovation of bundling transactions into blocks which somebody wins the prize of being able to commit to the chain if they [solve a busywork problem](https://en.bitcoin.it/wiki/Hashcash) faster than others.

Now Bitcoin and blockchain have pervaded people's consciousness and many perceive it as a solution for all sorts of decentralized applications. However, when the problems are framed slightly differently, there are much more efficient and elegant solutions (like holochains) which don't have the [processing bottlenecks](https://www.google.com/search?q=blockchain+bottleneck) of global consensus, storage requirements of everyone having a [FULL copy](https://blockchain.info/charts/blocks-size) of all the data, or [wasting so much electricity](https://blog.p2pfoundation.net/essay-of-the-day-bitcoin-mining-and-its-energy-footprint/2015/12/20) on busywork.

## How is a Holochain different from a DHT (Distributed Hash Table)?
Normal DHTs just allow distributed storage and retrieval of data. They don't have consistent validation rules to confirm authenticity, provenance, or integrity of data sources.

In fact, since many DHTs are used for illegal file sharing (Napster, Bittorrent, Sharezaa, etc.), they are designed to protect anonymity of uploaders so they don't get in trouble. File sharing DHTs frequently serve virus infected files, planted by uploaders intending to infect folks grabbing free software, music, or movies. There's no accountability nor assurance that data is what you want it to be.

By embedding validation rules in the propagation of data we can keep DHT data bound to signed sources. This can provide similar consistency and rule enforcement as blockchain approaches while completely eliminating the consensus bottleneck by enabling direct transaction between parties and their chains.

This allow the DHT to leverage the source chains to ensure tamper-proof immutability of data, as well as cryptographic signatures to verify its origins and provenance.


## What kind of projects is Holochain good for?
Sharing collaborative data without centralized control. Imagine a completely decentralized Wikipedia, DNS without root servers, or the ability to have fast reliable queries on a fully distributed PKI, etc.


## What kind of projects is it NOT good for?
Not great for anonymously publishing content.  Great for AUTHORITATIVELY publishing content.  For creating content that is accompanied by authorship (signed provenance) and tamperproof-ness (intrinsic data integrity).

Not ideal for applications which take a "data positivist" approach thinking that data is something that has existence independent of its source and creation. Unfortunately, most applications have been built inside of this mindset, and need to be redesigned to run properly on holochains. For example, cryptocurrencies built on top of managing the spending of cryptographic tokens instead of built on double-entry transactions between accounts.

## Can you run a cryptocurrency on a Holochain?
Theoretically, yes -- but for the moment, we'd discourage it.

You can't do it in the way everyone is used to building cryptocurrencies on cryptographic tokens. [Determining the status of tokens/coins](https://en.bitcoin.it/wiki/Double-spending) are what create the need for global consensus (about the existence/status/validity of the token or coin). However, there are [other approaches to making currencies](https://medium.com/metacurrency-project/perspectives-on-blockchains-and-cryptocurrencies-7ef391605bd1) which, for example, involve [issuance via mutual credit](https://medium.com/metacurrency-project/beyond-blockchain-simple-scalable-cryptocurrencies-1eb7aebac6ae) instead of issuance by fiat.

Unfortunately, this is a hotly contested topic by many who often don't have a deep understanding of currency design nor cryptography, so we're not going to go too deep in this FAQ. We intend to publish a white paper on this topic soon, as well as launch some currencies built this way.

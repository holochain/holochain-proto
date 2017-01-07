# FAQ

If you have more questions for our FAQ, please make a suggestion.

<!-- TOC START min:2 max:4 link:true update:true -->
  - [How is a Holochain different from a blockchain?](#how-is-a-holochain-different-from-a-blockchain)
  - [How is a Holochain different from a DHT (Distributed Hash Table)?](#how-is-a-holochain-different-from-a-dht-distributed-hash-table)
  - [What is Holochain good for?](#what-is-holochain-good-for)
  - [What is it NOT good for?](#what-is-it-not-good-for)

<!-- TOC END -->

## How is a Holochain different from a blockchain?
Long before blockchains were [hash chains](https://en.wikipedia.org/wiki/Hash_chain) and [hash trees](https://en.wikipedia.org/wiki/Merkle_tree). These structures ensure tamper-proof data integrity as progressive versions or additions to data are made. These kinds of hashes are often used as reference points to ensure data hasn't been messed with -- like making sure you're getting the program you meant to download, not some virus in its place.

The initial [Bitcoin white paper](https://bitcoin.org/bitcoin.pdf) introduced a blockchain as an architecture for decentralized production of a chain of digital currency transactions. This solved two problems (time/sequence of transactions, randomizing who writes to the chain) with one main innovation of bundling transactions into blocks which somebody wins the prize of being able to commit to the chain if they [solve a busywork problem](https://en.bitcoin.it/wiki/Hashcash) faster than others.

Now Bitcoin and blockchain have pervaded people's consciousness and many perceive it as a solution for all sorts of decentralized applications. However, when the problems are framed slightly differently, there are much more efficient and elegant solutions which don't have the [processing bottlenecks](https://www.google.com/search?q=blockchain+bottleneck) of global consensus, storage requirements of everyone having a [FULL copy](https://blockchain.info/charts/blocks-size) of all the data, or [wasting so much electricity](https://blog.p2pfoundation.net/essay-of-the-day-bitcoin-mining-and-its-energy-footprint/2015/12/20) on busywork.

## How is a Holochain different from a DHT (Distributed Hash Table)?
Normal DHTs just allow distributed storage and retrieval of data. They don't have validation rules to confirm authenticity, provenance, or integrity of data. In fact, since many DHTs are used for illegal file sharing, they are designed to protect anonymity of uploaders so they don't get in trouble for file sharing. Also these same sites frequently have virus infected files uploaded to mess up folks trying to get free software, music, and movies. There's no accountability nor assurance of data integrity.

By keeping data published on a DHT bound to its source embedding consistent validation rules in the production of data and it's propagation on the DHT, we can get similar
This makes it so that they don't have that layer of social coherence.  This was partly a result of those projects not want

## What is Holochain good for?
Sharing collaborative data without centralized control. Wikipedia example. Certificates / DPKI. DNS. Etc.

## What is it NOT good for?
Not great for anonymously publishing content.  Great for AUTHORITATIVELY publishing content.  For creating content that is accompanied by authorship (signed provenance) and tamperproof-ness (intrinsic data integrity).

```

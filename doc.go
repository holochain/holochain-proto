// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file

// Holochains are a distributed data store: DHT tightly bound to signed hash chains
// for provenance and data integrity.

/*
Holochain GoDoc

OVERVIEW

Holographic storage for distributed applications. 
A holochain is a validating distributed hash table (DHT) where every node 
enforces validation rules on data against the signed chains where the data 
originated.

In other words, a holochain functions very much like a blockchain without 
bottlenecks when it comes to enforcing validation rules, but is designed 
to be fully distributed through sharding so each node only needs to hold a 
portion of the data instead of a full copy of a global ledger. This makes 
it feasible to run blockchain-like applications on devices as lightweight 
as mobile phones

SHARED DATA INTEGRITY

Historically, data integrity has been ensured by restricting access to 
data. If we wanted to prevent anybody from tampering with data, we locked 
it behind firewalls, or set strict permissions on databases and file 
systems. Because with centrally stored data, having the ability to write 
to data typically means you can change whatever you want.

If we want to build peer-to-peer systems where we collectively hold data 
among many parties, we need better strategies for shared data integrity. 
Many are excited about building these kinds of applications on the 
blockchain, because they provide a strategy to maintain integrity of data 
that can be held by many peers without a single central authority.

However, other limitations have become apparent, such as high computational 
overhead for achieving consensus, and the Pareto Effects of Proof of Work 
and Proof of Stake which steer the system toward being more centralized 
than many would want.

Breakthroughs in shared data integrity enable new social, political, and 
organizational patterns with less tendencies toward corruption that emerge 
from power imbalances involved with selective parties controlling data, 
information, and protocols.

BEYOND BLOCKCHAIN BOTTLENECKS

We believe Holochains are one of these breakthroughs, because they take a 
different approach to ensuring the integrity of shared data. Instead of 
being built on top of cryptographic tokens they are organized around 
cryptographic validation of people (peers) validated against an immutable 
cryptographic record of those peers actions.

This change allows us to manage data integrity without the massive overhead 
of computing consensus on a global ledger. Our monotonic, validating, graph 
DHT (distributed hash table) achieves eventual consistency while only 
allowing valid data to propagate and holding everyone accountable for their 
actions.

The lower overhead of this approach makes it feasible to run full nodes on 
devices like cell phones or tablets which don't have massive computing power.

Holochain is designed as a data integrity engine for distributed 
applications. Unlike a distributed database, there are no methods for users 
to directly interact with the data because this would bypass application 
specific validation rules. All interactions happen only through the 
application code which enforce whatever business rules, application logic, 
or restrictions they need to, since different applications have different 
demands for strictness.

SUMMARY OF HOLOCHAIN ARCHITECTURE

You can think of a holochain as a shared DHT in which each piece of data is 
rooted in the signed hash chain of one or more parties. It is a validating 
DHT so data cannot propagate without first being validated by shared 
validation rules held by every node -- like every cell in your body has a 
copy of the same DNA.

Each holochain has THREE MAIN SUB-SYSTEMS

-Nucleus Application Engine

-Source Chain

-Shared Storage (Validating DHT)

see http://ceptr.org/images/Holochain_Subsystems.png

NUCLEUS APPLICATION ENGINE (APPLICATION)

The application is the glue that holds all the parts together into a unified 
whole. You connect to it with a web browser for a user interface. This 
application can read and write on your own local signed hash chain, and it 
can also get data from the Shared DHT, and put data you author out on that 
shared DHT.

Most importantly, it provides the VALIDATION RULES which everyone on that 
Holochain runs to make sure the data being held in the shared DHT can't be 
tampered with, counterfeited, or lost. As of March 2017, you can write 
applications in JavaScript or Lisp.

LOCAL SOURCE CHAIN

Instead of a shared global ledger like blockchains, every person has their own 
local chain that they sign things to before publishing them to the shared DHT. 
Interactions involving multiple parties (such as a currency transfer between 
two people) are signed by EACH party and committed to their own chains, and 
then shared to the DHT by each party.
see http://ceptr.org/images/Holochain_Source.png

Many of the applications people dream of running in shared decentralized 
manner (like a distributed Facebook, Twitter, Slack, Uber, or AirBnB) 
shouldn't need any kind of consensus from a large group of people. Why should 
I need consensus for a tweet or a social network update? Why should we need 
consensus from everyone in the community for me to reserve your spare room? 
What do these things have to do with anybody else's agreement?

Thankfully, if an app like this runs on a holochain, I can just write my tweet 
to my own chain, then share it. Or we can both sign the B&B reservation to each 
of our chains. And then the information that we've taken this action can 
propagate over the shared DHT, where nodes can confirm we did this according to 
shared rules or expectations.

SHARED STORAGE ON VALIDATING DHT

Distributed Hash Tables (DHTs) are already used for file sharing (bittorrent) 
and other widespread applications. In these systems, the data is content 
addressable by cryptographic hash, so you can confirm you receive unaltered data 
by hashing it yourself.

In our validating DHT, we confirm the PROVENANCE of every piece of data, 
validating the signature of its author, and that it has been committed to their 
local chain. Multi-party transactions create a "crossing" of chains which also 
assure that even if you try to alter your own chain, your transaction is 
published by others. Our DHT also has an unusual feature which allows meta-data 
to be put on data in the DHT which can be used to publish information about a 
person/node (such as their transactions, or top of their hashchain) or data 
element (such as tags, comments, or ratings).

Just like validation rules on blockchain nodes, if someone hacked their code to 
behave differently, even if they colluded with others, the rest of the nodes on 
the DHT would not validate their altered behavior and they will have essentially 
just forked themselves out of being able to participate on that holochain.
see http://ceptr.org/images/Holochain_DHT.png

More details see https://github.com/metacurrency/holochain/wiki/Architecture

WHY CEPTR? WHERE DOES THIS COME FROM?

Holochain is a part of a much larger vision for distributed computing to enable 
quantum leaps in our collective intelligence and abilities for groups to organize 
themselves on all scales. You can find out more about Ceptr at http://ceptr.org

A NOTE TO END USERS

Coming soon there will be applications built to make it easy to use holochains as 
your distributed database for all your daily needs. Hopefully, these applications 
will be as easy to find, install, and use as any other software you can think of. 
However, at the moment, these apps don't exist and holochain is largely for 
developers trying to build these things for you. Check back in Q2 of 2017 for some 
cool applications.
*/

package holochain

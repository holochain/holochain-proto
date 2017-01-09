// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file

// Holochains are a distributed data store: DHT tightly bound to signed hash chains
// for provenance and data integrity.

/*
Holographic storage for distributed applications. A holochain is a monotonic
distributed hash table (DHT) where every node enforces validation rules on
data before publishing that data against the signed chains where the data
originated.

In other words, a holochain functions very much like a blockchain without
bottlenecks when it comes to enforcing validation rules, but is designed
to be fully distributed with each node only needing to hold a small portion
of the data instead of everything needing a full copy of a global ledger.
This makes it feasible to run blockchain-like applications on devices as
lightweight as mobile phones.

Two Subsystems

There are two modes to participate in a holochain: as a **chain author**,
and as a **DHT node**. We expect most installations will be doing both things
and acting as full peers in a P2P data system. However, each could be run in a separate
container, communicating only by network interface.

Authoring your Local Chain

Your chain is your signed, sequential record of the data you create to share
on the holochain. Depending on the holochain's validation rules, this data
may also be immutable and non-repudiable. Your local chain/data-store follows
this pattern:

    1. Validates your new data
    2. Stores the data in a new chain entry
    3. Signs it to your chain
    4. Indexes the content
    5. Shares it to the DHT
    6. Responds to validation requests from DHT nodes

DHT Node -- Validating and Publishing

For serving data shared across the network. When your node receives a request
from another node to publish DHT data, it will first validate the signatures,
chain links, and any other application specific data integrity in the entity's
source chain who is publishing the data.

Installation and Usage

See http://github.com/metacurrency/holochain for installation instructions,
project status, and developer information.

.
*/

package holochain

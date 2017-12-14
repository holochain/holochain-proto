# Release Notes

## Alpha 0.0.1 --  Adventurer (12/8/2017)

This is an interim bug-fix and minor improvement release.  Here are some noteworthy fixes and the tickets where they were implemented, for details please check the commit log:

- Now compiles and runs on Windows, #508, #318
- Docker installation fixed
- Updates to test file and package file format, #502
- Tests 'Output' value can now be JSON instead of JSON string, #489
- `hcdev init -cloneExample` now accepts -fromBranch and -fromDevelop flags, #514
- Added substitutions for role's key and identity in scenario tests, #504
- Fixed bugs in bootstrap server, #508

## Alpha 0.0.0 --  Adventurer (10/23/2017)

This release marks the initial operational milestone of the major components of Holochain along with a [whitepaper](https://github.com/metacurrency/holochain/blob/master/holochain.pdf) that describes the system.


We call this the "Adventurer" release beacuse it's an early release appropriate for those with an open adventursome spirit, to play with.  Many  things will change in upcoming releases, and we recognize that you will likely to encounter difficulties on the way. We invite you to contact us on our [gitter](https://gitter.im/metacurrency/holochain) channel if you need help.

Here are some things NOT included in this release:

- **Security Audits**: Do not expect this code to be secure in any significant way.
- **Gossip Scaling**: Our proof-of-concept gossip protocol has significant performance problems and is missing a number of important features that will allow it to handle unstable networks and transient nodes.
- **DHT Sharding**: DHT sharding is not yet in place so nodes gossip with all other nodes.  This imposes obvious scaling problems.  This will be addressed in the [next alpha](https://github.com/metacurrency/holochain/milestone/12) release.

Please see our [Waffle](https://waffle.io/metacurrency/holochain) for a more detailed list of missing features and known problems slated for the next releases.

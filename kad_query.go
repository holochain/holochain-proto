// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//
// This code is adapted from the libp2p project, specifically:
// https://github.com/libp2p/go-libp2p-kad-dht/query.go
// The ipfs use of kademlia is substantially different than that needed by holochain so we remove
// parts we don't need and add others.

package holochain

import (
	"context"
	"sync"

	todoctr "github.com/ipfs/go-todocounter"
	routing "github.com/libp2p/go-libp2p-routing"
	//notif "github.com/libp2p/go-libp2p-routing/notifications"
	u "github.com/ipfs/go-ipfs-util"
	process "github.com/jbenet/goprocess"
	ctxproc "github.com/jbenet/goprocess/context"
	peer "github.com/libp2p/go-libp2p-peer"
	pset "github.com/libp2p/go-libp2p-peer/peerset"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/metacurrency/holochain/hash"
	queue "github.com/metacurrency/holochain/peerqueue"
)

var maxQueryConcurrency = AlphaValue

type dhtQuery struct {
	node        *Node
	key         Hash      // the key we're querying for
	qfunc       queryFunc // the function to execute per peer
	concurrency int       // the concurrency parameter
	log         *Logger
}

type dhtQueryResult struct {
	response    interface{}        // dht query
	peer        *pstore.PeerInfo   // FindPeer
	closerPeers []*pstore.PeerInfo // *
	success     bool

	finalSet *pset.PeerSet
}

// constructs query
func (node *Node) newQuery(k Hash, f queryFunc) *dhtQuery {
	return &dhtQuery{
		key:         k,
		node:        node,
		qfunc:       f,
		concurrency: maxQueryConcurrency,
		log:         node.log,
	}
}

// QueryFunc is a function that runs a particular query with a given peer.
// It returns either:
// - the value
// - a list of peers potentially better able to serve the query
// - an error
type queryFunc func(context.Context, peer.ID) (*dhtQueryResult, error)

// Run runs the query at hand. pass in a list of peers to use first.
func (q *dhtQuery) Run(ctx context.Context, peers []peer.ID) (*dhtQueryResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner := newQueryRunner(q)
	return runner.Run(ctx, peers)
}

type dhtQueryRunner struct {
	query          *dhtQuery        // query to run
	peersSeen      *pset.PeerSet    // all peers queried. prevent querying same peer 2x
	peersToQuery   *queue.ChanQueue // peers remaining to be queried
	peersRemaining todoctr.Counter  // peersToQuery + currently processing

	result *dhtQueryResult // query result
	errs   u.MultiErr      // result errors. maybe should be a map[peer.ID]error

	rateLimit chan struct{} // processing semaphore

	runCtx context.Context

	proc process.Process
	sync.RWMutex
}

func newQueryRunner(q *dhtQuery) *dhtQueryRunner {
	proc := process.WithParent(process.Background())
	ctx := ctxproc.OnClosingContext(proc)
	return &dhtQueryRunner{
		query:          q,
		peersToQuery:   queue.NewChanQueue(ctx, queue.NewXORDistancePQ(q.key)),
		peersRemaining: todoctr.NewSyncCounter(),
		peersSeen:      pset.New(),
		rateLimit:      make(chan struct{}, q.concurrency),
		proc:           proc,
	}
}

func (r *dhtQueryRunner) Run(ctx context.Context, peers []peer.ID) (*dhtQueryResult, error) {
	r.runCtx = ctx

	if len(peers) == 0 {
		Info("Running query with no peers!")
		return nil, nil
	}

	// setup concurrency rate limiting
	for i := 0; i < r.query.concurrency; i++ {
		r.rateLimit <- struct{}{}
	}

	// add all the peers we got first.
	for _, p := range peers {
		r.addPeerToQuery(p)
	}

	// go do this thing.
	// do it as a child proc to make sure Run exits
	// ONLY AFTER spawn workers has exited.
	r.proc.Go(r.spawnWorkers)

	// so workers are working.

	// wait until they're done.
	err := routing.ErrNotFound

	// now, if the context finishes, close the proc.
	// we have to do it here because the logic before is setup, which
	// should run without closing the proc.
	ctxproc.CloseAfterContext(r.proc, ctx)

	select {
	case <-r.peersRemaining.Done():
		r.proc.Close()
		r.RLock()
		defer r.RUnlock()

		err = routing.ErrNotFound

		// if every query to every peer failed, something must be very wrong.
		if len(r.errs) > 0 && len(r.errs) == r.peersSeen.Size() {
			r.query.log.Logf("query errs: %s", r.errs)
			err = r.errs[0]
		}

	case <-r.proc.Closed():
		r.RLock()
		defer r.RUnlock()
		err = context.DeadlineExceeded
	}

	if r.result != nil && r.result.success {
		return r.result, nil
	}

	return &dhtQueryResult{
		finalSet: r.peersSeen,
	}, err
}

func (r *dhtQueryRunner) addPeerToQuery(next peer.ID) {
	// if new peer is ourselves...
	if next == r.query.node.HashAddr {
		r.query.log.Log("addPeerToQuery skip self")
		return
	}

	if !r.peersSeen.TryAdd(next) {
		return
	}

	/*
		notif.PublishQueryEvent(r.runCtx, &notif.QueryEvent{
			Type: notif.AddingPeer,
			ID:   next,
		})
	*/
	r.peersRemaining.Increment(1)
	select {
	case r.peersToQuery.EnqChan <- next:
	case <-r.proc.Closing():
	}
}

func (r *dhtQueryRunner) spawnWorkers(proc process.Process) {
	for {

		select {
		case <-r.peersRemaining.Done():
			return

		case <-r.proc.Closing():
			return

		case <-r.rateLimit:
			select {
			case p, more := <-r.peersToQuery.DeqChan:
				if !more {
					return // channel closed.
				}

				// do it as a child func to make sure Run exits
				// ONLY AFTER spawn workers has exited.
				proc.Go(func(proc process.Process) {
					r.queryPeer(proc, p)
				})
			case <-r.proc.Closing():
				return
			case <-r.peersRemaining.Done():
				return
			}
		}
	}
}

func (r *dhtQueryRunner) queryPeer(proc process.Process, p peer.ID) {
	// ok let's do this!

	// create a context from our proc.
	ctx := ctxproc.OnClosingContext(proc)

	// make sure we do this when we exit
	defer func() {
		// signal we're done proccessing peer p
		r.peersRemaining.Decrement(1)
		r.rateLimit <- struct{}{}
	}()

	// make sure we're connected to the peer.
	// FIXME abstract away into the network layer
	if conns := r.query.node.host.Network().ConnsToPeer(p); len(conns) == 0 {
		r.query.log.Log("not connected. dialing.")

		/*
			notif.PublishQueryEvent(r.runCtx, &notif.QueryEvent{
				Type: notif.DialingPeer,
				ID:   p,
			})
		*/
		// while we dial, we do not take up a rate limit. this is to allow
		// forward progress during potentially very high latency dials.
		r.rateLimit <- struct{}{}

		pi := pstore.PeerInfo{ID: p}

		if err := r.query.node.host.Connect(ctx, pi); err != nil {
			r.query.log.Logf("Error connecting: %s", err)

			/*
				notif.PublishQueryEvent(r.runCtx, &notif.QueryEvent{
								Type:  notif.QueryError,
								Extra: err.Error(),
								ID:    p,
							})
			*/
			r.Lock()
			r.errs = append(r.errs, err)
			r.Unlock()
			<-r.rateLimit // need to grab it again, as we deferred.
			return
		}
		<-r.rateLimit // need to grab it again, as we deferred.
		r.query.log.Log("connected. dial success.")
	}

	// finally, run the query against this peer
	res, err := r.query.qfunc(ctx, p)

	if err != nil {
		r.query.log.Logf("ERROR worker for: %v %v", p, err)
		r.Lock()
		r.errs = append(r.errs, err)
		r.Unlock()

	} else if res.success {
		r.query.log.Logf("SUCCESS worker for: %v %v", p, res)
		r.Lock()
		r.result = res
		r.Unlock()
		go r.proc.Close() // signal to everyone that we're done.
		// must be async, as we're one of the children, and Close blocks.

	} else if len(res.closerPeers) > 0 {
		r.query.log.Logf("PEERS CLOSER -- worker for: %v (%d closer peers)", p, len(res.closerPeers))
		for _, next := range res.closerPeers {
			if next.ID == r.query.node.HashAddr { // dont add self.
				r.query.log.Logf("PEERS CLOSER -- worker for: %v found self", p)
				continue
			}

			// add their addresses to the dialer's peerstore
			r.query.node.peerstore.AddAddrs(next.ID, next.Addrs, pstore.TempAddrTTL)
			r.addPeerToQuery(next.ID)
			r.query.log.Logf("PEERS CLOSER -- worker for: %v added %v (%v)", p, next.ID, next.Addrs)
		}
	} else {
		r.query.log.Logf("QUERY worker for: %v - not found, and no closer peers. (res: %v)", p, res)
	}
}

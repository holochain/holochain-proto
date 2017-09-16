// modification of  https://github.com/libp2p/go-libp2p-peerstore/queue for holochain context

package peerqueue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	mh "github.com/multiformats/go-multihash"
)

func TestQueue(t *testing.T) {
	h1, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh1")
	h2, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	h3, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
	h4, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh4")
	h5, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh1")

	p1 := PeerIDFromHash(h1)
	p2 := PeerIDFromHash(h2)
	p3 := PeerIDFromHash(h3)
	p4 := PeerIDFromHash(h4)
	p5 := PeerIDFromHash(h5)

	pq := NewXORDistancePQ(h1)
	pq.Enqueue(p3)
	pq.Enqueue(p1)
	pq.Enqueue(p2)
	pq.Enqueue(p4)
	pq.Enqueue(p5)
	pq.Enqueue(p1)

	// should come out as: p1 or p5, p2, p3, p4
	d := pq.Dequeue()
	if d != p1 && d != p5 {
		t.Error("ordering failed")
	}

	d = pq.Dequeue()
	if d != p1 && d != p5 {
		t.Error("ordering failed")
	}

	d = pq.Dequeue()
	if d != p1 && d != p5 {
		t.Error("ordering failed")
	}

	d = pq.Dequeue()
	if d != p2 {
		t.Error("ordering failed")
	}

	d = pq.Dequeue()
	if d != p3 {
		t.Error("ordering failed")
	}

	d = pq.Dequeue()
	if d != p4 {
		t.Error("ordering failed")
	}
}

func newPeerTime(t time.Time) peer.ID {
	s := fmt.Sprintf("hmmm time: %v", t)
	h, _ := mh.Sum([]byte(s), mh.SHA2_256, -1)
	return peer.ID(h)
}

func TestSyncQueue(t *testing.T) {
	tickT := time.Microsecond * 50
	max := 5000
	consumerN := 10
	countsIn := make([]int, consumerN*2)
	countsOut := make([]int, consumerN)

	if testing.Short() {
		max = 1000
	}

	ctx := context.Background()
	pq := NewXORDistancePQ(HashFromPeerID(peer.ID("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31")))
	cq := NewChanQueue(ctx, pq)
	wg := sync.WaitGroup{}

	produce := func(p int) {
		defer wg.Done()

		tick := time.Tick(tickT)
		for i := 0; i < max; i++ {
			select {
			case tim := <-tick:
				countsIn[p]++
				cq.EnqChan <- newPeerTime(tim)
			case <-ctx.Done():
				return
			}
		}
	}

	consume := func(c int) {
		defer wg.Done()

		for {
			select {
			case <-cq.DeqChan:
				countsOut[c]++
				if countsOut[c] >= max*2 {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}

	// make n * 2 producers and n consumers
	for i := 0; i < consumerN; i++ {
		wg.Add(3)
		go produce(i)
		go produce(consumerN + i)
		go consume(i)
	}

	wg.Wait()

	sum := func(ns []int) int {
		total := 0
		for _, n := range ns {
			total += n
		}
		return total
	}

	if sum(countsIn) != sum(countsOut) {
		t.Errorf("didn't get all of them out: %d/%d", sum(countsOut), sum(countsIn))
	}
}

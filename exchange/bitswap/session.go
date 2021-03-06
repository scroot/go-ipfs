package bitswap

import (
	"context"
	"time"

	notifications "github.com/scroot/go-ipfs/exchange/bitswap/notifications"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	loggables "gx/ipfs/QmVesPmqbPp7xRGyY96tnBwzDtVV1nqv4SCVxo5zCqKyH8/go-libp2p-loggables"
	blocks "gx/ipfs/QmXxGS5QsUxpR3iqL5DjmsYPHR1Yz74siRQ4ChJqWFosMh/go-block-format"
	cid "gx/ipfs/Qma4RJSuh7mMeJQYCqMbKzekn6EwBo7HEs5AQYjVRMQATB/go-cid"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

const activeWantsLimit = 16

// Session holds state for an individual bitswap transfer operation.
// This allows bitswap to make smarter decisions about who to send wantlist
// info to, and who to request blocks from
type Session struct {
	ctx            context.Context
	tofetch        *cidQueue
	activePeers    map[peer.ID]struct{}
	activePeersArr []peer.ID

	bs           *Bitswap
	incoming     chan blkRecv
	newReqs      chan []*cid.Cid
	cancelKeys   chan []*cid.Cid
	interestReqs chan interestReq

	interest  *lru.Cache
	liveWants map[string]time.Time

	tick          *time.Timer
	baseTickDelay time.Duration

	latTotal time.Duration
	fetchcnt int

	notif notifications.PubSub

	uuid logging.Loggable

	id uint64
}

// NewSession creates a new bitswap session whose lifetime is bounded by the
// given context
func (bs *Bitswap) NewSession(ctx context.Context) *Session {
	s := &Session{
		activePeers:   make(map[peer.ID]struct{}),
		liveWants:     make(map[string]time.Time),
		newReqs:       make(chan []*cid.Cid),
		cancelKeys:    make(chan []*cid.Cid),
		tofetch:       newCidQueue(),
		interestReqs:  make(chan interestReq),
		ctx:           ctx,
		bs:            bs,
		incoming:      make(chan blkRecv),
		notif:         notifications.New(),
		uuid:          loggables.Uuid("GetBlockRequest"),
		baseTickDelay: time.Millisecond * 500,
		id:            bs.getNextSessionID(),
	}

	cache, _ := lru.New(2048)
	s.interest = cache

	bs.sessLk.Lock()
	bs.sessions = append(bs.sessions, s)
	bs.sessLk.Unlock()

	go s.run(ctx)

	return s
}

type blkRecv struct {
	from peer.ID
	blk  blocks.Block
}

func (s *Session) receiveBlockFrom(from peer.ID, blk blocks.Block) {
	s.incoming <- blkRecv{from: from, blk: blk}
}

type interestReq struct {
	c    *cid.Cid
	resp chan bool
}

// TODO: PERF: this is using a channel to guard a map access against race
// conditions. This is definitely much slower than a mutex, though its unclear
// if it will actually induce any noticeable slowness. This is implemented this
// way to avoid adding a more complex set of mutexes around the liveWants map.
// note that in the average case (where this session *is* interested in the
// block we received) this function will not be called, as the cid will likely
// still be in the interest cache.
func (s *Session) isLiveWant(c *cid.Cid) bool {
	resp := make(chan bool)
	s.interestReqs <- interestReq{
		c:    c,
		resp: resp,
	}
	return <-resp
}

func (s *Session) interestedIn(c *cid.Cid) bool {
	return s.interest.Contains(c.KeyString()) || s.isLiveWant(c)
}

const provSearchDelay = time.Second * 10

func (s *Session) addActivePeer(p peer.ID) {
	if _, ok := s.activePeers[p]; !ok {
		s.activePeers[p] = struct{}{}
		s.activePeersArr = append(s.activePeersArr, p)
	}
}

func (s *Session) resetTick() {
	if s.latTotal == 0 {
		s.tick.Reset(provSearchDelay)
	} else {
		avLat := s.latTotal / time.Duration(s.fetchcnt)
		s.tick.Reset(s.baseTickDelay + (3 * avLat))
	}
}

func (s *Session) run(ctx context.Context) {
	s.tick = time.NewTimer(provSearchDelay)
	newpeers := make(chan peer.ID, 16)
	for {
		select {
		case blk := <-s.incoming:
			s.tick.Stop()

			s.addActivePeer(blk.from)

			s.receiveBlock(ctx, blk.blk)

			s.resetTick()
		case keys := <-s.newReqs:
			for _, k := range keys {
				s.interest.Add(k.KeyString(), nil)
			}
			if len(s.liveWants) < activeWantsLimit {
				toadd := activeWantsLimit - len(s.liveWants)
				if toadd > len(keys) {
					toadd = len(keys)
				}

				now := keys[:toadd]
				keys = keys[toadd:]

				s.wantBlocks(ctx, now)
			}
			for _, k := range keys {
				s.tofetch.Push(k)
			}
		case keys := <-s.cancelKeys:
			s.cancel(keys)

		case <-s.tick.C:
			var live []*cid.Cid
			for c := range s.liveWants {
				cs, _ := cid.Cast([]byte(c))
				live = append(live, cs)
				s.liveWants[c] = time.Now()
			}

			// Broadcast these keys to everyone we're connected to
			s.bs.wm.WantBlocks(ctx, live, nil, s.id)

			if len(live) > 0 {
				go func(k *cid.Cid) {
					// TODO: have a task queue setup for this to:
					// - rate limit
					// - manage timeouts
					// - ensure two 'findprovs' calls for the same block don't run concurrently
					// - share peers between sessions based on interest set
					for p := range s.bs.network.FindProvidersAsync(ctx, k, 10) {
						newpeers <- p
					}
				}(live[0])
			}
			s.resetTick()
		case p := <-newpeers:
			s.addActivePeer(p)
		case lwchk := <-s.interestReqs:
			lwchk.resp <- s.cidIsWanted(lwchk.c)
		case <-ctx.Done():
			s.tick.Stop()
			return
		}
	}
}

func (s *Session) cidIsWanted(c *cid.Cid) bool {
	_, ok := s.liveWants[c.KeyString()]
	if !ok {
		ok = s.tofetch.Has(c)
	}

	return ok
}

func (s *Session) receiveBlock(ctx context.Context, blk blocks.Block) {
	c := blk.Cid()
	if s.cidIsWanted(c) {
		ks := c.KeyString()
		tval, ok := s.liveWants[ks]
		if ok {
			s.latTotal += time.Since(tval)
			delete(s.liveWants, ks)
		} else {
			s.tofetch.Remove(c)
		}
		s.fetchcnt++
		s.notif.Publish(blk)

		if next := s.tofetch.Pop(); next != nil {
			s.wantBlocks(ctx, []*cid.Cid{next})
		}
	}
}

func (s *Session) wantBlocks(ctx context.Context, ks []*cid.Cid) {
	for _, c := range ks {
		s.liveWants[c.KeyString()] = time.Now()
	}
	s.bs.wm.WantBlocks(ctx, ks, s.activePeersArr, s.id)
}

func (s *Session) cancel(keys []*cid.Cid) {
	for _, c := range keys {
		s.tofetch.Remove(c)
	}
}

func (s *Session) cancelWants(keys []*cid.Cid) {
	s.cancelKeys <- keys
}

func (s *Session) fetch(ctx context.Context, keys []*cid.Cid) {
	select {
	case s.newReqs <- keys:
	case <-ctx.Done():
	}
}

// GetBlocks fetches a set of blocks within the context of this session and
// returns a channel that found blocks will be returned on. No order is
// guaranteed on the returned blocks.
func (s *Session) GetBlocks(ctx context.Context, keys []*cid.Cid) (<-chan blocks.Block, error) {
	ctx = logging.ContextWithLoggable(ctx, s.uuid)
	return getBlocksImpl(ctx, keys, s.notif, s.fetch, s.cancelWants)
}

// GetBlock fetches a single block
func (s *Session) GetBlock(parent context.Context, k *cid.Cid) (blocks.Block, error) {
	return getBlock(parent, k, s.GetBlocks)
}

type cidQueue struct {
	elems []*cid.Cid
	eset  *cid.Set
}

func newCidQueue() *cidQueue {
	return &cidQueue{eset: cid.NewSet()}
}

func (cq *cidQueue) Pop() *cid.Cid {
	for {
		if len(cq.elems) == 0 {
			return nil
		}

		out := cq.elems[0]
		cq.elems = cq.elems[1:]

		if cq.eset.Has(out) {
			cq.eset.Remove(out)
			return out
		}
	}
}

func (cq *cidQueue) Push(c *cid.Cid) {
	if cq.eset.Visit(c) {
		cq.elems = append(cq.elems, c)
	}
}

func (cq *cidQueue) Remove(c *cid.Cid) {
	cq.eset.Remove(c)
}

func (cq *cidQueue) Has(c *cid.Cid) bool {
	return cq.eset.Has(c)
}

func (cq *cidQueue) Len() int {
	return cq.eset.Len()
}

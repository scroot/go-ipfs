package mdutils

import (
	"github.com/scroot/go-ipfs/blocks/blockstore"
	bsrv "github.com/scroot/go-ipfs/blockservice"
	"github.com/scroot/go-ipfs/exchange/offline"
	dag "github.com/scroot/go-ipfs/merkledag"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	dssync "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore/sync"
)

func Mock() dag.DAGService {
	return dag.NewDAGService(Bserv())
}

func Bserv() bsrv.BlockService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	return bsrv.New(bstore, offline.Exchange(bstore))
}

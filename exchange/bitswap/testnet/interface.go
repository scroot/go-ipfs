package bitswap

import (
	bsnet "github.com/scroot/go-ipfs/exchange/bitswap/network"
	"github.com/scroot/go-ipfs/thirdparty/testutil"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}

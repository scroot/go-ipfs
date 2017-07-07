package plugin

import (
	"github.com/ipfs/go-ipfs/core/coredag"

	node "gx/ipfs/QmUBtPvHKFAX43XMsyxsYpMi3U5VwZ4jYFTo4kFhvAR33G/go-ipld-format"
)

type PluginIPLD interface {
	Plugin

	RegisterBlockDecoders(dec node.BlockDecoder) error
	RegisterInputEncParsers(iec coredag.InputEncParsers) error
}

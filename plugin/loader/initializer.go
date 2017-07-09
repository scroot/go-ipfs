package loader

import (
	"github.com/ipfs/go-ipfs/core/coredag"
	"github.com/ipfs/go-ipfs/plugin"

	format "gx/ipfs/QmUBtPvHKFAX43XMsyxsYpMi3U5VwZ4jYFTo4kFhvAR33G/go-ipld-format"
)

func initalize(plugins []plugin.Plugin) error {
	for _, p := range plugins {
		err := p.Init()
		if err != nil {
			return err
		}
	}

	return nil
}

func run(plugins []plugin.Plugin) error {
	for _, pl := range plugins {
		err := runIPLDPlugin(pl)
		if err != nil {
			return err
		}
	}
	return nil
}

func runIPLDPlugin(pl plugin.Plugin) error {
	ipldpl, ok := pl.(plugin.PluginIPLD)
	if !ok {
		return nil
	}

	var err error
	err = ipldpl.RegisterBlockDecoders(format.DefaultBlockDecoder)
	if err != nil {
		return err
	}

	err = ipldpl.RegisterInputEncParsers(coredag.DefaultInputEncParsers)
	return err
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	node "gx/ipfs/QmUBtPvHKFAX43XMsyxsYpMi3U5VwZ4jYFTo4kFhvAR33G/go-ipld-format"
)

func init() {
	LoadPlugins = LoadPluginsLinux
}

func LoadPluginsLinux(cfgroot string) error {
	plugdirpath := filepath.Join(cfgroot, "plugins")
	return filepath.Walk(plugdirpath, func(fi string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			log.Warning("found directory inside plugins directory")
			return nil
		}

		if err := loadPlugin(fi); err != nil {
			return fmt.Errorf("loading plugin %s: %s", fi, err)
		}
		return nil
	})
}

func loadPlugin(pluginfi string) error {
	plugin, err := plugin.Open(pluginfi)
	if err != nil {
		return err
	}

	typestri, err := plugin.Lookup("PluginType")
	if err != nil {
		return fmt.Errorf("check type: %s", err)
	}

	typestr, ok := typestri.(*string)
	if !ok {
		return fmt.Errorf("'PluginType' var was not a string (got %T)", typestri)
	}

	switch *typestr {
	case "ipld":
		regfunci, err := plugin.Lookup("Register")
		if err != nil {
			return fmt.Errorf("ipld plugin had no Register function: %s", err)
		}

		regfunc, ok := regfunci.(func(node.BlockDecoder) error)
		if !ok {
			return fmt.Errorf("'Register' function was not func(node.BlockDecoder) error")
		}

		// TODO: still to be figured out is: how do we register handlers for `ipfs dag put` in here?
		// The issue is that the stuff you pipe to `ipfs dag put` doesnt
		// necessarily map 1:1 to a single node. With the blockchains code, the
		// data you send to 'put' turns into a block, some transactions, and a
		// merkletrie. Probably need to have a separate package like 'coredag'
		// or something external that holds a global registry of all that sort
		// of logic.

		// note: not relying on registration via plugin 'init' function. Could
		// result in things mysteriously not working when types arent quite
		// right. Thanks Go.
		return regfunc(node.DefaultBlockDecoder)
	case "libp2p-transport":
		// TODO: how should this get wired in? needs patching into the swarm instance?
		return fmt.Errorf("libp2p-transport plugins not yet available")
	default:
		return fmt.Errorf("unrecognized plugin type: %s", typestr)
	}
}

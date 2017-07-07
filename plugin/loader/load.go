package loader

import (
	"github.com/ipfs/go-ipfs/plugin"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
)

var log = logging.Logger("plugin/loader")

var loadPluginsFunc = func(string) ([]plugin.Plugin, error) {
	return nil, nil
}

func LoadPlugins(pluginDir string) ([]plugin.Plugin, error) {
	return loadPluginsFunc(pluginDir)
}

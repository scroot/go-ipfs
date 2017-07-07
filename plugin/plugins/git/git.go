package main

import (
	"compress/zlib"
	"io"

	"github.com/ipfs/go-ipfs/core/coredag"
	"github.com/ipfs/go-ipfs/plugin"

	"gx/ipfs/QmNw61A6sJoXMeP37mJRtQZdNhj5e3FdjoTN3v4FyE96Gk/go-cid"
	"gx/ipfs/QmUBtPvHKFAX43XMsyxsYpMi3U5VwZ4jYFTo4kFhvAR33G/go-ipld-format"
	git "gx/ipfs/QmdZKFV1ppRwGi5VQixitK5V2ihkDXyFckxaotPgXknHv4/go-ipld-git"
)

var Plugins = []plugin.Plugin{
	&GitPlugin{},
}

type GitPlugin struct{}

var _ plugin.PluginIPLD = (*GitPlugin)(nil)

func (*GitPlugin) Name() string {
	return "ipld-git"
}

func (*GitPlugin) Version() string {
	return "0.0.1"
}

func (*GitPlugin) Init() error {
	return nil
}

func (*GitPlugin) RegisterBlockDecoders(dec format.BlockDecoder) error {
	dec[cid.GitRaw] = git.DecodeBlock
	return nil
}

func (*GitPlugin) RegisterInputEncParsers(iec coredag.InputEncParsers) error {
	iec.AddParser("raw", "git", parseRawGit)
	iec.AddParser("zlib", "git", parseZlibGit)
	return nil
}

func parseRawGit(r io.Reader) ([]format.Node, error) {
	nd, err := git.ParseObject(r)
	if err != nil {
		return nil, err
	}

	return []format.Node{nd}, nil
}

func parseZlibGit(r io.Reader) ([]format.Node, error) {
	rc, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}

	defer rc.Close()
	return parseRawGit(rc)
}

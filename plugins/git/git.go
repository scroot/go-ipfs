package main

import (
	"compress/zlib"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs/core/coredag"
	git "github.com/ipfs/go-ipld-git"
	"gx/ipfs/QmUBtPvHKFAX43XMsyxsYpMi3U5VwZ4jYFTo4kFhvAR33G/go-ipld-format"
)

var PluginType = "ipld"

func Register(dec format.BlockDecoder) error {
	dec[cid.GitRaw] = git.DecodeBlock
	coredag.DefaultInputEncParsers.AddParser("raw", "git", parseRawGit)
	coredag.DefaultInputEncParsers.AddParser("zlib", "git", parseZlibGit)
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

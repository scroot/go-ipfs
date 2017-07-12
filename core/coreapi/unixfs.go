package coreapi

import (
	"context"
	"io"

	coreiface "github.com/scroot/go-ipfs/core/coreapi/interface"
	coreunix "github.com/scroot/go-ipfs/core/coreunix"
	uio "github.com/scroot/go-ipfs/unixfs/io"

	node "gx/ipfs/QmPAKbSsgEX5B6fpmxa61jXYnoWzZr5sNafd3qgPiSH8Uv/go-ipld-format"
	cid "gx/ipfs/Qma4RJSuh7mMeJQYCqMbKzekn6EwBo7HEs5AQYjVRMQATB/go-cid"
)

type UnixfsAPI CoreAPI

func (api *UnixfsAPI) Add(ctx context.Context, r io.Reader) (coreiface.Path, error) {
	k, err := coreunix.AddWithContext(ctx, api.node, r)
	if err != nil {
		return nil, err
	}
	c, err := cid.Decode(k)
	if err != nil {
		return nil, err
	}
	return ParseCid(c), nil
}

func (api *UnixfsAPI) Cat(ctx context.Context, p coreiface.Path) (coreiface.Reader, error) {
	dagnode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	r, err := uio.NewDagReader(ctx, dagnode, api.node.DAG)
	if err == uio.ErrIsDir {
		return nil, coreiface.ErrIsDir
	} else if err != nil {
		return nil, err
	}
	return r, nil
}

func (api *UnixfsAPI) Ls(ctx context.Context, p coreiface.Path) ([]*coreiface.Link, error) {
	dagnode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	var ndlinks []*node.Link
	dir, err := uio.NewDirectoryFromNode(api.node.DAG, dagnode)
	switch err {
	case nil:
		l, err := dir.Links(ctx)
		if err != nil {
			return nil, err
		}
		ndlinks = l
	case uio.ErrNotADir:
		ndlinks = dagnode.Links()
	default:
		return nil, err
	}

	links := make([]*coreiface.Link, len(ndlinks))
	for i, l := range ndlinks {
		links[i] = &coreiface.Link{l.Name, l.Size, l.Cid}
	}
	return links, nil
}

func (api *UnixfsAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}

package path_test

import (
	"context"
	"fmt"
	"testing"

	merkledag "github.com/scroot/go-ipfs/merkledag"
	dagmock "github.com/scroot/go-ipfs/merkledag/test"
	path "github.com/scroot/go-ipfs/path"

	node "gx/ipfs/QmPAKbSsgEX5B6fpmxa61jXYnoWzZr5sNafd3qgPiSH8Uv/go-ipld-format"
	util "gx/ipfs/QmWbjfz3u6HkAdPh34dgPchGbQjob6LXLhAeCGii2TX69n/go-ipfs-util"
)

func randNode() *merkledag.ProtoNode {
	node := new(merkledag.ProtoNode)
	node.SetData(make([]byte, 32))
	util.NewTimeSeededRand().Read(node.Data())
	return node
}

func TestRecurivePathResolution(t *testing.T) {
	ctx := context.Background()
	dagService := dagmock.Mock()

	a := randNode()
	b := randNode()
	c := randNode()

	err := b.AddNodeLink("grandchild", c)
	if err != nil {
		t.Fatal(err)
	}

	err = a.AddNodeLink("child", b)
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range []node.Node{a, b, c} {
		_, err = dagService.Add(n)
		if err != nil {
			t.Fatal(err)
		}
	}

	aKey := a.Cid()

	segments := []string{aKey.String(), "child", "grandchild"}
	p, err := path.FromSegments("/ipfs/", segments...)
	if err != nil {
		t.Fatal(err)
	}

	resolver := path.NewBasicResolver(dagService)
	node, err := resolver.ResolvePath(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	cKey := c.Cid()
	key := node.Cid()
	if key.String() != cKey.String() {
		t.Fatal(fmt.Errorf(
			"recursive path resolution failed for %s: %s != %s",
			p.String(), key.String(), cKey.String()))
	}
}

package dagcmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	coredag "github.com/ipfs/go-ipfs/core/coredag"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"

	node "gx/ipfs/QmPAKbSsgEX5B6fpmxa61jXYnoWzZr5sNafd3qgPiSH8Uv/go-ipld-format"
	cid "gx/ipfs/Qma4RJSuh7mMeJQYCqMbKzekn6EwBo7HEs5AQYjVRMQATB/go-cid"
	ipldcbor "gx/ipfs/Qmcdid3XrCxcoNQUqZKiiKtM7JXxtyipU3izyRqwjFbVWw/go-ipld-cbor"
)

var DagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with ipld dag objects.",
		ShortDescription: `
'ipfs dag' is used for creating and manipulating dag objects.

This subcommand is currently an experimental feature, but it is intended
to deprecate and replace the existing 'ipfs object' command moving forward.
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"put": DagPutCmd,
		"get": DagGetCmd,
	},
}

type OutputObject struct {
	Cid *cid.Cid
}

var DagPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a dag node to ipfs.",
		ShortDescription: `
'ipfs dag put' accepts input from a file or stdin and parses it
into an object of the specified format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("object data", true, false, "The object to put").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "f", "Format that the object will be added as.").Default("cbor"),
		cmds.StringOption("input-enc", "Format that the input object will be.").Default("json"),
		cmds.BoolOption("pin", "Pin this object when adding.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fi, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		ienc, _, _ := req.Option("input-enc").String()
		format, _, _ := req.Option("format").String()
		dopin, _, err := req.Option("pin").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if dopin {
			defer n.Blockstore.PinLock().Unlock()
		}

		nds, err := coredag.ParseInputs(ienc, format, fi)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var c *cid.Cid
		b := n.DAG.Batch()
		for _, nd := range nds {
			cid, err := b.Add(nd)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			c = cid
		}
		if err := b.Commit(); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if dopin {
			n.Pinning.PinWithMode(c, pin.Recursive)

			err := n.Pinning.Flush()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		res.SetOutput(&OutputObject{Cid: c})
	},
	Type: OutputObject{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			oobj, ok := res.Output().(*OutputObject)
			if !ok {
				return nil, fmt.Errorf("expected a different object in marshaler")
			}

			return strings.NewReader(oobj.Cid.String()), nil
		},
	},
}

var DagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a dag node from ipfs.",
		ShortDescription: `
'ipfs dag get' fetches a dag node from ipfs and prints it out in the specifed format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The object to get").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		p, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		obj, rem, err := n.Resolver.ResolveToLastNode(req.Context(), p)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var out interface{} = obj
		if len(rem) > 0 {
			final, _, err := obj.Resolve(rem)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			out = final
		}

		res.SetOutput(out)
	},
}

func convertJsonToType(r io.Reader, format string) (node.Node, error) {
	switch format {
	case "cbor", "dag-cbor":
		return ipldcbor.FromJson(r)
	case "dag-pb", "protobuf":
		return nil, fmt.Errorf("protobuf handling in 'dag' command not yet implemented")
	default:
		return nil, fmt.Errorf("unknown target format: %s", format)
	}
}

func convertRawToType(r io.Reader, format string) (node.Node, error) {
	switch format {
	case "cbor", "dag-cbor":
		data, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}

		return ipldcbor.Decode(data)
	default:
		return nil, fmt.Errorf("unsupported target format for raw input: %s", format)
	}
}

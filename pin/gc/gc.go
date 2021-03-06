package gc

import (
	"context"
	"errors"
	"fmt"

	bstore "github.com/scroot/go-ipfs/blocks/blockstore"
	dag "github.com/scroot/go-ipfs/merkledag"
	pin "github.com/scroot/go-ipfs/pin"

	node "gx/ipfs/QmPAKbSsgEX5B6fpmxa61jXYnoWzZr5sNafd3qgPiSH8Uv/go-ipld-format"
	cid "gx/ipfs/Qma4RJSuh7mMeJQYCqMbKzekn6EwBo7HEs5AQYjVRMQATB/go-cid"
)

// Result represents an incremental output from a garbage collection
// run.  It contains either an error, or the cid of a removed object.
type Result struct {
	KeyRemoved *cid.Cid
	Error      error
}

// GC performs a mark and sweep garbage collection of the blocks in the blockstore
// first, it creates a 'marked' set and adds to it the following:
// - all recursively pinned blocks, plus all of their descendants (recursively)
// - bestEffortRoots, plus all of its descendants (recursively)
// - all directly pinned blocks
// - all blocks utilized internally by the pinner
//
// The routine then iterates over every block in the blockstore and
// deletes any block that is not found in the marked set.
//
func GC(ctx context.Context, bs bstore.GCBlockstore, ls dag.LinkService, pn pin.Pinner, bestEffortRoots []*cid.Cid) <-chan Result {
	unlocker := bs.GCLock()
	ls = ls.GetOfflineLinkService()

	output := make(chan Result, 128)

	go func() {
		defer close(output)
		defer unlocker.Unlock()

		gcs, err := ColoredSet(ctx, pn, ls, bestEffortRoots, output)
		if err != nil {
			output <- Result{Error: err}
			return
		}

		keychan, err := bs.AllKeysChan(ctx)
		if err != nil {
			output <- Result{Error: err}
			return
		}

		errors := false

	loop:
		for {
			select {
			case k, ok := <-keychan:
				if !ok {
					break loop
				}
				if !gcs.Has(k) {
					err := bs.DeleteBlock(k)
					if err != nil {
						errors = true
						output <- Result{Error: &CannotDeleteBlockError{k, err}}
						//log.Errorf("Error removing key from blockstore: %s", err)
						// continue as error is non-fatal
						continue loop
					}
					select {
					case output <- Result{KeyRemoved: k}:
					case <-ctx.Done():
						break loop
					}
				}
			case <-ctx.Done():
				break loop
			}
		}
		if errors {
			output <- Result{Error: ErrCannotDeleteSomeBlocks}
		}
	}()

	return output
}

func Descendants(ctx context.Context, getLinks dag.GetLinks, set *cid.Set, roots []*cid.Cid) error {
	for _, c := range roots {
		set.Add(c)

		// EnumerateChildren recursively walks the dag and adds the keys to the given set
		err := dag.EnumerateChildren(ctx, getLinks, c, set.Visit)
		if err != nil {
			return err
		}
	}

	return nil
}

// ColoredSet computes the set of nodes in the graph that are pinned by the
// pins in the given pinner.
func ColoredSet(ctx context.Context, pn pin.Pinner, ls dag.LinkService, bestEffortRoots []*cid.Cid, output chan<- Result) (*cid.Set, error) {
	// KeySet currently implemented in memory, in the future, may be bloom filter or
	// disk backed to conserve memory.
	errors := false
	gcs := cid.NewSet()
	getLinks := func(ctx context.Context, cid *cid.Cid) ([]*node.Link, error) {
		links, err := ls.GetLinks(ctx, cid)
		if err != nil {
			errors = true
			output <- Result{Error: &CannotFetchLinksError{cid, err}}
		}
		return links, nil
	}
	err := Descendants(ctx, getLinks, gcs, pn.RecursiveKeys())
	if err != nil {
		errors = true
		output <- Result{Error: err}
	}

	bestEffortGetLinks := func(ctx context.Context, cid *cid.Cid) ([]*node.Link, error) {
		links, err := ls.GetLinks(ctx, cid)
		if err != nil && err != dag.ErrNotFound {
			errors = true
			output <- Result{Error: &CannotFetchLinksError{cid, err}}
		}
		return links, nil
	}
	err = Descendants(ctx, bestEffortGetLinks, gcs, bestEffortRoots)
	if err != nil {
		errors = true
		output <- Result{Error: err}
	}

	for _, k := range pn.DirectKeys() {
		gcs.Add(k)
	}

	err = Descendants(ctx, getLinks, gcs, pn.InternalPins())
	if err != nil {
		errors = true
		output <- Result{Error: err}
	}

	if errors {
		return nil, ErrCannotFetchAllLinks
	}

	return gcs, nil
}

var ErrCannotFetchAllLinks = errors.New("garbage collection aborted: could not retrieve some links")

var ErrCannotDeleteSomeBlocks = errors.New("garbage collection incomplete: could not delete some blocks")

type CannotFetchLinksError struct {
	Key *cid.Cid
	Err error
}

func (e *CannotFetchLinksError) Error() string {
	return fmt.Sprintf("could not retrieve links for %s: %s", e.Key, e.Err)
}

type CannotDeleteBlockError struct {
	Key *cid.Cid
	Err error
}

func (e *CannotDeleteBlockError) Error() string {
	return fmt.Sprintf("could not remove %s: %s", e.Key, e.Err)
}

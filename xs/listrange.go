// Package xs contains eXtended actions (xactions) except storage services
// (mirror, ec) and extensions (downloader, lru).
/*
 * Copyright (c) 2018-2021, NVIDIA CORPORATION. All rights reserved.
 */
package xs

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/NVIDIA/aistore/3rdparty/glog"
	"github.com/NVIDIA/aistore/cluster"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/cmn/cos"
	"github.com/NVIDIA/aistore/cmn/debug"
	"github.com/NVIDIA/aistore/objwalk"
	"github.com/NVIDIA/aistore/xaction"
	"github.com/NVIDIA/aistore/xreg"
)

// Assorted list/range xactions: evict, delete, prefetch multiple objects
//
// Supported ranges:
//   1. bash-extension style: `file-{0..100}`
//   2. at-style: `file-@100`
//   3. if none of the above, fall back to just prefix matching
//
// NOTE: rebalancing vs performance comment below.

// common for all list-range
type (
	// one multi-object operation work item
	lrwi interface {
		do(*cluster.LOM, *lriterator)
	}
	// two selected methods that lriterator needs for itself (a strict subset of cluster.Xact)
	lrxact interface {
		Bck() *cluster.Bck
		Aborted() bool
		Finished() bool
	}
	// common mult-obj operation context
	// common iterateList()/iterateRange() logic
	lriterator struct {
		xact    lrxact
		t       cluster.Target
		ctx     context.Context
		msg     *cmn.ListRangeMsg
		freeLOM bool // free LOM upon return from lriterator.do()
	}
)

// concrete list-range type xactions (see also: archive.go)
type (
	evdFactory struct {
		xreg.RenewBase
		xact *evictDelete
		kind string
		msg  *cmn.ListRangeMsg
	}
	evictDelete struct {
		xaction.XactBase
		lriterator
	}
	prfFactory struct {
		xreg.RenewBase
		xact *prefetch
		msg  *cmn.ListRangeMsg
	}
	prefetch struct {
		xaction.XactBase
		lriterator
	}

	TestXFactory struct{ prfFactory } // tests only
)

// interface guard
var (
	_ cluster.Xact = (*evictDelete)(nil)
	_ cluster.Xact = (*prefetch)(nil)

	_ xreg.Renewable = (*evdFactory)(nil)
	_ xreg.Renewable = (*prfFactory)(nil)

	_ lrwi = (*evictDelete)(nil)
	_ lrwi = (*prefetch)(nil)
)

////////////////
// lriterator //
////////////////

func (r *lriterator) init(xact lrxact, t cluster.Target, msg *cmn.ListRangeMsg, freeLOM bool) {
	r.xact = xact
	r.t = t
	r.ctx = context.Background()
	r.msg = msg
	r.freeLOM = freeLOM
}

func (r *lriterator) iterateRange(wi lrwi, smap *cluster.Smap) error {
	pt, err := cos.NewParsedTemplate(r.msg.Template)
	if err != nil {
		return err
	}
	if len(pt.Ranges) != 0 {
		return r.iterateTemplate(smap, &pt, wi)
	}
	return r.iteratePrefix(smap, pt.Prefix, wi)
}

func (r *lriterator) iterateTemplate(smap *cluster.Smap, pt *cos.ParsedTemplate, wi lrwi) error {
	getNext := pt.Iter()
	for objName, hasNext := getNext(); hasNext; objName, hasNext = getNext() {
		if r.xact.Aborted() || r.xact.Finished() {
			return nil
		}
		lom := cluster.AllocLOM(objName)
		err := r.do(lom, wi, smap)
		if err != nil {
			cluster.FreeLOM(lom)
			return err
		}
		if r.freeLOM {
			cluster.FreeLOM(lom)
		}
	}
	return nil
}

func (r *lriterator) iteratePrefix(smap *cluster.Smap, prefix string, wi lrwi) error {
	var (
		objList *cmn.BucketList
		err     error
		bck     = r.xact.Bck()
	)
	if err := bck.Init(r.t.Bowner()); err != nil {
		return err
	}
	bremote := bck.IsRemote()
	if !bremote {
		smap = nil // not needed
	} else if bck.IsHTTP() {
		return fmt.Errorf("cannot list bucket %s for prefix %q (plain HTTP buckets are not list-able) - use alternative templating",
			bck, prefix)
	}
	msg := &cmn.ListObjsMsg{Prefix: prefix, Props: cmn.GetPropsStatus}
	for {
		if r.xact.Aborted() || r.xact.Finished() {
			break
		}
		if bremote {
			objList, _, err = r.t.Backend(bck).ListObjects(bck, msg)
		} else {
			walk := objwalk.NewWalk(r.ctx, r.t, bck, msg)
			objList, err = walk.DefaultLocalObjPage(msg)
		}
		if err != nil {
			return err
		}
		for _, be := range objList.Entries {
			if !be.IsStatusOK() {
				continue
			}
			if r.xact.Aborted() || r.xact.Finished() {
				return nil
			}
			lom := cluster.AllocLOM(be.Name)
			err := r.do(lom, wi, smap)
			if err != nil {
				cluster.FreeLOM(lom)
				return err
			}
			if r.freeLOM {
				cluster.FreeLOM(lom)
			}
		}

		// Stop when the last page is reached.
		if objList.ContinuationToken == "" {
			break
		}
		// Update `ContinuationToken` for the next request.
		msg.ContinuationToken = objList.ContinuationToken
	}
	return nil
}

func (r *lriterator) iterateList(wi lrwi, smap *cluster.Smap) error {
	for _, objName := range r.msg.ObjNames {
		if r.xact.Aborted() || r.xact.Finished() {
			break
		}
		lom := cluster.AllocLOM(objName)
		err := r.do(lom, wi, smap)
		if err != nil {
			cluster.FreeLOM(lom)
			return err
		}
		if r.freeLOM {
			cluster.FreeLOM(lom)
		}
	}
	return nil
}

func (r *lriterator) do(lom *cluster.LOM, wi lrwi, smap *cluster.Smap) error {
	if err := lom.Init(r.xact.Bck().Bck); err != nil {
		return err
	}
	if smap != nil {
		_, local, err := lom.HrwTarget(smap)
		if err != nil {
			return err
		}
		if !local {
			return nil
		}
	}
	wi.do(lom, r)
	return nil
}

//////////////////
// evict/delete //
//////////////////

func (p *evdFactory) New(args xreg.Args, bck *cluster.Bck) xreg.Renewable {
	msg := args.Custom.(*cmn.ListRangeMsg)
	debug.Assert(!msg.IsList() || !msg.HasTemplate())
	np := &evdFactory{RenewBase: xreg.RenewBase{Args: args, Bck: bck}, kind: p.kind, msg: msg}
	return np
}

func (p *evdFactory) Start() error {
	p.xact = newEvictDelete(&p.Args, p.kind, p.Bck, p.msg)
	return nil
}

func (p *evdFactory) Kind() string      { return p.kind }
func (p *evdFactory) Get() cluster.Xact { return p.xact }

func (*evdFactory) WhenPrevIsRunning(xreg.Renewable) (xreg.WPR, error) {
	return xreg.WprKeepAndStartNew, nil
}

func newEvictDelete(xargs *xreg.Args, kind string, bck *cluster.Bck, msg *cmn.ListRangeMsg) (ed *evictDelete) {
	ed = &evictDelete{}
	ed.lriterator.init(ed, xargs.T, msg, true /*freeLOM*/)
	ed.InitBase(xargs.UUID, kind, bck)
	return
}

func (r *evictDelete) Run(*sync.WaitGroup) {
	var (
		err  error
		smap = r.t.Sowner().Get()
	)
	if r.msg.IsList() {
		err = r.iterateList(r, smap)
	} else {
		err = r.iterateRange(r, smap)
	}
	r.Finish(err)
}

func (r *evictDelete) do(lom *cluster.LOM, _ *lriterator) {
	errCode, err := r.t.DeleteObject(lom, r.Kind() == cmn.ActEvictObjects)
	if errCode == http.StatusNotFound {
		return
	}
	if err != nil {
		if !cmn.IsErrObjNought(err) {
			glog.Warning(err)
		}
		return
	}
	r.ObjsAdd(1, lom.SizeBytes(true)) // loaded and evicted
}

//////////////
// prefetch //
//////////////

func (*prfFactory) New(args xreg.Args, bck *cluster.Bck) xreg.Renewable {
	msg := args.Custom.(*cmn.ListRangeMsg)
	debug.Assert(!msg.IsList() || !msg.HasTemplate())
	np := &prfFactory{RenewBase: xreg.RenewBase{Args: args, Bck: bck}, msg: msg}
	return np
}

func (p *prfFactory) Start() error {
	b := cluster.NewBckEmbed(p.Bck.Bck)
	if err := b.Init(p.Args.T.Bowner()); err != nil {
		if cmn.IsErrRemoteBckNotFound(err) {
			glog.Warning(err) // may show up later via ais/prxtrybck.go logic
		} else {
			return err
		}
	} else if b.IsAIS() {
		glog.Errorf("bucket %q: can only prefetch remote buckets", b)
		return fmt.Errorf("bucket %q: can only prefetch remote buckets", b)
	}
	p.xact = newPrefetch(&p.Args, p.Kind(), p.Bck, p.msg)
	return nil
}

func (*prfFactory) Kind() string        { return cmn.ActPrefetchObjects }
func (p *prfFactory) Get() cluster.Xact { return p.xact }

func (*prfFactory) WhenPrevIsRunning(xreg.Renewable) (xreg.WPR, error) {
	return xreg.WprKeepAndStartNew, nil
}

func newPrefetch(xargs *xreg.Args, kind string, bck *cluster.Bck, msg *cmn.ListRangeMsg) (prf *prefetch) {
	prf = &prefetch{}
	prf.lriterator.init(prf, xargs.T, msg, true /*freeLOM*/)
	prf.InitBase(xargs.UUID, kind, bck)
	prf.lriterator.xact = prf
	return
}

func (r *prefetch) Run(*sync.WaitGroup) {
	var (
		err  error
		smap = r.t.Sowner().Get()
	)
	if r.msg.IsList() {
		err = r.iterateList(r, smap)
	} else {
		err = r.iterateRange(r, smap)
	}
	r.Finish(err)
}

func (r *prefetch) do(lom *cluster.LOM, _ *lriterator) {
	if err := lom.Load(true /*cache it*/, false /*locked*/); err != nil {
		if !cmn.IsObjNotExist(err) {
			return
		}
	} else if lom.VersionConf().ValidateWarmGet {
		if equal, _, err := r.t.CompareObjects(r.ctx, lom); equal || err != nil {
			return
		}
	} else {
		return // simply exists
	}
	// NOTE: not setting atime as prefetching does not mean the object is being accessed.
	// On the other hand, zero atime makes the object's lifespan in the cache too short - the first
	// housekeeping traversal will remove it. Set special `-now` value for subsequent correction.
	// (see cluster/lom_cache_hk.go)
	lom.SetAtimeUnix(-time.Now().UnixNano())
	if _, err := r.t.GetCold(r.ctx, lom, cmn.OwtGetTryLock); err != nil {
		if err != cmn.ErrSkip {
			glog.Warning(err)
		}
		return
	}
	if verbose {
		glog.Infof("prefetch: %s", lom)
	}
	r.ObjsAdd(1, lom.SizeBytes())
}

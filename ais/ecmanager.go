// Package ais provides core functionality for the AIStore object storage.
/*
 * Copyright (c) 2018, NVIDIA CORPORATION. All rights reserved.
 */
//
// extended action aka xaction
//
package ais

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/NVIDIA/aistore/3rdparty/glog"
	"github.com/NVIDIA/aistore/cluster"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/ec"
	"github.com/NVIDIA/aistore/fs"
	"github.com/NVIDIA/aistore/transport"
)

type ecManager struct {
	sync.RWMutex
	bndlOnce   sync.Once
	t          *targetrunner
	xacts      map[string]*ec.Xacts // bckName -> xact map, only local buckets allowed, no naming collisions
	bowner     *bmdowner            // bucket manager
	bckMD      *bucketMD            // bucket metadata, used to get EC enabled/disabled information
	netReq     string               // network used to send object request
	netResp    string               // network used to send/receive slices
	reqBundle  *transport.StreamBundle
	respBundle *transport.StreamBundle
}

var ECM *ecManager

func newECM(t *targetrunner) *ecManager {
	config := cmn.GCO.Get()
	netReq, netResp := cmn.NetworkIntraControl, cmn.NetworkIntraData
	if !config.Net.UseIntraControl {
		netReq = cmn.NetworkPublic
	}
	if !config.Net.UseIntraData {
		netResp = cmn.NetworkPublic
	}

	ECM = &ecManager{
		netReq:  netReq,
		netResp: netResp,
		t:       t,
		bowner:  t.bmdowner,
		xacts:   make(map[string]*ec.Xacts),
		bckMD:   t.bmdowner.get(),
	}

	for _, bck := range ECM.bckMD.LBmap {
		if bck.EC.Enabled {
			ECM.bndlOnce.Do(ECM.initECBundles)
			break
		}
	}

	var err error
	if _, err = transport.Register(ECM.netReq, ec.ReqStreamName, ECM.makeRecvRequest()); err != nil {
		glog.Errorf("Failed to register recvRequest: %v", err)
		return nil
	}
	if _, err = transport.Register(ECM.netResp, ec.RespStreamName, ECM.makeRecvResponse()); err != nil {
		glog.Errorf("Failed to register respResponse: %v", err)
		return nil
	}

	return ECM
}

func (mgr *ecManager) initECBundles() {
	cmn.AssertMsg(mgr.reqBundle == nil && mgr.respBundle == nil, "EC Bundles have been already initialized")

	cbReq := func(hdr transport.Header, reader io.ReadCloser, err error) {
		if err != nil {
			glog.Errorf("Failed to request %s/%s: %v", hdr.Bucket, hdr.Objname, err)
		}
	}

	client := transport.NewDefaultClient()
	extraReq := transport.Extra{Callback: cbReq}

	reqSbArgs := transport.SBArgs{
		Multiplier: transport.IntraBundleMultiplier,
		Extra:      &extraReq,
		Network:    mgr.netReq,
		Trname:     ec.ReqStreamName,
	}

	respSbArgs := transport.SBArgs{
		Multiplier: transport.IntraBundleMultiplier,
		Trname:     ec.RespStreamName,
		Network:    mgr.netResp,
	}

	mgr.reqBundle = transport.NewStreamBundle(mgr.t.smapowner, mgr.t.si, client, reqSbArgs)
	mgr.respBundle = transport.NewStreamBundle(mgr.t.smapowner, mgr.t.si, client, respSbArgs)
}

func (mgr *ecManager) newGetXact(bucket string) *ec.XactGet {
	return ec.NewGetXact(mgr.t, mgr.t.bmdowner,
		mgr.t.smapowner, mgr.t.si, bucket, mgr.reqBundle, mgr.respBundle)
}

func (mgr *ecManager) newPutXact(bucket string) *ec.XactPut {
	return ec.NewPutXact(mgr.t, mgr.t.bmdowner,
		mgr.t.smapowner, mgr.t.si, bucket, mgr.reqBundle, mgr.respBundle)
}

func (mgr *ecManager) newReqXact(bucket string) *ec.XactRespond {
	return ec.NewRespondXact(mgr.t, mgr.t.bmdowner,
		mgr.t.smapowner, mgr.t.si, bucket, mgr.reqBundle, mgr.respBundle)
}

func (mgr *ecManager) restoreBckGetXact(bckName string) *ec.XactGet {
	xact := mgr.getBckXacts(bckName).Get()
	if xact == nil || xact.Finished() {
		xact = mgr.t.xactions.renewGetEC(bckName)
		mgr.getBckXacts(bckName).SetGet(xact)
	}

	return xact
}

func (mgr *ecManager) restoreBckPutXact(bckName string) *ec.XactPut {
	xact := mgr.getBckXacts(bckName).Put()
	if xact == nil || xact.Finished() {
		xact = mgr.t.xactions.renewPutEC(bckName)
		mgr.getBckXacts(bckName).SetPut(xact)
	}

	return xact
}

func (mgr *ecManager) restoreBckReqXact(bckName string) *ec.XactRespond {
	xact := mgr.getBckXacts(bckName).Req()
	if xact == nil || xact.Finished() {
		xact = mgr.t.xactions.renewRespondEC(bckName)
		mgr.getBckXacts(bckName).SetReq(xact)
	}

	return xact
}

func (mgr *ecManager) getBckXacts(bckName string) *ec.Xacts {
	mgr.RLock()
	defer mgr.RUnlock()

	xacts, ok := mgr.xacts[bckName]

	if !ok {
		xacts = &ec.Xacts{}
		mgr.xacts[bckName] = xacts
	}

	return xacts
}

// A function to process command requests from other targets
func (mgr *ecManager) makeRecvRequest() transport.Receive {
	return func(w http.ResponseWriter, hdr transport.Header, object io.Reader, err error) {
		if err != nil {
			glog.Errorf("Request failed: %v", err)
			return
		}
		// check if the header contains a valid request
		if len(hdr.Opaque) == 0 {
			glog.Error("Empty request")
			return
		}

		iReq := ec.IntraReq{}
		if err := iReq.Unmarshal(hdr.Opaque); err != nil {
			glog.Errorf("Failed to unmarshal request: %v", err)
			return
		}

		// command requests should not have a body, but if it has,
		// the body must be drained to avoid errors
		if hdr.ObjAttrs.Size != 0 {
			if _, err := ioutil.ReadAll(object); err != nil {
				glog.Errorf("Failed to read request body: %v", err)
				return
			}
		}

		mgr.restoreBckReqXact(hdr.Bucket).DispatchReq(iReq, hdr.Bucket, hdr.Objname)
	}
}

// A function to process big chunks of data (replica/slice/meta) sent from other targets
func (mgr *ecManager) makeRecvResponse() transport.Receive {
	return func(w http.ResponseWriter, hdr transport.Header, object io.Reader, err error) {
		if err != nil {
			glog.Errorf("Receive failed: %v", err)
			return
		}
		// check if the request is valid
		if len(hdr.Opaque) == 0 {
			glog.Error("Empty request")
			return
		}

		iReq := ec.IntraReq{}
		if err := iReq.Unmarshal(hdr.Opaque); err != nil {
			glog.Errorf("Failed to unmarshal request: %v", err)
			return
		}

		switch iReq.Act {
		case ec.ReqPut:
			mgr.restoreBckReqXact(hdr.Bucket).DispatchResp(iReq, hdr.Bucket, hdr.Objname, hdr.ObjAttrs, object)
		case ec.ReqMeta, ec.RespPut:
			mgr.restoreBckGetXact(hdr.Bucket).DispatchResp(iReq, hdr.Bucket, hdr.Objname, hdr.ObjAttrs, object)
		default:
			glog.Errorf("Unknown EC response action %d", iReq.Act)
		}
	}
}

func (mgr *ecManager) EncodeObject(lom *cluster.LOM) error {
	if !lom.BckProps.EC.Enabled {
		return ec.ErrorECDisabled
	}
	cmn.Assert(lom.FQN != "")
	cmn.Assert(lom.ParsedFQN.MpathInfo != nil && lom.ParsedFQN.MpathInfo.Path != "")

	if _, oos := lom.T.AvgCapUsed(nil); oos {
		return errors.New("OOS") // out of space
	}
	spec, _ := fs.CSM.FileSpec(lom.FQN)
	if spec != nil && !spec.PermToProcess() {
		return nil
	}

	req := &ec.Request{
		Action: ec.ActSplit,
		IsCopy: ec.IsECCopy(lom.Size, &lom.BckProps.EC),
		LOM:    lom,
	}

	if errstr := lom.Fill("", cluster.LomAtime|cluster.LomVersion|cluster.LomCksum); errstr != "" {
		return errors.New(errstr)
	}

	mgr.restoreBckPutXact(lom.Bucket).Encode(req)

	return nil
}

func (mgr *ecManager) CleanupObject(lom *cluster.LOM) {
	if !lom.BckProps.EC.Enabled {
		return
	}
	cmn.Assert(lom.FQN != "")
	cmn.Assert(lom.ParsedFQN.MpathInfo != nil && lom.ParsedFQN.MpathInfo.Path != "")
	req := &ec.Request{
		Action: ec.ActDelete,
		LOM:    lom,
	}

	mgr.restoreBckPutXact(lom.Bucket).Cleanup(req)
}

func (mgr *ecManager) RestoreObject(lom *cluster.LOM) error {
	if !lom.BckProps.EC.Enabled {
		return ec.ErrorECDisabled
	}

	cmn.Assert(lom.ParsedFQN.MpathInfo != nil && lom.ParsedFQN.MpathInfo.Path != "")
	req := &ec.Request{
		Action: ec.ActRestore,
		LOM:    lom,
		ErrCh:  make(chan error), // unbuffered
	}

	mgr.restoreBckGetXact(lom.Bucket).Decode(req)

	// wait for EC completes restoring the object
	return <-req.ErrCh
}

// disableBck starts to reject new EC requests, rejects pending ones
func (mgr *ecManager) disableBck(bckName string) {
	mgr.restoreBckGetXact(bckName).ClearRequests()
	mgr.restoreBckPutXact(bckName).ClearRequests()
}

// enableBck aborts xact disable and starts to accept new EC requests
// enableBck uses the same channel as disableBck, so order of executing them is the same as
// order which they arrived to a target in
func (mgr *ecManager) enableBck(bckName string) {
	mgr.restoreBckGetXact(bckName).EnableRequests()
	mgr.restoreBckPutXact(bckName).EnableRequests()
}

func (mgr *ecManager) BucketsMDChanged() {
	newBckMD := mgr.bowner.get()
	oldBckMD := mgr.bckMD
	if newBckMD.Version <= mgr.bckMD.Version {
		return
	}

	mgr.bckMD = newBckMD

	if newBckMD.ecUsed() && !oldBckMD.ecUsed() {
		// init EC streams if there were not initialized on the start
		// no need to close them when last EC bucket is disabled
		// as they close itself on idle
		mgr.bndlOnce.Do(mgr.initECBundles)
	}

	for bckName, newBck := range newBckMD.LBmap {
		// Disable EC for buckets that existed and have changed EC.Enabled to false
		// Enable EC for buckets that existed and have change EC.Enabled to true
		if oldBck, existed := oldBckMD.LBmap[bckName]; existed {
			if !oldBck.EC.Enabled && newBck.EC.Enabled {
				mgr.enableBck(bckName)
			} else if oldBck.EC.Enabled && !newBck.EC.Enabled {
				mgr.disableBck(bckName)
			}
		}
	}
}

#
# Copyright (c) 2021-2023, NVIDIA CORPORATION. All rights reserved.
#

from __future__ import annotations  # pylint: disable=unused-variable
import base64

from typing import Any, Mapping, List, Optional, Dict

from pydantic import BaseModel, validator

from aistore.sdk.const import PROVIDER_AIS


# pylint: disable=too-few-public-methods,unused-variable,missing-function-docstring


class Namespace(BaseModel):
    """
    A bucket namespace
    """

    uuid: str = ""
    name: str = ""


class ActionMsg(BaseModel):
    """
    Represents the action message passed by the client via json
    """

    action: str
    name: str = ""
    value: Any = None


class HttpError(BaseModel):
    """
    Represents the errors returned by the API
    """

    status: int
    message: str = ""
    method: str = ""
    url_path: str = ""
    remote_addr: str = ""
    caller: str = ""
    node: str = ""


class NetInfo(BaseModel):
    """
    Represents a set of network-related info
    """

    node_hostname: str = ""
    daemon_port: str = ""
    direct_url: str = ""


class Snode(BaseModel):
    """
    Represents a system node
    """

    daemon_id: str
    daemon_type: str
    public_net: NetInfo = None
    intra_control_net: NetInfo = None
    intra_data_net: NetInfo = None
    flags: int = 0


class Smap(BaseModel):
    """
    Represents a system map
    """

    tmap: Mapping[str, Snode]
    pmap: Mapping[str, Snode]
    proxy_si: Snode
    version: int = 0
    uuid: str = ""
    creation_time: str = ""


class BucketEntry(BaseModel):
    """
    Represents a single entry in a bucket -- an object
    """

    name: str
    size: int = 0
    checksum: str = ""
    atime: str = ""
    version: str = ""
    target_url: str = ""
    copies: int = 0
    flags: int = 0
    object: "Object" = None

    def is_cached(self):
        return (self.flags & (1 << 6)) != 0

    def is_ok(self):
        return (self.flags & ((1 << 5) - 1)) == 0


class BucketList(BaseModel):
    """
    Represents the response when getting a list of bucket items, containing a list of BucketEntry objects
    """

    uuid: str
    entries: Optional[List[BucketEntry]] = []
    continuation_token: str
    flags: int

    def get_entries(self):
        return self.entries

    # pylint: disable=no-self-argument
    @validator("entries")
    def set_entries(cls, entries):
        if entries is None:
            entries = []
        return entries


class BucketModel(BaseModel):
    """
    Represents the response from the API containing bucket info
    """

    name: str
    provider: str = PROVIDER_AIS
    namespace: Namespace = None

    def as_dict(self):
        dict_rep = {"name": self.name, "provider": self.provider}
        if self.namespace:
            dict_rep["namespace"] = self.namespace
        return dict_rep


class JobArgs(BaseModel):
    """
    Represents the set of args to pass when making a job-related request
    """

    id: str = ""
    kind: str = ""
    daemon_id: str = ""
    bucket: BucketModel = None
    buckets: List[BucketModel] = None
    only_running: bool = False

    def as_dict(self):
        return {
            "ID": self.id,
            "Kind": self.kind,
            "DaemonID": self.daemon_id,
            "Bck": self.bucket,
            "Buckets": self.buckets,
            "OnlyRunning": self.only_running,
        }


class JobStatus(BaseModel):
    """
    Represents the response of an API query to fetch job status
    """

    uuid: str = ""
    err: str = ""
    end_time: int = 0
    aborted: bool = False


class ETL(BaseModel):  # pylint: disable=too-few-public-methods,unused-variable
    """
    Represents the API response when querying an ETL
    """

    id: str = ""
    obj_count: int = 0
    in_bytes: int = 0
    out_bytes: int = 0


class ETLDetails(BaseModel):
    """
    Represents the API response of queries on single ETL details
    """

    id: str
    communication: str
    timeout: str
    code: Optional[bytes]
    spec: Optional[str]
    dependencies: Optional[str]
    runtime: Optional[str]  # see ext/etl/runtime/all.go
    chunk_size: int = 0

    @validator("code")
    def set_code(cls, code):  # pylint: disable=no-self-argument
        if code is not None:
            code = base64.b64decode(code)
        return code

    @validator("spec")
    def set_spec(cls, spec):  # pylint: disable=no-self-argument
        if spec is not None:
            spec = base64.b64decode(spec)
        return spec


class PromoteAPIArgs(BaseModel):
    """
    Represents the set of args the sdk will pass to AIStore when making a promote request and
    provides conversion to the expected json format
    """

    target_id: str = ""
    source_path: str = ""
    object_name: str = ""
    recursive: bool = False
    overwrite_dest: bool = False
    delete_source: bool = False
    src_not_file_share: bool = False

    def as_dict(self):
        return {
            "tid": self.target_id,
            "src": self.source_path,
            "obj": self.object_name,
            "rcr": self.recursive,
            "ovw": self.overwrite_dest,
            "dls": self.delete_source,
            "notshr": self.src_not_file_share,
        }


class JobStats(BaseModel):
    """
    Structure for job statistics
    """

    objects: int = 0
    bytes: int = 0
    out_objects: int = 0
    out_bytes: int = 0
    in_objects: int = 0
    in_bytes: int = 0


class JobSnapshot(BaseModel):
    """
    Structure for the data returned when querying a single job on a single target node
    """

    id: str = ""
    kind: str = ""
    start_time: str = ""
    end_time: str = ""
    bucket: BucketModel = None
    source_bck: str = ""
    dest_bck: str = ""
    rebalance_id: str = ""
    stats: JobStats = None
    aborted: bool = False
    is_idle: bool = False


class CopyBckMsg(BaseModel):
    """
    API message structure for copying a bucket
    """

    prepend: str
    dry_run: bool
    force: bool

    def as_dict(self):
        return {"prepend": self.prepend, "dry_run": self.dry_run, "force": self.force}


class ListObjectsMsg(BaseModel):
    """
    API message structure for listing objects in a bucket
    """

    prefix: str
    page_size: int
    uuid: str
    props: str
    continuation_token: str

    def as_dict(self):
        return {
            "prefix": self.prefix,
            "pagesize": self.page_size,
            "uuid": self.uuid,
            "props": self.props,
            "continuation_token": self.continuation_token,
        }


class TransformBckMsg(BaseModel):
    """
    API message structure for requesting an etl transform on a bucket
    """

    etl_name: str
    timeout: str

    def as_dict(self):
        return {"id": self.etl_name, "request_timeout": self.timeout}


class TCBckMsg(BaseModel):
    """
    API message structure for transforming or copying between buckets.
    Can be used on its own for an entire bucket or encapsulated in TCMultiObj to apply only to a selection of objects
    """

    ext: Dict[str, str] = None
    copy_msg: CopyBckMsg = None
    transform_msg: TransformBckMsg = None

    def as_dict(self):
        dict_rep = {}
        if self.ext:
            dict_rep["ext"] = self.ext
        if self.copy_msg:
            for key, val in self.copy_msg.as_dict().items():
                dict_rep[key] = val
        if self.transform_msg:
            for key, val in self.transform_msg.as_dict().items():
                dict_rep[key] = val
        return dict_rep


class TCMultiObj(BaseModel):
    """
    API message structure for transforming or copying multiple objects between buckets
    """

    to_bck: BucketModel
    tc_msg: TCBckMsg = None
    continue_on_err: bool
    object_selection: dict

    def as_dict(self):
        dict_rep = self.object_selection
        if self.tc_msg:
            for key, val in self.tc_msg.as_dict().items():
                dict_rep[key] = val
        dict_rep["tobck"] = self.to_bck.as_dict()
        dict_rep["coer"] = self.continue_on_err
        return dict_rep

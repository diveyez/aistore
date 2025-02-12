---
layout: post
title: CLUSTER
permalink: /docs/cli/cluster
redirect_from:
 - /cli/cluster.md/
 - /docs/cli/cluster.md/
---

# Introduction

The `ais cluster` command supports the following subcommands:

```console
$ ais cluster <TAB-TAB>
add-remove-nodes  decommission  rebalance  remote-attach  remote-detach  set-primary  show  shutdown
```

As always, each above subcommand will have its own help and usage examples - the latter possibly spread across multiple documents.

> For any keyword or text of any kind, you can easily look up examples and descriptions (if available) via a simple `find`, for instance:

```console
$ find . -type f -name "*.md" | xargs grep "ais.*mountpath"
```

Note that there is a single CLI command to [grow](#join-a-node) a cluster, and multiple commands to scale it down.

Scaling down can be done gracefully or forcefully, and also temporarily or permanently.

For background, usage examples, and details, please see [this document](/docs/leave_cluster.md).

# CLI Reference for Cluster and Node (Daemon) management
This section lists cluster and node management operations within the AIS CLI, via `ais cluster`.

## Table of Contents
- [Cluster or Daemon status](#cluster-or-daemon-status)
- [Show cluster map](#show-cluster-map)
- [Show cluster stats](#show-cluster-stats)
- [Show disk stats](#show-disk-stats)
- [Join a node](#join-a-node)
- [Remove a node](#remove-a-node)
- [Remote AIS cluster](#remote-ais-cluster)
  - [Attach remote cluster](#attach-remote-cluster)
  - [Detach remote cluster](#detach-remote-cluster)
  - [Show remote clusters](#show-remote-clusters)

## Cluster or Daemon status

`ais show cluster [DAEMON_TYPE|DAEMON_ID]`

Display information about `DAEMON_ID` or all nodes of `DAEMON_TYPE`. `DAEMON_TYPE` is either `proxy` or `target`.
If you give no arguments to `ais show cluster`, information about all daemons in the AIS cluster is displayed.

> Note: Like all other `ais show` commands, `ais show cluster` is aliased to `ais cluster show` for ease of use.
> Both variations are used interchangeably throughout the documentation.

### Options

| Flag | Type | Description | Default |
| --- | --- | --- | --- |
| `--json, -j` | `bool` | Output in JSON format | `false` |
| `--count` | `int` | Can be used in combination with `--refresh` option to limit the number of generated reports | `1` |
| `--refresh` | `duration` | Refresh interval - time duration between reports. The usual unit suffixes are supported and include `m` (for minutes), `s` (seconds), `ms` (milliseconds) | ` ` |
| `--no-headers` | `bool` | Display tables without headers | `false` |

### Examples

```console
$ ais show cluster
PROXY            MEM USED %      MEM AVAIL       UPTIME
pufGp8080[P]     0.28%           15.43GiB        17m
ETURp8083        0.26%           15.43GiB        17m
sgahp8082        0.26%           15.43GiB        17m
WEQRp8084        0.27%           15.43GiB        17m
Watdp8081        0.26%           15.43GiB        17m

TARGET           MEM USED %      MEM AVAIL       CAP USED %      CAP AVAIL       CPU USED %      REBALANCE       UPTIME
iPbHt8088        0.28%           15.43GiB        14.00%          1.178TiB        0.13%           -               17m
Zgmlt8085        0.28%           15.43GiB        14.00%          1.178TiB        0.13%           -               17m
oQZCt8089        0.28%           15.43GiB        14.00%          1.178TiB        0.14%           -               17m
dIzMt8086        0.28%           15.43GiB        14.00%          1.178TiB        0.13%           -               17m
YodGt8087        0.28%           15.43GiB        14.00%          1.178TiB        0.14%           -               17m

Summary:
 Proxies:       5 (0 - unelectable)
 Targets:       5
 Primary Proxy: pufGp8080
 Smap Version:  14
 Deployment:    dev
```

## Show cluster map

`ais show cluster smap [DAEMON_ID]`

Show a copy of the cluster map (smap) present on `DAEMON_ID`.
If `DAEMON_ID` is not set, it will show the smap of the daemon that the `AIS_ENDPOINT` points at by default.

### Options

| Flag | Type | Description | Default |
| --- | --- | --- | --- |
| `--count` | `int` | Can be used in combination with `--refresh` option to limit the number of generated reports | `1` |
| `--refresh` | `duration` | Refresh interval - time duration between reports. The usual unit suffixes are supported and include `m` (for minutes), `s` (seconds), `ms` (milliseconds) | ` ` |
| `--json, -j` | `bool` | Output in JSON format | `false` |

### Examples

#### Show smap from a given node

Ask a specific node for its cluster map (Smap) replica:

```console
$ ais show cluster smap <TAB-TAB>
... p[ETURp8083] ...

$ ais show cluster smap p[ETURp8083]
NODE             TYPE    PUBLIC URL
ETURp8083        proxy   http://127.0.0.1:8083
WEQRp8084        proxy   http://127.0.0.1:8084
Watdp8081        proxy   http://127.0.0.1:8081
pufGp8080[P]     proxy   http://127.0.0.1:8080
sgahp8082        proxy   http://127.0.0.1:8082

NODE             TYPE    PUBLIC URL
YodGt8087        target  http://127.0.0.1:8087
Zgmlt8085        target  http://127.0.0.1:8085
dIzMt8086        target  http://127.0.0.1:8086
iPbHt8088        target  http://127.0.0.1:8088
oQZCt8089        target  http://127.0.0.1:8089

Non-Electable:

Primary Proxy: pufGp8080
Proxies: 5       Targets: 5      Smap Version: 14
```

## Show cluster stats

`ais show cluster stats [DAEMON_ID] [STATS_FILTER]`

Show current (live) statistics, including:
* utilization percentages
* used and available storage capacity
* variety of latencies, including list-objects and intra-cluster control comm.
* networking stats (transmitted and received object numbers and total bytes), and more.

For a broader background and details, please see [Monitoring AIStore with Prometheus](/docs/prometheus.md).

You can give a specific node (denoted by `DAEMON_ID`) as the argument for `ais show cluster stats` to narrow the scope of the statistics, or you can view stats for the entire cluster. 
In the latter case, the command displays aggregated counters and cluster-wide averages, where applicable.

### Options

| Flag | Type | Description | Default |
| --- | --- | --- | --- |
| `--json, -j` | `bool` | JSON format           | `false` |
| `--raw`    | `bool` | display exact raw statistics values instead of human-readable ones | `false` |
| `--refresh` | `duration` | refresh interval - time duration between reports. The usual unit suffixes are supported and include `m` (for minutes), `s` (seconds), `ms` (milliseconds). Press `Ctrl+C` to stop monitoring | ` ` |

### Examples

Monitor cluster performance with a 10 seconds refresh interval:

```console
$ ais show cluster stats --refresh 10s
PROPERTY                         VALUE
proxy.get.n                      60000
proxy.kalive.ns                  72.651942ms
proxy.lst.n                      1
proxy.lst.ns                     10.17283ms
proxy.put.n                      86531
proxy.up.ns.time                 5m30s
target.append.ns                 0
target.disk.sda.avg.rsize        298636
target.disk.sda.avg.wsize        1077943
target.disk.sda.util             33
...
target.stream.in.n               24168
target.stream.in.size            149.39MiB
target.stream.out.n              24168
target.stream.out.size           110.23MiB
```

Monitor intra-cluster networking at a 3s seconds interval:

```console
$ ais show cluster stats streams --refresh 3s

target.stream.in.n          41288
target.stream.in.size       4.01GiB
target.stream.out.n         41288
target.stream.out.size      194.98MiB
```

Use `--raw` option:

```console
$ ais cluster show stats target.dl --raw

PROPERTY         VALUE
target.dl.ns     228747302
target.dl.size   848700

$ ais cluster show stats target.dl
PROPERTY         VALUE
target.dl.ns     228.747302ms
target.dl.size   828.81KiB
```

Excerpt from global stats to show both proxy and target grouping:

```console
$ ais cluster show stats --raw

PROPERTY                         VALUE
proxy.get.n                      90
proxy.get.ns                     0
proxy.lst.n                      5
proxy.lst.ns                     17250254
...
target.dl.ns                     228747302
target.dl.size                   848700
target.get.bps                   422657829
target.get.n                     90
target.get.ns                    33319026
```

Detailed node statistics:

```console
$ ais cluster show stats t[sCFgt15AB]

PROPERTY                 VALUE
append.ns                0
dl.ns                    229.142325ms
dl.size                  132.61KiB
dsort.creation.req.ns    0
dsort.creation.resp.ns   0
err.dl.n                 4
err.get.n                14
get.bps                  74MiB/s
get.n                    19
get.ns                   36.782125ms
get.redir.ns             459.786µs
kalive.ns                619.869272ms
kalive.ns.max            0
kalive.ns.min            0
lst.n                    7
lst.ns                   3.695827ms
put.n                    40
put.ns                   485.817135ms
put.redir.ns             75.654033ms
stream.in.n              139
stream.out.n             175
stream.out.size          283.90MiB
up.ns.time               169m
mountpath.0.path         /tmp/ais/mp1/8
mountpath.0.used         21.64GiB
mountpath.0.avail        28.33GiB
mountpath.0.%used        43
...
```

Filtering to show only those metrics that contain specific substring

```console
$ ais show cluster stats t[sCFgt15AB] put --raw

PROPERTY         VALUE
put.n            183864
put.ns           42711506911
put.redir.ns     105969

# And the same in human-readable format
$ ais show cluster stats t[sCFgt15AB] put
PROPERTY         VALUE
put.n            183864
put.ns           42.711506911s
put.redir.ns     0.106ms
```

Monitor node's statistics with a 2 seconds refresh interval

```console
$ ais cluster show stats t[sCFgt15AB] put --refresh 2s

PROPERTY         VALUE
put.n            24
put.ns           306.935732ms
put.redir.ns     106.648013ms

PROPERTY         VALUE
put.n            24
put.ns           306.935732ms
put.redir.ns     106.648013ms

PROPERTY         VALUE
put.n            26
put.ns           327.711826ms
put.redir.ns     108.714872ms

PROPERTY         VALUE
put.n            32
put.ns           416.776232ms
put.redir.ns     115.384523ms
```

## Show disk stats

`ais show storage disk [TARGET_ID]`

Show the disk stats of the `TARGET_ID`. If `TARGET_ID` isn't given, disk stats for all targets will be shown.

### Options

| Flag | Type | Description | Default |
| --- | --- | --- | --- |
| `--json, -j` | `bool` | Output in JSON format | `false` |
| `--count` | `int` | Can be used in combination with `--refresh` option to limit the number of generated reports | `1` |
| `--refresh` | `duration` | Refresh interval - time duration between reports. The usual unit suffixes are supported and include `m` (for minutes), `s` (seconds), `ms` (milliseconds) | ` ` |
| `--no-headers` | `bool` | Display tables without headers | `false` |

### Examples

#### Display disk reports stats N times every M seconds

Display 5 reports of all targets' disk statistics, with 10s intervals between each report.

```console
$ ais show storage disk --count 2 --refresh 10s
Target		Disk	Read		Write		%Util
163171t8088	sda	6.00KiB/s	171.00KiB/s	49
948212t8089	sda	6.00KiB/s	171.00KiB/s	49
41981t8085	sda	6.00KiB/s	171.00KiB/s	49
490062t8086	sda	6.00KiB/s	171.00KiB/s	49
164472t8087	sda	6.00KiB/s	171.00KiB/s	49

Target		Disk	Read		Write		%Util
163171t8088	sda	1.00KiB/s	4.26MiB/s	96
41981t8085	sda	1.00KiB/s	4.26MiB/s	96
948212t8089	sda	1.00KiB/s	4.26MiB/s	96
490062t8086	sda	1.00KiB/s	4.29MiB/s	96
164472t8087	sda	1.00KiB/s	4.26MiB/s	96
```

## Join a node

`ais cluster add-remove-nodes join --role=proxy IP:PORT`

Join a proxy to the cluster.

`ais cluster add-remove-nodes join --role=target IP:PORT`

Join a target to the cluster.

Note: The node will try to join the cluster using an ID it detects (either in the filesystem's xattrs or on disk) or that it generates for itself.
If you would like to specify an ID, you can do so while starting the [`aisnode` executable](/docs/command_line.md).

### Examples

#### Join node

Join a proxy node with socket address `192.168.0.185:8086`

```console
$ ais cluster add-remove-nodes join --role=proxy 192.168.0.185:8086
Proxy with ID "23kfa10f" successfully joined the cluster.
```

## Remove a node

**Temporarily remove an existing node from the cluster:**

`ais cluster add-remove-nodes start-maintenance DAEMON_ID`
`ais cluster add-remove-nodes stop-maintenance DAEMON_ID`

Starting maintenance puts the node in maintenance mode, and the cluster gradually transitions to
operating without the specified node (which is labeled `maintenance` in the cluster map). Stopping
maintenance will revert this.

`ais cluster add-remove-nodes shutdown DAEMON_ID`

Shutting down a node will put the node in maintenance mode first, and then shut down the `aisnode`
process on the node.


**Permanently remove an existing node from the cluster:**

`ais cluster add-remove-nodes decommission DAEMON_ID`

Decommissioning a node will safely remove a node from the cluster by triggering a cluster-wide
rebalance first. This can be avoided by specifying `--no-rebalance`.


### Options

| Flag | Type | Description | Default |
| --- | --- | --- | --- |
| `--no-rebalance` | `bool` | By default, `ais cluster add-remove-nodes maintenance` and `ais cluster add-remove-nodes decommission` triggers a global cluster-wide rebalance. The `--no-rebalance` flag disables automatic rebalance thus providing for the administrative option to rebalance the cluster manually at a later time. BEWARE: advanced usage only! | `false` |

### Examples

#### Decommission node

**Permananently remove proxy p[omWp8083] from the cluster:**

```console
$ ais cluster add-remove-nodes decommission <TAB-TAB>
p[cFOp8082]   p[Hqhp8085]   p[omWp8083]   t[bFat8087]   t[Icjt8089]   t[ofPt8091]
p[dpKp8084]   p[NGVp8081]   p[Uerp8080]   t[erbt8086]   t[IDDt8090]   t[TKSt8088]

$ ais cluster add-remove-nodes decommission p[omWp8083]

Node "omWp8083" has been successfully removed from the cluster.
```

**To terminate `aisnode` on a given machine, use the `shutdown` command, e.g.:**

```console
$ ais cluster add-remove-nodes shutdown t[23kfa10f]
```

Similar to the `maintenance` option, `shutdown` triggers global rebalancing then shuts down the corresponding `aisnode` process (target `t[23kfa10f]` in the example above).

#### Temporarily put node in maintenance

```console
$ ais show cluster
PROXY            MEM USED %      MEM AVAIL       UPTIME
202446p8082      0.09%           31.28GiB        70s
279128p8080[P]   0.11%           31.28GiB        80s

TARGET           MEM USED %      MEM AVAIL       CAP USED %      CAP AVAIL       CPU USED %      REBALANCE       UPTIME
147665t8084      0.10%           31.28GiB        16%             2.458TiB        0.12%           -               70s
165274t8087      0.10%           31.28GiB        16%             2.458TiB        0.12%           -               70s

$ ais cluster add-remove-nodes start-maintenance 147665t8084
$ ais show cluster
PROXY            MEM USED %      MEM AVAIL       UPTIME
202446p8082      0.09%           31.28GiB        70s
279128p8080[P]   0.11%           31.28GiB        80s

TARGET           MEM USED %      MEM AVAIL       CAP USED %      CAP AVAIL       CPU USED %      REBALANCE       UPTIME  STATUS
147665t8084      0.10%           31.28GiB        16%             2.458TiB        0.12%           -               71s     maintenance
165274t8087      0.10%           31.28GiB        16%             2.458TiB        0.12%           -               71s     online
```

#### Take a node out of maintenance

```console
$ ais cluster add-remove-nodes stop-maintenance t[147665t8084]
$ ais show cluster
PROXY            MEM USED %      MEM AVAIL       UPTIME
202446p8082      0.09%           31.28GiB        80s
279128p8080[P]   0.11%           31.28GiB        90s

TARGET           MEM USED %      MEM AVAIL       CAP USED %      CAP AVAIL       CPU USED %      REBALANCE       UPTIME
147665t8084      0.10%           31.28GiB        16%             2.458TiB        0.12%           -               80s
165274t8087      0.10%           31.28GiB        16%             2.458TiB        0.12%           -               80s
```

## Remote AIS cluster

Given an arbitrary pair of AIS clusters A and B, cluster B can be *attached* to cluster A, thus providing (to A) a fully-accessible (list-able, readable, writeable) *backend*.

For background, terminology, and definitions, and for many more usage examples, please see:

* [Remote AIS cluster](/docs/providers.md#remote-ais-cluster)
* [Usage examples and easy-to-use scripts for developers](/docs/development.md)

### Attach remote cluster

`ais cluster remote-attach UUID=URL [UUID=URL...]`

or

`ais cluster remote-attach ALIAS=URL [ALIAS=URL...]`

Attach a remote AIS cluster to a local one via the remote cluster public URL. Alias (a user-defined name) can be used instead of cluster UUID for convenience.
For more details and background on *remote clustering*, please refer to this [document](/docs/providers.md).

#### Examples

Attach two remote clusters, the first - by its UUID, the second one - via user-friendly alias (`two`).

```console
$ ais cluster remote-attach a345e890=http://one.remote:51080 two=http://two.remote:51080`
```

### Detach remote cluster

`ais cluster remote-detach UUID|ALIAS`

Detach a remote cluster using its alias or UUID.

#### Examples

Example below assumes that the remote has user-given alias `two`:

```console
$ ais cluster remote-detach two
```

### Show remote clusters

`ais show remote-cluster`

Show details about attached remote clusters.

#### Examples
The following two commands attach and then show the remote cluster at the address `my.remote.ais:51080`:

```console
$ ais cluster remote-attach alias111=http://my.remote.ais:51080
Remote cluster (alias111=http://my.remote.ais:51080) successfully attached
$ ais show remote-cluster
UUID      URL                     Alias     Primary         Smap  Targets  Online
eKyvPyHr  my.remote.ais:51080     alias111  p[80381p11080]  v27   10       yes
```

Notice that:

* user can assign an arbitrary name (aka alias) to a given remote cluster
* the remote cluster does *not* have to be online at attachment time; offline or currently unreachable clusters are shown as follows:

```console
$ ais show remote-cluster
UUID        URL                       Alias     Primary         Smap  Targets  Online
eKyvPyHr    my.remote.ais:51080       alias111  p[primary1]     v27   10       no
<alias222>  <other.remote.ais:51080>            n/a             n/a   n/a      no
```

Notice the difference between the first and the second lines in the printout above: while both clusters appear to be currently offline (see the rightmost column), the first one was accessible at some earlier time and therefore we show that it has (in this example) 10 storage nodes and other details.

To `detach` any of the previously configured associations, simply run:

```console
$ ais cluster remote-detach alias111
$ ais show remote-cluster
UUID        URL                       Alias     Primary         Smap  Targets  Online
<alias222>  <other.remote.ais:51080>            n/a             n/a   n/a      no
```

// Package sys provides methods to read system information
/*
 * Copyright (c) 2018-2022, NVIDIA CORPORATION. All rights reserved.
 */
package sys

import (
	"os"

	"github.com/NVIDIA/aistore/cmn/cos"
	"github.com/NVIDIA/aistore/cmn/debug"
)

// TODO: add more mem and CPU stats and details

func GetMemCPU() cos.MemCPUInfo {
	var (
		mem MemStat
		err error
	)
	err = mem.Get()
	debug.AssertNoErr(err)

	proc, err := ProcessStats(os.Getpid())
	debug.AssertNoErr(err)

	return cos.MemCPUInfo{
		MemAvail:   mem.ActualFree,
		MemUsed:    proc.Mem.Resident,
		PctMemUsed: float64(proc.Mem.Resident) * 100 / float64(mem.Total),
		PctCPUUsed: proc.CPU.Percent,
	}
}

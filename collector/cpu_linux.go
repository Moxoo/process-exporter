// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !nocpu
// +build !nocpu

package collector

import (
	"fmt"
	"log"
	"log/syslog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/procfs"
)

type cpuCollector struct {
	fs            procfs.FS
	syslog        *syslog.Writer
	cpuStats      map[int64]procfs.CPUStat
	cpuTotal      procfs.CPUStat
	cpuStatsMutex sync.Mutex
}

// Idle jump back limit in seconds.
const jumpBackSeconds = 3.0

var (
	cpuCounter = 0
)

// NewCPUCollector returns a new Collector exposing kernel/system statistics.
func NewCPUCollector() error {
	fs, err := procfs.NewFS(procPath)
	if err != nil {
		return fmt.Errorf("failed to open procfs: %w", err)
	}

	sysLogger, sysLogConErr := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "cpu")
	if sysLogConErr != nil {
		return fmt.Errorf("failed to connect syslog: %w", sysLogConErr)
	}
	defer sysLogger.Close()

	c := &cpuCollector{
		fs:       fs,
		syslog:   sysLogger,
		cpuStats: make(map[int64]procfs.CPUStat),
	}

	go c.start()
	return nil
}

func (c *cpuCollector) start() {
	c.scrape()
	for range time.Tick(cpuCollectInterval) {
		c.scrape()
	}
}

func (c *cpuCollector) scrape() {
	err := c.Update()

	if err != nil {
		log.Println(err)
	}
}

// Update implements Collector and exposes cpu related metrics from /proc/stat and /sys/.../cpu/.
func (c *cpuCollector) Update() error {
	if err := c.updateStat(); err != nil {
		return err
	}
	return nil
}

// updateStat reads /proc/stat through procfs and exports CPU-related metrics.
func (c *cpuCollector) updateStat() error {
	cpuCounter++
	stats, err := c.fs.Stat()
	if err != nil {
		return err
	}

	if len(c.cpuStats) > 0 && cpuCounter == cpuSyslogWriteScale {
		userCpu := ((stats.CPUTotal.User - c.cpuTotal.User) / (cpuCollectInterval.Seconds()) / float64(len(stats.CPU))) * 100
		sysCpu := ((stats.CPUTotal.System - c.cpuTotal.System) / (cpuCollectInterval.Seconds()) / float64(len(stats.CPU))) * 100
		idleCpu := ((stats.CPUTotal.Idle - c.cpuTotal.Idle) / (cpuCollectInterval.Seconds()) / float64(len(stats.CPU))) * 100
		totalCpu := userCpu + sysCpu

		// write syslog: cpuname|用户态的CPU使用率|内核态的CPU使用率|CPU空闲率|该CPU总使用率
		sysFormat := "%s|%.1f|%.1f|%.1f|%.1f"
		buffer := fmt.Sprintf(sysFormat, "cpu", userCpu, sysCpu, idleCpu, totalCpu)
		sysLogWriteErr := c.syslog.Info(buffer)
		if sysLogWriteErr != nil {
			log.Println(sysLogWriteErr)
		}

		for cpuID, cpuStat := range c.cpuStats {
			cpuNum := strconv.Itoa(int(cpuID))
			userCpu = ((stats.CPU[cpuID].User - cpuStat.User) / (cpuCollectInterval.Seconds())) * 100
			sysCpu = ((stats.CPU[cpuID].System - cpuStat.System) / (cpuCollectInterval.Seconds())) * 100
			idleCpu = ((stats.CPU[cpuID].Idle - cpuStat.Idle) / (cpuCollectInterval.Seconds())) * 100
			totalCpu = userCpu + sysCpu
			buffer = fmt.Sprintf(sysFormat, "cpu"+cpuNum, userCpu, sysCpu, idleCpu, totalCpu)
			sysLogWriteErr = c.syslog.Info(buffer)
			if sysLogWriteErr != nil {
				log.Println(sysLogWriteErr)
			}
		}
		cpuCounter = 0
	}

	c.updateCPUStats(stats.CPU, stats.CPUTotal)
	return nil
}

// updateCPUStats updates the internal cache of CPU stats.
func (c *cpuCollector) updateCPUStats(newStats map[int64]procfs.CPUStat, newStat procfs.CPUStat) {
	// Acquire a lock to update the stats.
	c.cpuStatsMutex.Lock()
	defer c.cpuStatsMutex.Unlock()

	// Reset the cache if the list of CPUs has changed.
	for i, n := range newStats {
		cpuStats := c.cpuStats[i]

		// If idle jumps backwards by more than X seconds, assume we had a hotplug event and reset the stats for this CPU.
		//CPU Idle counter jumped backwards more than jumpBackSeconds seconds, possible hotplug event, resetting CPU stats
		if (cpuStats.Idle - n.Idle) >= jumpBackSeconds {
			cpuStats = procfs.CPUStat{}
		}

		if n.Idle >= cpuStats.Idle {
			cpuStats.Idle = n.Idle
		}

		if n.User >= cpuStats.User {
			cpuStats.User = n.User
		}

		if n.System >= cpuStats.System {
			cpuStats.System = n.System
		}

		c.cpuStats[i] = cpuStats
	}
	c.cpuTotal = newStat
	//TODO: Remove offline CPUs.
}

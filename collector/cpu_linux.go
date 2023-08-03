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
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/procfs"
	//"golang.org/x/exp/maps"
	//"golang.org/x/exp/slices"
)

type cpuCollector struct {
	fs            procfs.FS
	cpuStats      map[int64]procfs.CPUStat
	cpuTotal      procfs.CPUStat
	cpuStatsMutex sync.Mutex
}

// Idle jump back limit in seconds.
const jumpBackSeconds = 3.0

var (
	jumpBackDebugMessage = fmt.Sprintf("CPU Idle counter jumped backwards more than %f seconds, possible hotplug event, resetting CPU stats", jumpBackSeconds)
)

// NewCPUCollector returns a new Collector exposing kernel/system statistics.
func NewCPUCollector() error {
	fs, err := procfs.NewFS(procPath)
	if err != nil {
		return fmt.Errorf("failed to open procfs: %w", err)
	}

	c := &cpuCollector{
		fs:       fs,
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
	stats, err := c.fs.Stat()
	if err != nil {
		return err
	}

	if len(c.cpuStats) > 0 {
		// Acquire a lock to read the stats.
		//c.cpuStatsMutex.Lock()
		//defer c.cpuStatsMutex.Unlock()
		userCpu := 100 * (stats.CPUTotal.User - c.cpuTotal.User) / (cpuCollectInterval.Seconds()) / float64(len(stats.CPU))
		sysCpu := 100 * (stats.CPUTotal.System - c.cpuTotal.System) / (cpuCollectInterval.Seconds()) / float64(len(stats.CPU))
		log.Printf("us cpu usage: %.1f, sy cpu usage: %.1f", userCpu, sysCpu)

		for cpuID, cpuStat := range c.cpuStats {
			cpuNum := strconv.Itoa(int(cpuID))
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.User, cpuNum, "user")
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.Nice, cpuNum, "nice")
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.System, cpuNum, "system")
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.Idle, cpuNum, "idle")
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.Iowait, cpuNum, "iowait")
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.IRQ, cpuNum, "irq")
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.SoftIRQ, cpuNum, "softirq")
			//ch <- prometheus.MustNewConstMetric(c.cpu, prometheus.CounterValue, cpuStat.Steal, cpuNum, "steal")
			log.Printf("cpu%s:, %f", cpuNum, cpuStat.User)
		}
	}

	c.updateCPUStats(stats.CPU, stats.CPUTotal)
	return nil
}

// updateCPUStats updates the internal cache of CPU stats.
func (c *cpuCollector) updateCPUStats(newStats map[int64]procfs.CPUStat, newStat procfs.CPUStat) {
	// Acquire a lock to update the stats.
	//c.cpuStatsMutex.Lock()
	//defer c.cpuStatsMutex.Unlock()

	// Reset the cache if the list of CPUs has changed.
	for i, n := range newStats {
		cpuStats := c.cpuStats[i]

		// If idle jumps backwards by more than X seconds, assume we had a hotplug event and reset the stats for this CPU.
		if (cpuStats.Idle - n.Idle) >= jumpBackSeconds {
			//level.Debug(c.logger).Log("msg", jumpBackDebugMessage, "cpu", i, "old_value", cpuStats.Idle, "new_value", n.Idle)
			cpuStats = procfs.CPUStat{}
		}

		if n.Idle >= cpuStats.Idle {
			cpuStats.Idle = n.Idle
		} else {
			//level.Debug(c.logger).Log("msg", "CPU Idle counter jumped backwards", "cpu", i, "old_value", cpuStats.Idle, "new_value", n.Idle)
		}

		if n.User >= cpuStats.User {
			cpuStats.User = n.User
		} else {
			//level.Debug(c.logger).Log("msg", "CPU User counter jumped backwards", "cpu", i, "old_value", cpuStats.User, "new_value", n.User)
		}

		if n.Nice >= cpuStats.Nice {
			cpuStats.Nice = n.Nice
		} else {
			//level.Debug(c.logger).Log("msg", "CPU Nice counter jumped backwards", "cpu", i, "old_value", cpuStats.Nice, "new_value", n.Nice)
		}

		if n.System >= cpuStats.System {
			cpuStats.System = n.System
		} else {
			//level.Debug(c.logger).Log("msg", "CPU System counter jumped backwards", "cpu", i, "old_value", cpuStats.System, "new_value", n.System)
		}

		if n.Iowait >= cpuStats.Iowait {
			cpuStats.Iowait = n.Iowait
		} else {
			//level.Debug(c.logger).Log("msg", "CPU Iowait counter jumped backwards", "cpu", i, "old_value", cpuStats.Iowait, "new_value", n.Iowait)
		}

		if n.IRQ >= cpuStats.IRQ {
			cpuStats.IRQ = n.IRQ
		} else {
			//level.Debug(c.logger).Log("msg", "CPU IRQ counter jumped backwards", "cpu", i, "old_value", cpuStats.IRQ, "new_value", n.IRQ)
		}

		if n.SoftIRQ >= cpuStats.SoftIRQ {
			cpuStats.SoftIRQ = n.SoftIRQ
		} else {
			//level.Debug(c.logger).Log("msg", "CPU SoftIRQ counter jumped backwards", "cpu", i, "old_value", cpuStats.SoftIRQ, "new_value", n.SoftIRQ)
		}

		if n.Steal >= cpuStats.Steal {
			cpuStats.Steal = n.Steal
		} else {
			//level.Debug(c.logger).Log("msg", "CPU Steal counter jumped backwards", "cpu", i, "old_value", cpuStats.Steal, "new_value", n.Steal)
		}

		if n.Guest >= cpuStats.Guest {
			cpuStats.Guest = n.Guest
		} else {
			//level.Debug(c.logger).Log("msg", "CPU Guest counter jumped backwards", "cpu", i, "old_value", cpuStats.Guest, "new_value", n.Guest)
		}

		if n.GuestNice >= cpuStats.GuestNice {
			cpuStats.GuestNice = n.GuestNice
		} else {
			//level.Debug(c.logger).Log("msg", "CPU GuestNice counter jumped backwards", "cpu", i, "old_value", cpuStats.GuestNice, "new_value", n.GuestNice)
		}

		c.cpuStats[i] = cpuStats
	}
	c.cpuTotal = newStat
	//TODO: Remove offline CPUs.
}

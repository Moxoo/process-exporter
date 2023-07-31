// Copyright 2019 The Prometheus Authors
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
	"container/list"
	"fmt"
	"github.com/prometheus/procfs/sysfs"
	"log"
	"strconv"
	"time"
)

type cpuFreqCollector struct {
	fs     sysfs.FS
}

var (
	sysPath = "/sys"
)

// NewCPUFreqCollector returns a new Collector exposing kernel/system statistics.
func NewCPUFreqCollector() (*cpuFreqCollector, error) {
	fs, err := sysfs.NewFS(sysPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sysfs: %w", err)
	}

	c := &cpuFreqCollector{
		fs:     fs,
	}

	go c.start()

	return c, nil
}

func (c *cpuFreqCollector) start() {
	c.scrape()
	for range time.Tick(interval) {
		c.scrape()
	}
}

func (c *cpuFreqCollector) scrape() {
	begin := time.Now()
	err := c.Update()
	duration := time.Since(begin)

	if err != nil {
		log.Println(err)
	} else {
		log.Printf("cpufreq collector succeeded, duration_seconds: %f", duration.Seconds())
	}
}

// Update implements Collector and exposes cpu related metrics from /proc/stat and /sys/.../cpu/.
func (c *cpuFreqCollector) Update() error {
	cpuFreqs, err := c.fs.SystemCpufreq()
	if err != nil {
		return err
	}

	cpuFreqsList := list.New()
	// sysfs cpufreq values are kHz, thus multiply by 1000 to export base units (hz).
	// See https://www.kernel.org/doc/Documentation/cpu-freq/user-guide.txt
	for _, stats := range cpuFreqs {
		if stats.CpuinfoCurrentFrequency != nil {
			//log.Printf("cur: %f", float64(*stats.CpuinfoCurrentFrequency)*1000.0)
		}
		if stats.CpuinfoMinimumFrequency != nil {
			//log.Printf("min: %f", float64(*stats.CpuinfoMinimumFrequency)*1000.0)
		}
		if stats.CpuinfoMaximumFrequency != nil {
			//log.Printf("max: %f", float64(*stats.CpuinfoMaximumFrequency)*1000.0)
		}
		if stats.ScalingCurrentFrequency != nil {
			cpuFreqsList.PushBack(stats.Name + ":" + strconv.Itoa(int(*stats.ScalingCurrentFrequency)*1000.0))
		}
		if stats.ScalingMinimumFrequency != nil {
			//log.Printf("scale min: %f", float64(*stats.ScalingMinimumFrequency)*1000.0)
		}
		if stats.ScalingMaximumFrequency != nil {
			//log.Printf("scale max: %f", float64(*stats.ScalingMaximumFrequency)*1000.0)
		}
	}

	// Traverse the list and print its elements
	cpuFreqsFormat := ""
	for e := cpuFreqsList.Front(); e != nil; e = e.Next() {
		cpuFreqsFormat += e.Value.(string)
		if e.Next() != nil {
			cpuFreqsFormat += "|"
		}
	}
	log.Printf("%s", cpuFreqsFormat)
	return nil
}

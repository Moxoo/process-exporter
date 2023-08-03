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
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/procfs/sysfs"
	"log"
	"log/syslog"
	"reflect"
	"strconv"
	"time"
)

type cpuFreqCollector struct {
	fs     sysfs.FS
	syslog *syslog.Writer
}

var lastCpuFreqs []sysfs.SystemCPUCpufreqStats
var cpuFreqsCounter = 0
var lastCpuFreqsList = list.New()

// NewCPUFreqCollector returns a new Collector exposing kernel/system statistics.
func NewCPUFreqCollector() error {
	fs, err := sysfs.NewFS(sysPath)
	if err != nil {
		return fmt.Errorf("failed to open sysfs: %w", err)
	}

	sysLogger, sysLogConErr := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "cpufreq")
	if sysLogConErr != nil {
		return fmt.Errorf("failed to connect syslog: %w", sysLogConErr)
	}
	defer sysLogger.Close()

	c := &cpuFreqCollector{
		fs:     fs,
		syslog: sysLogger,
	}

	go c.start()

	return nil
}

func (c *cpuFreqCollector) start() {
	c.scrape()
	for range time.Tick(cpufreqCollectInterval) {
		c.scrape()
	}
}

func (c *cpuFreqCollector) scrape() {
	err := c.Update()

	if err != nil {
		log.Println(err)
	}
}

// Update implements Collector and exposes cpu related metrics from /proc/stat and /sys/.../cpu/.
func (c *cpuFreqCollector) Update() error {
	cpuFreqsCounter++
	cpuFreqs, err := c.fs.SystemCpufreq()
	if err != nil {
		return err
	}

	cpuFreqsList := list.New()
	for _, stats := range cpuFreqs {
		// sysfs cpufreq values are kHz, thus multiply by 1000 to export base units (hz).
		// See https://www.kernel.org/doc/Documentation/cpu-freq/user-guide.txt
		if stats.ScalingCurrentFrequency != nil {
			//log.Printf("cur: %f", float64(*stats.CpuinfoCurrentFrequency)*1000.0)
			cpuFreqsList.PushBack(stats.Name + ":" + strconv.FormatInt(int64(*stats.ScalingCurrentFrequency*1000.0), 10))
		}
	}
	if cpuFreqsCounter == cpufreqSyslogWriteScale || (lastCpuFreqsList.Len() > 0 && !reflect.DeepEqual(lastCpuFreqsList, cpuFreqsList)) {
		// write syslog
		cpuFreqsFormat := ""
		for e := cpuFreqsList.Front(); e != nil; e = e.Next() {
			cpuFreqsFormat += e.Value.(string)
			if e.Next() != nil {
				cpuFreqsFormat += "|"
			}
		}
		sysLogWriteErr := c.syslog.Info(cpuFreqsFormat)
		if sysLogWriteErr != nil {
			return sysLogWriteErr
		}
		if cpuFreqsCounter == cpufreqSyslogWriteScale {
			cpuFreqsCounter = 0
		}
	}
	// record last cpu freqs list
	lastCpuFreqsList = cpuFreqsList
	return nil
}

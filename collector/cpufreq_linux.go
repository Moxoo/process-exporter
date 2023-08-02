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
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/procfs/sysfs"
	"log"
	"log/syslog"
	"strconv"
	"time"
)

type cpuFreqCollector struct {
	fs     sysfs.FS
	syslog *syslog.Writer
	sqlite *sql.DB
}

var (
	sysPath = "/sys"
)

// NewCPUFreqCollector returns a new Collector exposing kernel/system statistics.
func NewCPUFreqCollector() error {
	fs, err := sysfs.NewFS(sysPath)
	if err != nil {
		return fmt.Errorf("failed to open sysfs: %w", err)
	}

	sysLogger, sysLogConErr := syslog.New(syslog.LOG_INFO | syslog.LOG_USER, "cpufreq")
	if sysLogConErr != nil {
		return fmt.Errorf("failed to connect syslog: %w", sysLogConErr)
	}
	defer sysLogger.Close()

	sqliteDb, sqliteDbErr := sql.Open("sqlite3", "/userdata/log/sqlite/exporter.db")
	if sqliteDbErr != nil {
		return fmt.Errorf("failed to open sqlite: %w", sqliteDbErr)
	}

	_, sqliteCreateErr := sqliteDb.Exec(`
		CREATE TABLE IF NOT EXISTS cpufreq (
		    ts INTEGER,
		    hostname TEXT,
		    ip TEXT,
		    pid INTEGER,
		    cpu0 INTEGER,
		    cpu1 INTEGER,
		    cpu2 INTEGER,
		    cpu3 INTEGER,
		    cpu4 INTEGER,
		    cpu5 INTEGER,
		    cpu6 INTEGER,
		    cpu7 INTEGER,
			PRIMARY KEY (ts, hostname, ip, pid)
		)
	`)
	if sqliteCreateErr != nil {
		return fmt.Errorf("error creating table: %w", sqliteCreateErr)
	}

	c := &cpuFreqCollector{
		fs:     fs,
		syslog: sysLogger,
		sqlite: sqliteDb,
	}

	go c.start()

	return nil
}

func (c *cpuFreqCollector) start() {
	c.scrape()
	for range time.Tick(interval) {
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
	cpuFreqs, err := c.fs.SystemCpufreq()
	if err != nil {
		return err
	}

	cpuFreqsList := list.New()
	cpuFreqsMap := make(map[string]int64)
	// sysfs cpufreq values are kHz, thus multiply by 1000 to export base units (hz).
	// See https://www.kernel.org/doc/Documentation/cpu-freq/user-guide.txt
	for _, stats := range cpuFreqs {
		if stats.CpuinfoCurrentFrequency != nil {
			//log.Printf("cur: %f", float64(*stats.CpuinfoCurrentFrequency)*1000.0)
			cpuFreqsList.PushBack(stats.Name + ":" + strconv.FormatInt(int64(*stats.CpuinfoCurrentFrequency*1000.0), 10))
			cpuFreqsMap[stats.Name] = int64(*stats.CpuinfoCurrentFrequency*1000.0)
		}
	}

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

	// insert sqlite
	ts := time.Now().UnixMilli()
	_, sqliteInsertErr := c.sqlite.Exec("INSERT INTO cpufreq (ts, hostname, ip, pid, cpu0, cpu1, cpu2, cpu3, cpu4, cpu5, cpu6, cpu7) " +
		"VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
		ts, hostname, ip[0].String(), pid,
		cpuFreqsMap["0"], cpuFreqsMap["1"], cpuFreqsMap["2"], cpuFreqsMap["3"], cpuFreqsMap["4"],cpuFreqsMap["5"], cpuFreqsMap["6"], cpuFreqsMap["7"])
	if sqliteInsertErr != nil {
		return sqliteInsertErr
	}

	return nil
}

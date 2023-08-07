//go:build !process
// +build !process

package collector

import (
	"fmt"
	"github.com/ncabatoff/process-exporter/src/common"
	"github.com/ncabatoff/process-exporter/src/proc"
	"log"
	"log/syslog"
	"time"
)

var lastGroupByName proc.GroupByName
var counter = 0

type (
	ProcessCollectorOption struct {
		ProcFSPath string
		Children   bool
		Namer      common.MatchNamer
	}

	NamedProcessCollector struct {
		*proc.Grouper
		source proc.Source
		syslog *syslog.Writer
	}
)

func NewProcessCollector(options ProcessCollectorOption) error {
	fs, err := proc.NewFS(options.ProcFSPath)
	if err != nil {
		return err
	}

	sysLogger, sysLogConErr := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "procinfo")
	if sysLogConErr != nil {
		return fmt.Errorf("failed to connect syslog: %w", sysLogConErr)
	}
	defer sysLogger.Close()

	p := &NamedProcessCollector{
		Grouper: proc.NewGrouper(options.Namer, options.Children),
		source:  fs,
		syslog:  sysLogger,
	}

	// 开启定时采集进程信息的协程
	go p.start()
	return nil
}

func (p *NamedProcessCollector) start() {
	p.scrape()
	for range time.Tick(procinfoCollectInterval) {
		p.scrape()
	}
}

func (p *NamedProcessCollector) scrape() {
	counter++
	_, groups, err := p.Update(p.source.AllProcs())
	if err != nil {
		log.Printf("error reading procs: %v", err)
	} else {
		if len(lastGroupByName) > 0 && counter == procinfoSyslogWriteScale {
			for gname, gcounts := range groups {
				// update
				cpuUsageUser := 100 * (gcounts.CPUUserTime - lastGroupByName[gname].CPUUserTime) / procinfoCollectInterval.Seconds()
				cpuUsageSys := 100 * (gcounts.CPUSystemTime - lastGroupByName[gname].CPUSystemTime) / procinfoCollectInterval.Seconds()
				cpuUsage := cpuUsageUser + cpuUsageSys

				// write syslog: nodeName|用户态的CPU使用率|内核态的CPU使用率|总CPU使用率|物理内存|虚拟内存
				sysFormat := "%s|%.1f|%.1f|%.1f|%d|%d"
				buffer := fmt.Sprintf(sysFormat, gname, cpuUsageUser, cpuUsageSys, cpuUsage, gcounts.Memory.ResidentBytes, gcounts.Memory.VirtualBytes)
				//log.Printf("%s", buffer)
				sysLogWriteErr := p.syslog.Info(buffer)
				if sysLogWriteErr != nil {
					log.Println(sysLogWriteErr)
				}
			}
			counter = 0
		}
		// record last group info
		lastGroupByName = groups
	}
}

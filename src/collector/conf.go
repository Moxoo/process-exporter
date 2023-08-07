package collector

import (
	"time"
)

var (
	procinfoCollectInterval  = 3 * time.Second
	procinfoSyslogWriteScale = 5

	cpufreqCollectInterval  = 5 * time.Second
	cpufreqSyslogWriteScale = 6

	cpuCollectInterval  = 3 * time.Second
	cpuSyslogWriteScale = 5

	thermalCollectInterval = 5 * time.Second

	sysPath  = "/sys"
	procPath = "/proc"
)

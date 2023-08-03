package collector

import (
	"net"
	"os"
	"time"
)

var (
	hostname, _ = os.Hostname()
	ip, _       = net.LookupIP(hostname)
	pid         = os.Getpid()

	procinfoCollectInterval  = 3 * time.Second
	procinfoSyslogWriteScale = 5

	cpufreqCollectInterval  = 5 * time.Second
	cpufreqSyslogWriteScale = 6

	cpuCollectInterval = 3 * time.Second

	sysPath  = "/sys"
	procPath = "/proc"
)

//go:build !nothermalzone
// +build !nothermalzone

package collector

import (
	"fmt"
	"log"
	"log/syslog"
	"strconv"
	"time"

	"github.com/prometheus/procfs/sysfs"
)

type thermalZoneCollector struct {
	fs     sysfs.FS
	syslog *syslog.Writer
}

// NewThermalZoneCollector returns a new Collector exposing kernel/system statistics.
func NewThermalZoneCollector() error {
	fs, err := sysfs.NewFS(sysPath)
	if err != nil {
		return fmt.Errorf("failed to open sysfs: %w", err)
	}

	sysLogger, sysLogConErr := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "thermal")
	if sysLogConErr != nil {
		return fmt.Errorf("failed to connect syslog: %w", sysLogConErr)
	}
	defer sysLogger.Close()

	t := &thermalZoneCollector{
		fs:     fs,
		syslog: sysLogger,
	}

	go t.start()

	return nil
}

func (t *thermalZoneCollector) start() {
	t.scrape()
	for range time.Tick(thermalCollectInterval) {
		t.scrape()
	}
}

func (t *thermalZoneCollector) scrape() {
	err := t.Update()

	if err != nil {
		log.Println(err)
	}
}

func (t *thermalZoneCollector) Update() error {
	thermalZones, err := t.fs.ClassThermalZoneStats()
	if err != nil {
		return err
	}

	if len(thermalZones) > 0 {
		thermals := [10]float64{}
		for _, stats := range thermalZones {
			//log.Printf("%.1f, %s, %s", float64(stats.Temp)/1000.0, stats.Name, stats.Type)
			i, _ := strconv.Atoi(stats.Name)
			thermals[i] = float64(stats.Temp) / 1000.0
		}

		// write syslog: soc-thermal|bigcore0-thermal|bigcore1-thermal|littlecore-thermal|center-thermal|gpu-thermal|npu-thermal
		sysFormat := "%.1f|%.1f|%.1f|%.1f|%.1f|%.1f|%.1f"
		buffer := fmt.Sprintf(sysFormat, thermals[0], thermals[1], thermals[2], thermals[3], thermals[4], thermals[5], thermals[6])
		sysLogWriteErr := t.syslog.Info(buffer)
		if sysLogWriteErr != nil {
			log.Println(sysLogWriteErr)
		}
	}

	return nil
}

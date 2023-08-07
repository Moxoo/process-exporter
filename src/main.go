package main

import (
	"flag"
	"fmt"
	"github.com/ncabatoff/process-exporter/src/collector"
	"github.com/ncabatoff/process-exporter/src/common"
	"github.com/ncabatoff/process-exporter/src/config"
	"log"
)

// Version is set at build time use ldflags.
var version string

func printManual() {
	fmt.Print(`Usage:
  trident [options] -config.path filename.yml

or 

  trident [options] -procnames name1,...,nameN [-namemapping k1,v1,...,kN,vN]

The recommended option is to use a config file, but for convenience and
backwards compatibility the -procnames/-namemapping options exist as an
alternative.
 
The -children option (default:true) makes it so that any process that otherwise
isn't part of its own group becomes part of the first group found (if any) when
walking the process tree upwards.  In other words, resource usage of
subprocesses is added to their parent's usage unless the subprocess identifies
as a different group name.

Command-line process selection (procnames/namemapping):

  Every process not in the procnames list is ignored.  Otherwise, all processes
  found are reported on as a group based on the process name they share. 
  Here 'process name' refers to the value found in the second field of
  /proc/<pid>/stat, which is truncated at 15 chars.

  The -namemapping option allows assigning a group name based on a combination of
  the process name and command line.  For example, using 

    -namemapping "python2,([^/]+)\.py,java,-jar\s+([^/]+).jar" 

  will make it so that each different python2 and java -jar invocation will be
  tracked with distinct metrics.  Processes whose remapped name is absent from
  the procnames list will be ignored.  Here's an example that I run on my home
  machine (Ubuntu Xenian):

    trident -namemapping "upstart,(--user)" \
      -procnames chromium-browse,bash,prometheus,prombench,gvim,upstart:-user

  Since it appears that upstart --user is the parent process of my X11 session,
  this will make all apps I start count against it, unless they're one of the
  others named explicitly with -procnames.

Config file process selection (filename.yml):

  See README.md.
` + "\n")

}

func main() {
	var (
		procfsPath = flag.String("procfs", "/proc",
			"path to read proc data from")
		children = flag.Bool("children", true,
			"if a proc is tracked, track with it any children that aren't part of their own group")
		man = flag.Bool("man", false,
			"print manual")
		configPath = flag.String("config.path", "",
			"path to YAML config file")
	)
	flag.Parse()

	if *man {
		printManual()
		return
	}

	var matchnamer common.MatchNamer

	if *configPath != "" {
		cfg, err := config.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("error reading config file %q: %v", *configPath, err)
		}
		log.Printf("Reading metrics from %s based on %q", *procfsPath, *configPath)
		matchnamer = cfg.MatchNamers
	}

	done := make(chan bool)
	// create proc info collector
	go func() {
		err := collector.NewProcessCollector(
			collector.ProcessCollectorOption{
				ProcFSPath: *procfsPath,
				Children:   *children,
				Namer:      matchnamer,
			},
		)
		if err != nil {
			log.Fatalf("Error initializing proc info collector: %v", err)
		}
	}()

	// create cpu freq collector goroutine
	go func() {
		err := collector.NewCPUFreqCollector()
		if err != nil {
			log.Fatalf("Error initializing cpu freq collector: %v", err)
		}
	}()

	// create cpu collector goroutine
	go func() {
		err := collector.NewCPUCollector()
		if err != nil {
			log.Fatalf("Error initializing cpu collector: %v", err)
		}
	}()

	// create thermal collector goroutine
	go func() {
		err := collector.NewThermalZoneCollector()
		if err != nil {
			log.Fatalf("Error initializing thermal collector: %v", err)
		}
	}()

	<-done
}

package proc

import (
	"github.com/ncabatoff/process-exporter/src/common"
	"time"
)

type (
	// Grouper is the top-level interface to the process metrics.  All tracked
	// procs sharing the same group name are aggregated.
	Grouper struct {
		// groupAccum records the historical accumulation of a group so that
		// we can avoid ever decreasing the counts we return.
		groupAccum map[string]Counts
		tracker    *Tracker
	}

	// GroupByName maps group name to group metrics.
	GroupByName map[string]Group

	// Group describes the metrics of a single group.
	Group struct {
		Counts
		States
		Wchans map[string]int
		Procs  int
		Memory
		OldestStartTime time.Time
		OpenFDs         uint64
		WorstFDratio    float64
	}
)

// NewGrouper creates a grouper.
func NewGrouper(namer common.MatchNamer, trackChildren bool) *Grouper {
	g := Grouper{
		groupAccum: make(map[string]Counts),
		tracker:    NewTracker(namer, trackChildren),
	}
	return &g
}

func groupadd(grp Group, ts Update) Group {
	var zeroTime time.Time

	grp.Procs++
	grp.Memory.ResidentBytes += ts.Memory.ResidentBytes
	grp.Memory.VirtualBytes += ts.Memory.VirtualBytes
	grp.Memory.VmSwapBytes += ts.Memory.VmSwapBytes
	grp.Memory.ProportionalBytes += ts.Memory.ProportionalBytes
	grp.Memory.ProportionalSwapBytes += ts.Memory.ProportionalSwapBytes
	if ts.Filedesc.Open != -1 {
		grp.OpenFDs += uint64(ts.Filedesc.Open)
	}
	openratio := float64(ts.Filedesc.Open) / float64(ts.Filedesc.Limit)
	if grp.WorstFDratio < openratio {
		grp.WorstFDratio = openratio
	}
	grp.Counts.Add(ts.Latest)
	grp.States.Add(ts.States)
	if grp.OldestStartTime == zeroTime || ts.Start.Before(grp.OldestStartTime) {
		grp.OldestStartTime = ts.Start
	}

	if grp.Wchans == nil {
		grp.Wchans = make(map[string]int)
	}
	for wchan, count := range ts.Wchans {
		grp.Wchans[wchan] += count
	}

	return grp
}

// Update asks the tracker to report on each tracked process by name.
// These are aggregated by groupname, augmented by accumulated counts
// from the past, and returned.  Note that while the Tracker reports
// only what counts have changed since last cycle, Grouper.Update
// returns counts that never decrease.  Even once the last process
// with name X disappears, name X will still appear in the results
// with the same counts as before; of course, all non-count metrics
// will be zero.
func (g *Grouper) Update(iter Iter) (CollectErrors, GroupByName, error) {
	cerrs, tracked, err := g.tracker.Update(iter)
	if err != nil {
		return cerrs, nil, err
	}
	return cerrs, g.groups(tracked), nil
}

// Translate the updates into a new GroupByName and update internal history.
func (g *Grouper) groups(tracked []Update) GroupByName {
	groups := make(GroupByName)

	for _, update := range tracked {
		groups[update.GroupName] = groupadd(groups[update.GroupName], update)
	}

	// Add any accumulated counts to what was just observed,
	// and update the accumulators.
	for gname, group := range groups {
		if oldcounts, ok := g.groupAccum[gname]; ok {
			group.Counts.Add(Delta(oldcounts))
		}
		g.groupAccum[gname] = group.Counts
		groups[gname] = group
	}

	// Now add any groups that were observed in the past but aren't running now.
	for gname, gcounts := range g.groupAccum {
		if _, ok := groups[gname]; !ok {
			groups[gname] = Group{Counts: gcounts}
		}
	}

	return groups
}

// Package state remembers, between runs of `ddm deploy`, what each view was
// last built from and how long building took, so unchanged sites are skipped
// and a server that is always building can say so.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Build is the last build of one published view.
type Build struct {
	Commit string    `json:"commit"` // a sha, "none", or a pool id (build.Result.Commit)
	Stamp  string    `json:"stamp"`  // what else went into it: the spec, this repo's tools, ddm itself
	At     time.Time `json:"at"`     // when it finished
	Error  string    `json:"error,omitempty"`
}

// Run is one stretch of building, for the busy measure.
type Run struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// State is everything kept, in <work>/state.json.
type State struct {
	Builds map[string]Build `json:"builds"` // by Key
	Runs   []Run            `json:"runs"`   // the last week's
}

// Key names a published view: "<view>/<served>".
func Key(view, served string) string { return view + "/" + served }

func path(work string) string { return filepath.Join(work, "state.json") }

// Load reads the state, or returns an empty one if there is none yet.
func Load(work string) (*State, error) {
	st := &State{Builds: map[string]Build{}}
	data, err := os.ReadFile(path(work))
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	} else if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, err
	}
	if st.Builds == nil {
		st.Builds = map[string]Build{}
	}
	return st, nil
}

// Save writes the state atomically, dropping runs older than a week.
func (st *State) Save(work string, now time.Time) error {
	kept := st.Runs[:0]
	for _, r := range st.Runs {
		if now.Sub(r.End) < 7*24*time.Hour {
			kept = append(kept, r)
		}
	}
	st.Runs = kept
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	tmp := path(work) + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path(work))
}

// Busy is the share of the window before now spent building.
func (st *State) Busy(now time.Time, window time.Duration) float64 {
	from := now.Add(-window)
	var spent time.Duration
	for _, r := range st.Runs {
		start, end := r.Start, r.End
		if start.Before(from) {
			start = from
		}
		if end.After(now) {
			end = now
		}
		if end.After(start) {
			spent += end.Sub(start)
		}
	}
	return float64(spent) / float64(window)
}

// Why says whether a view needs building, and why. head is the commit it
// would build now, stamp what else would go into it, and published whether
// its current publish exists. ok=false means skip, and why says until when.
func (b Build) Why(head, stamp string, published bool, cooldown time.Duration, now time.Time) (ok bool, why string) {
	switch {
	case b.At.IsZero():
		return true, "never built"
	case !published:
		return true, "not published"
	case b.Stamp != stamp:
		return true, "the spec or ddm changed"
	case b.Error != "" && b.Commit == head:
		// The same commit is retried after an hour, or the cooldown if
		// longer: the cause may have been the network, or something fixed
		// outside the repo. A new commit (a fix) is built as usual.
		retry := b.At.Add(max(cooldown, time.Hour))
		if now.Before(retry) {
			return false, "failed at " + short(b.Commit) + "; retrying after " + retry.Format(time.DateTime)
		}
		return true, "retrying a failed build"
	case b.Commit == head:
		return false, "up to date at " + short(head)
	case now.Before(b.At.Add(cooldown)):
		return false, "new commit " + short(head) + " waits for the cooldown, until " + b.At.Add(cooldown).Format(time.DateTime)
	}
	return true, "new commit " + short(head)
}

func short(c string) string {
	if len(c) == 40 {
		return c[:7]
	}
	return c
}

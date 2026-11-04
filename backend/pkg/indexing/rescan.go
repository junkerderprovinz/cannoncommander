package indexing

import "errors"

// ErrNoScheduler means the source is not indexed, so there is nothing to
// rebuild.
var ErrNoScheduler = errors.New("indexing is turned off for this source")

func (idx *Index) rescanChan() chan struct{} {
	idx.rescanOnce.Do(func() { idx.rescan = make(chan struct{}, 1) })
	return idx.rescan
}

// RequestFullScan asks the scheduler to read the whole source again, every
// scanner in full, ahead of its plan. The scheduler does the work, so the
// pass never runs alongside a scheduled scan of the same folder. A request
// while one is pending is folded into it.
func (idx *Index) RequestFullScan() error {
	idx.mu.RLock()
	running := idx.schedulerStop != nil
	idx.mu.RUnlock()
	if !running || idx.Config.ResolvedRules.IndexingDisabled {
		return ErrNoScheduler
	}
	select {
	case idx.rescanChan() <- struct{}{}:
	default:
	}
	return nil
}

// FullPasses counts the passes over every scanner the scheduler finished,
// the first one after the start included.
func (idx *Index) FullPasses() uint64 { return idx.fullPasses.Load() }

// ActiveScan returns the folder a scanner is reading right now, or "".
func (idx *Index) ActiveScan() string { return idx.getActiveScannerPath() }

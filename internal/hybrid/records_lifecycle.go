package hybrid

import (
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/rainchan/webmux/internal/statefile"
)

// recoverAfterRestart marks active records for recovery when opened.
// Leftover processes matching the recorded root identity, session ID marker,
// or descendant tree are terminated before active records become interrupted.
func (r *records) recoverAfterRestart(now time.Time) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.mu.Lock()
	if err := r.unavailableLocked(); err != nil {
		r.mu.Unlock()
		return err
	}
	next := slices.Clone(r.list)
	r.mu.Unlock()
	changed := false
	for i, rec := range next {
		if rec.State != StateActive && rec.PID == 0 && rec.Process == nil {
			continue
		}
		cleaned := cleanupRecordOrphans(rec.ID, rec.PID, rec.Process)
		rec = rec.clone()
		if cleaned && (rec.PID != 0 || rec.Process != nil) {
			rec.PID, rec.Process = 0, nil
			changed = true
		}
		if rec.State == StateActive {
			at := now.UTC()
			rec.State, rec.InterruptedAt, rec.UpdatedAt = StateInterrupted, &at, at
			if rec.Activity == "" {
				rec.Activity = "idle"
			}
			rec.ResumeOnOpen = true
			changed = true
		}
		next[i] = rec
	}
	if !changed {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.commitLocked(next); err != nil {
		return err
	}
	r.list = next
	return nil
}

// close writes any pending change and releases the file lock.
func (r *records) close() {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	if r.pending != nil {
		r.pending.Stop()
		r.pending = nil
	}
	if r.dirty && r.broken == nil {
		if err := r.commitLocked(r.list); err != nil {
			r.log.Error("hybrid records final write failed; changes are not persisted", "error", err)
		}
	}
	r.closed = true
	r.mu.Unlock()
	r.store.Close()
}

// markReleased sets rel on every record in ids and writes the file at once.
// Every record must exist and be closed or interrupted; otherwise nothing
// changes.
func (r *records) markReleased(ids []string, rel Release) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.unavailableLocked(); err != nil {
		return err
	}
	next := slices.Clone(r.list)
	for _, id := range ids {
		i := r.index(id)
		if i < 0 {
			return fmt.Errorf("%w: 记录 %s", ErrNotFound, id)
		}
		if next[i].State == StateActive {
			return fmt.Errorf("%w: 会话 %s 仍在运行", ErrConflict, id)
		}
		rec := next[i].clone()
		at := rel
		rec.Released = &at
		rec.ResumeOnOpen = false
		next[i] = rec
	}
	if err := r.commitLocked(next); err != nil {
		return err
	}
	r.list = next
	return nil
}

// clearReleased removes the release mark releaseID from every record that
// carries it, writes the file at once and returns their IDs.
func (r *records) clearReleased(releaseID string) ([]string, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.unavailableLocked(); err != nil {
		return nil, err
	}
	next := slices.Clone(r.list)
	var ids []string
	for i, rec := range next {
		if rec.Released == nil || rec.Released.ID != releaseID {
			continue
		}
		rec = rec.clone()
		rec.Released = nil
		next[i] = rec
		ids = append(ids, rec.ID)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if err := r.commitLocked(next); err != nil {
		return nil, err
	}
	r.list = next
	return ids, nil
}

// retryable reports whether a failed write may succeed later: an I/O or
// permission failure can pass, an oversize or frozen store cannot.
func retryable(err error) bool {
	return statefile.IsKind(err, statefile.IO) || statefile.IsKind(err, statefile.Permission)
}

// setLogger sets where failed coalesced writes are reported.
func (r *records) setLogger(log *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = log
}

func (r *records) scheduleLocked() {
	r.scheduleAfterLocked(persistDelay)
}

func (r *records) scheduleAfterLocked(delay time.Duration) {
	if r.pending != nil || !r.dirty {
		return
	}
	r.pending = time.AfterFunc(delay, func() {
		r.writeMu.Lock()
		defer r.writeMu.Unlock()
		r.mu.Lock()
		defer r.mu.Unlock()
		r.pending = nil
		if r.unavailableLocked() != nil {
			return
		}
		if err := r.commitLocked(r.list); err != nil {
			// Memory is now ahead of the disk. Say so, and keep trying while
			// the failure can pass (a full disk); an uncertain commit
			// already froze the store inside statefile.
			r.log.Error("hybrid records write failed; changes are not persisted", "error", err)
		}
	})
}

// commitLocked commits one write transaction. writeMu excludes other
// mutations; mu is released during filesystem I/O and held on return.
func (r *records) commitLocked(list []Record) error {
	if r.pending != nil {
		r.pending.Stop()
		r.pending = nil
	}
	wasDirty := r.dirty
	r.dirty = true
	r.mu.Unlock()
	err := r.commitStore(storedRecords{V: recordsVersion, Records: list})
	r.mu.Lock()
	if err != nil {
		r.dirty = wasDirty
		if wasDirty && retryable(err) {
			r.scheduleAfterLocked(retryDelay)
		}
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	r.dirty = false
	return nil
}

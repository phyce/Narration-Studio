package activity

import (
	"sync"
	"time"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeveritySuccess Severity = "success"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityStatus  Severity = "status"
)

type Entry struct {
	ID          uint64    `json:"id"`
	ChainID     uint64    `json:"chainId"`
	Timestamp   time.Time `json:"timestamp"`
	Severity    Severity  `json:"severity"`
	Summary     string    `json:"summary"`
	Detail      string    `json:"detail,omitempty"`
	StatusID    int       `json:"statusId"`
	StatusLabel string    `json:"statusLabel"`
	Source      string    `json:"source,omitempty"`
}

const flushThreshold = 50
const flushInterval = 2 * time.Second
const maxRecentInMemory = 1000

var (
	bufferMutex    sync.Mutex
	recentEntries  []Entry
	pendingEntries []Entry
	lastID         uint64
	startupOnce    sync.Once
	flusherOnce    sync.Once
	flushSignal    = make(chan struct{}, 1)
)

func Record(entry Entry) Entry {
	defer func() {
		_ = recover()
	}()

	startupOnce.Do(loadLastID)
	flusherOnce.Do(startFlusher)

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	bufferMutex.Lock()
	lastID++
	entry.ID = lastID
	if entry.ChainID == 0 {
		entry.ChainID = entry.ID
	}

	recentEntries = append(recentEntries, entry)
	if len(recentEntries) > maxRecentInMemory {
		recentEntries = recentEntries[1:]
	}

	pendingEntries = append(pendingEntries, entry)
	overThreshold := len(pendingEntries) >= flushThreshold
	bufferMutex.Unlock()

	if overThreshold {
		signalFlush()
	}

	return entry
}

func Recent(limit int) []Entry {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()

	start := 0
	if limit > 0 && len(recentEntries) > limit {
		start = len(recentEntries) - limit
	}

	out := make([]Entry, len(recentEntries)-start)
	copy(out, recentEntries[start:])
	return out
}

func Flush() {
	bufferMutex.Lock()
	if len(pendingEntries) == 0 {
		bufferMutex.Unlock()
		return
	}
	batch := pendingEntries
	pendingEntries = nil
	bufferMutex.Unlock()

	if err := writeBatch(batch); err != nil {
		bufferMutex.Lock()
		pendingEntries = append(batch, pendingEntries...)
		bufferMutex.Unlock()
	}
}

func signalFlush() {
	select {
	case flushSignal <- struct{}{}:
	default:
	}
}

func startFlusher() {
	go func() {
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				Flush()
			case <-flushSignal:
				Flush()
			}
		}
	}()
}

package poller

import (
	"sync"
	"time"
)

// TargetSnapshot holds the latest poll data for a single OLT target.
type TargetSnapshot struct {
	Target       string
	Up           bool
	DriverV2     bool
	Duration     time.Duration
	Labels       map[string]string
	OLT          *OLTStatusSnapshot
	PONPorts     []PONPortSnapshot
	ONUs         []ONUSnapshot
	Alarms       []AlarmSnapshot
	ErrorCount   uint64
	PollTime     time.Time
}

// OLTStatusSnapshot holds OLT-level status data.
type OLTStatusSnapshot struct {
	Vendor        string
	Model         string
	Firmware      string
	SerialNumber  string
	CPUPercent    float64
	MemoryPercent float64
	Temperature   float64
	ActiveONUs    int
	TotalONUs     int
	UptimeSeconds int64
}

// PONPortSnapshot holds per-PON-port data.
type PONPortSnapshot struct {
	Port       string
	ONUCount   int
	RxPowerDBm float64
	TxPowerDBm float64
}

// ONUSnapshot holds per-ONU data.
type ONUSnapshot struct {
	PONPort      string
	ONUID        int
	Serial       string
	Vendor       string
	Model        string
	AdminState   string
	OperState    string
	IsOnline     bool
	RxPowerDBm   float64
	TxPowerDBm   float64
	OLTRxDBm     float64
	DistanceM    int
	Temperature  float64
	IsWithinSpec bool
	BytesUp      uint64
	BytesDown    uint64
}

// AlarmSnapshot holds a single alarm.
type AlarmSnapshot struct {
	Severity string
}

// SnapshotStore is a thread-safe store of per-target snapshots.
type SnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]*TargetSnapshot
}

// NewSnapshotStore creates a new store.
func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{
		snapshots: make(map[string]*TargetSnapshot),
	}
}

// Set stores a snapshot for the given target.
func (s *SnapshotStore) Set(name string, snap *TargetSnapshot) {
	s.mu.Lock()
	s.snapshots[name] = snap
	s.mu.Unlock()
}

// GetAll returns a copy of all current snapshots.
func (s *SnapshotStore) GetAll() []*TargetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*TargetSnapshot, 0, len(s.snapshots))
	for _, snap := range s.snapshots {
		out = append(out, snap)
	}
	return out
}

// Remove deletes the snapshot for a target.
func (s *SnapshotStore) Remove(name string) {
	s.mu.Lock()
	delete(s.snapshots, name)
	s.mu.Unlock()
}

// Len returns the number of stored snapshots.
func (s *SnapshotStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}

// HasData returns true if at least one snapshot exists.
func (s *SnapshotStore) HasData() bool {
	return s.Len() > 0
}

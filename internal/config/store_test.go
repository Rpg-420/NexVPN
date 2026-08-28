package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isolatedStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return &Store{
		dataDir:     dir,
		dataFile:    filepath.Join(dir, "config.json"),
		historyFile: filepath.Join(dir, "history.json"),
		idIndex:     make(map[string]int),
		data:        StoreData{Profiles: []Profile{}, Settings: Settings{}},
		history:     []ConnectionSession{},
	}
}

func TestStoreBatchIDsAreUniqueAndHistoryIsSnapshot(t *testing.T) {
	s := isolatedStore(t)
	ids, err := s.AddProfileBatch([]Profile{{ID: "same"}, {ID: "same"}, {}})
	if err != nil {
		t.Fatalf("AddProfileBatch failed: %v", err)
	}
	if len(ids) != 3 || ids[0] == ids[1] || ids[0] == "" || ids[2] == "" {
		t.Fatalf("unexpected IDs: %#v", ids)
	}

	session := ConnectionSession{ID: "one", ConnectedAt: time.Now()}
	if err := s.RecordConnectionSession(session); err != nil {
		t.Fatal(err)
	}
	history := s.GetConnectionHistory()
	if len(history) != 1 {
		t.Fatalf("history length = %d", len(history))
	}
	history[0].ID = "mutated"
	if s.GetConnectionHistory()[0].ID != "one" {
		t.Fatal("GetConnectionHistory returned mutable internal state")
	}

	// Confirm the asynchronous writer eventually persists valid JSON.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(s.historyFile)
		if readErr == nil {
			var persisted []ConnectionSession
			if json.Unmarshal(data, &persisted) == nil && len(persisted) == 1 {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("history was not persisted")
}

func TestStoreEmptyBatchIsSafe(t *testing.T) {
	s := isolatedStore(t)
	ids, err := s.AddProfileBatch(nil)
	if err != nil {
		t.Fatalf("empty batch failed: %v", err)
	}
	if len(ids) != 0 || len(s.GetProfiles()) != 0 {
		t.Fatalf("empty batch changed store: ids=%v profiles=%d", ids, len(s.GetProfiles()))
	}
}

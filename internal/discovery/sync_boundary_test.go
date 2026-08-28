package discovery

import (
	"testing"
	"time"

	"nexvpn/internal/config"
)

func TestValidSyncPayloadBoundaries(t *testing.T) {
	now := time.Now().Unix()
	base := SyncPayload{
		DeviceName: "peer",
		Timestamp:  now,
		Profiles:   []config.Profile{{ID: "p1"}},
	}
	if !validSyncPayload(base) {
		t.Fatal("current valid payload was rejected")
	}

	cases := []struct {
		name   string
		mutate func(*SyncPayload)
	}{
		{"empty device", func(p *SyncPayload) { p.DeviceName = "" }},
		{"empty profiles", func(p *SyncPayload) { p.Profiles = nil }},
		{"old timestamp", func(p *SyncPayload) { p.Timestamp = now - 601 }},
		{"future timestamp", func(p *SyncPayload) { p.Timestamp = now + 61 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := base
			tc.mutate(&payload)
			if validSyncPayload(payload) {
				t.Fatal("invalid payload was accepted")
			}
		})
	}

	tooMany := base
	tooMany.Profiles = make([]config.Profile, maxSyncProfiles+1)
	if validSyncPayload(tooMany) {
		t.Fatal("payload above profile limit was accepted")
	}
}

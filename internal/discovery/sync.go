package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"nexvpn/internal/config"
)

const (
	SyncBroadcastPort = 10815
	SyncMagicHeader   = "NEXVPN_P2P_SYNC_V1:"
)

type SyncPayload struct {
	DeviceName string           `json:"device_name"`
	Timestamp  int64            `json:"timestamp"`
	Profiles   []config.Profile `json:"profiles"`
}

// BroadcastProfilesToLAN broadcasts selected profiles to local network
func BroadcastProfilesToLAN(profiles []config.Profile, deviceName string) (int, error) {
	if len(profiles) == 0 {
		return 0, fmt.Errorf("no profiles to broadcast")
	}

	if deviceName == "" {
		deviceName = "NexVPN Node"
	}

	payload := SyncPayload{
		DeviceName: deviceName,
		Timestamp:  time.Now().Unix(),
		Profiles:   profiles,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	msg := []byte(SyncMagicHeader + string(bytes))

	// Broadcast address
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", SyncBroadcastPort))
	if err != nil {
		return 0, err
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Send 3 broadcast bursts to overcome packet drops
	for i := 0; i < 3; i++ {
		_, _ = conn.Write(msg)
		time.Sleep(100 * time.Millisecond)
	}

	return len(profiles), nil
}

// ListenForNearbyProfiles listens for incoming broadcast for a specific duration
func ListenForNearbyProfiles(ctx context.Context, timeout time.Duration) (*SyncPayload, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", SyncBroadcastPort))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return nil, err
		}

		data := string(buf[:n])
		if len(data) > len(SyncMagicHeader) && data[:len(SyncMagicHeader)] == SyncMagicHeader {
			jsonStr := data[len(SyncMagicHeader):]
			var payload SyncPayload
			if err := json.Unmarshal([]byte(jsonStr), &payload); err == nil && len(payload.Profiles) > 0 {
				return &payload, nil
			}
		}
	}
}

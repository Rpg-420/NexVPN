//go:build !windows

package core

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// QueryTunnelAdapterBytes scans network interfaces to find active TUN interface on Linux/macOS
func QueryTunnelAdapterBytes() (rx int64, tx int64, found bool) {
	// On Linux, read /proc/net/dev
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.Split(line, ":")
		ifName := strings.TrimSpace(parts[0])

		if strings.HasPrefix(ifName, "tun") || strings.HasPrefix(ifName, "utun") || strings.HasPrefix(ifName, "singbox") || strings.HasPrefix(ifName, "wg") {
			fields := strings.Fields(parts[1])
			if len(fields) >= 9 {
				rxBytes, _ := strconv.ParseInt(fields[0], 10, 64)
				txBytes, _ := strconv.ParseInt(fields[8], 10, 64)
				return rxBytes, txBytes, true
			}
		}
	}

	return 0, 0, false
}

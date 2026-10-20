//go:build !windows

package sysproxy

func flushDNSPlatform() error {
	return nil
}

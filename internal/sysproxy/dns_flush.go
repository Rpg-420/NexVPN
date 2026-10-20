package sysproxy

// FlushDNS flushes the OS DNS resolver cache
func FlushDNS() error {
	return flushDNSPlatform()
}

// FlushDNSAndResetNetwork clears system proxy and flushes the DNS cache
func FlushDNSAndResetNetwork() error {
	_ = ClearSystemProxy()
	return FlushDNS()
}

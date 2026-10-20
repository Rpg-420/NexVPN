package sysproxy

// SetSystemProxy enables system proxy pointing to the given host:port.
func SetSystemProxy(host string, port int) error {
	return setSystemProxyPlatform(host, port)
}

// SetFullSystemProxy configures HTTP and SOCKS proxy in OS settings
func SetFullSystemProxy(httpPort int, socksPort int) error {
	return setFullSystemProxyPlatform(httpPort, socksPort)
}

// ClearSystemProxy disables OS system proxy completely
func ClearSystemProxy() error {
	return clearSystemProxyPlatform()
}

// CheckAndClearStaleProxy resets system proxy if it was left pointing to local NexVPN ports.
// It intentionally does NOT clear the system proxy unconditionally: a user (or their
// employer's network policy) may have configured a legitimate proxy independent of
// NexVPN, and wiping it out on every app launch would silently break their network.
func CheckAndClearStaleProxy() {
	if !isStaleNexVPNProxy() {
		return
	}
	_ = ClearSystemProxy()
}

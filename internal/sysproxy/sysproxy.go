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

// CheckAndClearStaleProxy resets system proxy if it was left pointing to local NexVPN ports
func CheckAndClearStaleProxy() {
	_ = ClearSystemProxy()
}

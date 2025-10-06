package util

// SafeClose safely closes a channel, preventing double-close panics
func SafeClose(ch chan struct{}) {
	select {
	case <-ch:
		// Already closed
	default:
		close(ch)
	}
}

// SafeCloseBool safely closes a bool channel, preventing double-close panics
func SafeCloseBool(ch chan bool) {
	select {
	case <-ch:
		// Already closed
	default:
		close(ch)
	}
}

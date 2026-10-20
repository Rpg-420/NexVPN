package sysproc

import "context"

// PowerEvent kinds. They are strings rather than an enum because they are
// written straight into the launch diary and read by a human debugging a
// dropped connection.
const (
	// PowerSuspend fires as the machine is about to sleep. Sockets are torn
	// down by the OS, so an established tunnel never survives it.
	PowerSuspend = "suspend"
	// PowerResume fires when the machine wakes. This is the interesting one:
	// the core process may have died silently while the system was asleep.
	PowerResume = "resume"
	// PowerAC and PowerBattery report a mains/battery transition. On laptops
	// this is a useful proxy for "the user just unplugged and the radio may
	// be about to change", and it is cheap to observe.
	PowerAC      = "ac"
	PowerBattery = "battery"
)

// PowerEvent reports a suspend/resume or mains-power transition.
type PowerEvent struct {
	Kind string
}

// WatchPowerEvents reports power transitions until ctx is cancelled, at which
// point the returned channel is closed.
//
// Nothing here is allowed to be load-bearing for correctness: a machine where
// the registration fails simply behaves as it did before, because the engine's
// health monitor still notices a dead core. The value is latency — a laptop
// that wakes with a dead tunnel reconnects in about a second instead of
// waiting for the next health check.
func WatchPowerEvents(ctx context.Context) <-chan PowerEvent {
	out := make(chan PowerEvent, 4)
	registerPowerNotifications(ctx, func(kind string) {
		select {
		case out <- PowerEvent{Kind: kind}:
		default: // consumer is behind; drop rather than block a system thread
		}
	})

	go func() {
		<-ctx.Done()
		close(out)
	}()

	return out
}

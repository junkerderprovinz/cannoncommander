//go:build desktop

package desktop

import "sync"

// waitingEvents bounds what the app may emit before the server listens, such
// as folders handed over on the command line while the database opens.
const waitingEvents = 64

type event struct {
	typ  string
	data any
}

var (
	eventMu sync.Mutex
	send    func(typ string, data any)
	waiting []event
)

// Emit hands an event from the desktop app to its window, which receives it
// on the event stream as "desktop." plus name, with data as its payload.
func Emit(name string, data any) {
	e := event{"desktop." + name, data}
	eventMu.Lock()
	defer eventMu.Unlock()
	if send == nil {
		if len(waiting) == waitingEvents {
			waiting = waiting[1:]
		}
		waiting = append(waiting, e)
		return
	}
	send(e.typ, e.data)
}

// Listen is how the server takes the app's events and publishes them. The
// events emitted before it listened go out first.
func Listen(publish func(typ string, data any)) {
	eventMu.Lock()
	defer eventMu.Unlock()
	send = publish
	if send == nil {
		return
	}
	for _, e := range waiting {
		send(e.typ, e.data)
	}
	waiting = nil
}

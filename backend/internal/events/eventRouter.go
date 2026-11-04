package events

import (
	"sync"
)

type EventMessage struct {
	EventType string `json:"eventType"`
	Message   string `json:"message"`
}

type sourceEvent struct {
	source string
	event  EventMessage
}

type userEvent struct {
	event EventMessage
	users []string
}

var (
	userEventChan    = make(chan userEvent, 100)
	sourceUpdateChan = make(chan sourceEvent, 100)

	userClientsMu   sync.RWMutex
	userClients     = make(map[string][]chan EventMessage)
	sourceClientsMu sync.RWMutex
	sourceClients   = make(map[string]map[chan EventMessage]struct{})
)

func init() {
	go handleUserEvents()
	go handleSourceUpdates() // Add this

}

func handleUserEvents() {
	for ue := range userEventChan {
		for _, user := range ue.users {
			userClientsMu.RLock()
			for _, ch := range userClients[user] {
				select {
				case ch <- ue.event:
				default:
				}
			}
			userClientsMu.RUnlock()
		}
	}
}

func SendSourceUpdate(source string, message string) {
	event := sourceEvent{
		source: source,
		event: EventMessage{
			EventType: "sourceUpdate",
			Message:   message,
		},
	}
	select {
	case sourceUpdateChan <- event:
		// Event sent successfully
	default:
		// Channel is full, log warning but don't block
		// This shouldn't happen under normal circumstances
	}
}

func handleSourceUpdates() {
	for update := range sourceUpdateChan {
		sourceClientsMu.RLock()
		for ch := range sourceClients[update.source] {
			select {
			case ch <- update.event:
			default:
				// Channel full, message dropped
			}
		}
		sourceClientsMu.RUnlock()
	}
}

func Shutdown() {
	userClientsMu.Lock()
	defer userClientsMu.Unlock()
	sourceClientsMu.Lock()
	defer sourceClientsMu.Unlock()

	for username, clientChannels := range userClients {
		for _, ch := range clientChannels {
			// Clean up source clients
			for source, clients := range sourceClients {
				delete(clients, ch)
				if len(clients) == 0 {
					delete(sourceClients, source)
				}
			}
			close(ch)
		}
		delete(userClients, username)
	}
}

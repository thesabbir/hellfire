package bus

import (
	"sync"
)

// EventType represents different types of configuration events
type EventType string

const (
	EventConfigChanged        EventType = "config.changed"
	EventConfigCommitted      EventType = "config.committed"
	EventConfigReverted       EventType = "config.reverted"
	EventSnapshotCreated      EventType = "snapshot.created"
	EventTransactionStarted   EventType = "transaction.started"
	EventTransactionCompleted EventType = "transaction.completed"
	EventTransactionFailed    EventType = "transaction.failed"
	EventRollbackStarted      EventType = "rollback.started"
)

// Event represents a configuration event
type Event struct {
	Type       EventType
	ConfigName string
	Data       interface{}
}

// Handler is a function that handles events
type Handler func(event Event)

// Bus is a simple pub/sub event bus
type Bus struct {
	mu        sync.RWMutex
	handlers  map[EventType][]Handler
	chanSize  int
	eventChan chan Event
	wg        sync.WaitGroup
	stopped   bool
}

// NewBus creates a new event bus
func NewBus() *Bus {
	b := &Bus{
		handlers:  make(map[EventType][]Handler),
		chanSize:  100,
		eventChan: make(chan Event, 100),
	}
	b.start()
	return b
}

// Subscribe subscribes a handler to an event type
func (b *Bus) Subscribe(eventType EventType, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.handlers[eventType] == nil {
		b.handlers[eventType] = make([]Handler, 0)
	}
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish publishes an event to all subscribers
func (b *Bus) Publish(event Event) {
	if b.stopped {
		return
	}

	select {
	case b.eventChan <- event:
	default:
		// Event channel full, drop event
		// In production, you might want to log this
	}
}

// start starts the event processing goroutine
func (b *Bus) start() {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for event := range b.eventChan {
			b.dispatch(event)
		}
	}()
}

// dispatch dispatches an event to all registered handlers
func (b *Bus) dispatch(event Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	for _, handler := range handlers {
		// Run handlers in goroutines to avoid blocking
		go func(h Handler) {
			defer func() {
				// Recover from panics in handlers
				if r := recover(); r != nil {
					// In production, log this
				}
			}()
			h(event)
		}(handler)
	}
}

// Stop stops the event bus
func (b *Bus) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	b.mu.Unlock()

	close(b.eventChan)
	b.wg.Wait()
}

// GlobalBus is the global event bus instance
var GlobalBus = NewBus()

// Subscribe subscribes to the global bus
func Subscribe(eventType EventType, handler Handler) {
	GlobalBus.Subscribe(eventType, handler)
}

// Publish publishes to the global bus
func Publish(event Event) {
	GlobalBus.Publish(event)
}

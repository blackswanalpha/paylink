package events

import (
	"context"
	"log"
	"sync"
)

// BusConfig holds configuration for the event bus.
type BusConfig struct {
	InternalBufferSize   int
	SubscriberBufferSize int
}

// DefaultBusConfig returns sensible defaults.
func DefaultBusConfig() BusConfig {
	return BusConfig{
		InternalBufferSize:   4096,
		SubscriberBufferSize: 256,
	}
}

// Subscriber is a handle for receiving events.
type Subscriber struct {
	id     uint64
	ch     chan *Event
	done   chan struct{}
	closed bool
	mu     sync.Mutex
}

// Ch returns the event channel for this subscriber.
func (s *Subscriber) Ch() <-chan *Event {
	return s.ch
}

// Done returns a channel that is closed when this subscriber is removed.
func (s *Subscriber) Done() <-chan struct{} {
	return s.done
}

// Bus is the internal event pub/sub system.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[uint64]*Subscriber
	nextID      uint64
	publishCh   chan *Event
	config      BusConfig
}

// NewBus creates a new event bus.
func NewBus(config BusConfig) *Bus {
	return &Bus{
		subscribers: make(map[uint64]*Subscriber),
		publishCh:   make(chan *Event, config.InternalBufferSize),
		config:      config,
	}
}

// Start begins the dispatch loop. Blocks until ctx is cancelled.
func (b *Bus) Start(ctx context.Context) {
	log.Println("Event bus started")
	for {
		select {
		case <-ctx.Done():
			b.closeAll()
			log.Println("Event bus stopped")
			return
		case evt := <-b.publishCh:
			b.fanOut(evt)
		}
	}
}

// Publish sends an event to all subscribers. Non-blocking for the caller.
func (b *Bus) Publish(evt *Event) {
	select {
	case b.publishCh <- evt:
	default:
		log.Printf("Event bus: internal buffer full, dropping event seq=%d kind=%s", evt.Sequence, evt.Kind)
	}
}

// Subscribe creates a new subscriber with the default buffer size.
func (b *Bus) Subscribe() *Subscriber {
	return b.SubscribeWithBuffer(b.config.SubscriberBufferSize)
}

// SubscribeWithBuffer creates a new subscriber with a custom buffer size.
func (b *Bus) SubscribeWithBuffer(bufferSize int) *Subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	sub := &Subscriber{
		id:   b.nextID,
		ch:   make(chan *Event, bufferSize),
		done: make(chan struct{}),
	}
	b.subscribers[sub.id] = sub
	return sub
}

// Unsubscribe removes a subscriber.
func (b *Bus) Unsubscribe(sub *Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub.mu.Lock()
	defer sub.mu.Unlock()

	if !sub.closed {
		sub.closed = true
		close(sub.done)
		close(sub.ch)
	}
	delete(b.subscribers, sub.id)
}

// SubscriberCount returns the number of active subscribers.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

func (b *Bus) fanOut(evt *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		sub.mu.Lock()
		if sub.closed {
			sub.mu.Unlock()
			continue
		}
		select {
		case sub.ch <- evt:
		default:
			log.Printf("Event bus: subscriber %d buffer full, dropping event seq=%d", sub.id, evt.Sequence)
		}
		sub.mu.Unlock()
	}
}

func (b *Bus) closeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, sub := range b.subscribers {
		sub.mu.Lock()
		if !sub.closed {
			sub.closed = true
			close(sub.done)
			close(sub.ch)
		}
		sub.mu.Unlock()
		delete(b.subscribers, id)
	}
}

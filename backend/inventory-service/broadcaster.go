package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type SeatUpdateEvent struct {
	EventID   string    `json:"eventId"`
	SeatID    string    `json:"seatId"`
	Row       string    `json:"row,omitempty"`
	Number    int       `json:"number,omitempty"`
	TierID    string    `json:"tierId,omitempty"`
	Status    string    `json:"status"` // AVAILABLE, HOLD, BOOKED
	Timestamp time.Time `json:"timestamp"`
}

type SeatBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan SeatUpdateEvent]struct{}
	rdb         *redis.Client
}

func NewSeatBroadcaster(rdb *redis.Client) *SeatBroadcaster {
	sb := &SeatBroadcaster{
		subscribers: make(map[string]map[chan SeatUpdateEvent]struct{}),
		rdb:         rdb,
	}

	if rdb != nil {
		go sb.listenRedisPubSub()
	}

	return sb
}

func (sb *SeatBroadcaster) Subscribe(eventID string) (chan SeatUpdateEvent, func()) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	ch := make(chan SeatUpdateEvent, 50)
	if _, ok := sb.subscribers[eventID]; !ok {
		sb.subscribers[eventID] = make(map[chan SeatUpdateEvent]struct{})
	}
	sb.subscribers[eventID][ch] = struct{}{}

	unsubscribe := func() {
		sb.mu.Lock()
		defer sb.mu.Unlock()
		if subs, ok := sb.subscribers[eventID]; ok {
			delete(subs, ch)
			close(ch)
			if len(subs) == 0 {
				delete(sb.subscribers, eventID)
			}
		}
	}

	return ch, unsubscribe
}

func (sb *SeatBroadcaster) Broadcast(event SeatUpdateEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Dispatch to local subscribers
	sb.mu.RLock()
	if subs, ok := sb.subscribers[event.EventID]; ok {
		for ch := range subs {
			select {
			case ch <- event:
			default:
				// Skip if subscriber channel buffer is full
			}
		}
	}
	sb.mu.RUnlock()

	// Publish to Redis for multi-instance distributed sync
	if sb.rdb != nil {
		go func(e SeatUpdateEvent) {
			data, err := json.Marshal(e)
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = sb.rdb.Publish(ctx, "inventory:seat_updates:"+e.EventID, string(data)).Err()
			}
		}(event)
	}
}

func (sb *SeatBroadcaster) BroadcastMultiple(eventID string, status string, seatIDs []string) {
	for _, seatID := range seatIDs {
		sb.Broadcast(SeatUpdateEvent{
			EventID:   eventID,
			SeatID:    seatID,
			Status:    status,
			Timestamp: time.Now().UTC(),
		})
	}
}

func (sb *SeatBroadcaster) listenRedisPubSub() {
	ctx := context.Background()
	pubsub := sb.rdb.PSubscribe(ctx, "inventory:seat_updates:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var event SeatUpdateEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
			sb.mu.RLock()
			if subs, ok := sb.subscribers[event.EventID]; ok {
				for subCh := range subs {
					select {
					case subCh <- event:
					default:
					}
				}
			}
			sb.mu.RUnlock()
		} else {
			log.Printf("Error unmarshaling Redis seat update: %v", err)
		}
	}
}

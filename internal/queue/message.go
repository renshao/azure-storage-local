package queue

import (
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

// Message represents a single queue message.
type Message struct {
	ID              string
	Text            string
	InsertionTime   time.Time
	ExpirationTime  time.Time
	TimeNextVisible time.Time
	PopReceipt      string
	DequeueCount    int
}

// NewMessage creates a new message with the given parameters.
// visibilityTimeout: seconds until the message becomes visible (0 = immediately visible)
// ttlSeconds: time-to-live in seconds (-1 = never expires, 0 = default 7 days)
func NewMessage(text string, now time.Time, visibilityTimeout, ttlSeconds int) *Message {
	if ttlSeconds == 0 {
		ttlSeconds = 7 * 24 * 60 * 60 // 7 days default
	}

	var expirationTime time.Time
	if ttlSeconds == -1 {
		expirationTime = now.Add(100 * 365 * 24 * time.Hour) // ~100 years
	} else {
		expirationTime = now.Add(time.Duration(ttlSeconds) * time.Second)
	}

	return &Message{
		ID:              uuid.New().String(),
		Text:            text,
		InsertionTime:   now,
		ExpirationTime:  expirationTime,
		TimeNextVisible: now.Add(time.Duration(visibilityTimeout) * time.Second),
		PopReceipt:      generatePopReceipt(),
		DequeueCount:    0,
	}
}

// IsVisible returns true if the message is currently visible.
func (m *Message) IsVisible(now time.Time) bool {
	return now.After(m.TimeNextVisible) || now.Equal(m.TimeNextVisible)
}

// IsExpired returns true if the message has passed its TTL.
func (m *Message) IsExpired(now time.Time) bool {
	return now.After(m.ExpirationTime)
}

// Dequeue marks the message as dequeued: increments count, sets visibility timeout, new pop receipt.
func (m *Message) Dequeue(now time.Time, visibilityTimeout int) {
	m.DequeueCount++
	m.TimeNextVisible = now.Add(time.Duration(visibilityTimeout) * time.Second)
	m.PopReceipt = generatePopReceipt()
}

func generatePopReceipt() string {
	return base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))
}

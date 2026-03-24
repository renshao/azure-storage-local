package queue

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Queue represents a single queue with its metadata and messages.
type Queue struct {
	Name      string
	Metadata  map[string]string
	Messages  []*Message
	mu        sync.Mutex
	CreatedAt time.Time
}

// Store is the in-memory, thread-safe store for all queues.
type Store struct {
	mu     sync.RWMutex
	queues map[string]*Queue
}

// NewStore creates a new empty queue store.
func NewStore() *Store {
	return &Store{
		queues: make(map[string]*Queue),
	}
}

// CreateQueue creates a new queue. Returns (created bool, conflict bool).
// If queue exists with same metadata: created=false, conflict=false (204).
// If queue exists with different metadata: created=false, conflict=true (409).
// If queue is new: created=true, conflict=false (201).
func (s *Store) CreateQueue(name string, metadata map[string]string) (created bool, conflict bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.queues[name]; ok {
		if metadataEqual(existing.Metadata, metadata) {
			return false, false
		}
		return false, true
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	s.queues[name] = &Queue{
		Name:      name,
		Metadata:  metadata,
		Messages:  make([]*Message, 0),
		CreatedAt: time.Now().UTC(),
	}
	return true, false
}

// DeleteQueue removes a queue. Returns false if queue doesn't exist.
func (s *Store) DeleteQueue(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		return false
	}
	delete(s.queues, name)
	return true
}

// GetQueue returns a queue by name, or nil if not found.
func (s *Store) GetQueue(name string) *Queue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queues[name]
}

// ListQueues returns all queue names sorted alphabetically.
func (s *Store) ListQueues() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.queues))
	for name := range s.queues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// QueueInfo holds queue name and metadata for list operations.
type QueueInfo struct {
	Name     string
	Metadata map[string]string
}

// ListQueuesWithDetails returns queues filtered by prefix, with optional metadata.
func (s *Store) ListQueuesWithDetails(prefix string, includeMetadata bool) []QueueInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []QueueInfo
	for name, q := range s.queues {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		info := QueueInfo{Name: name}
		if includeMetadata {
			q.mu.Lock()
			meta := make(map[string]string, len(q.Metadata))
			for k, v := range q.Metadata {
				meta[k] = v
			}
			q.mu.Unlock()
			info.Metadata = meta
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// SetMetadata replaces all metadata on a queue. Returns false if queue not found.
func (s *Store) SetMetadata(queueName string, metadata map[string]string) bool {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return false
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if metadata == nil {
		metadata = make(map[string]string)
	}
	q.Metadata = metadata
	return true
}

// GetMetadata returns the metadata and approximate message count for a queue.
func (s *Store) GetMetadata(queueName string) (metadata map[string]string, approxCount int, ok bool) {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return nil, 0, false
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Copy metadata
	meta := make(map[string]string, len(q.Metadata))
	for k, v := range q.Metadata {
		meta[k] = v
	}
	return meta, len(q.Messages), true
}

// PutMessage adds a message to the queue.
func (s *Store) PutMessage(queueName string, text string, visibilityTimeout, ttlSeconds int) (*Message, error) {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	now := time.Now().UTC()
	msg := NewMessage(text, now, visibilityTimeout, ttlSeconds)

	q.mu.Lock()
	defer q.mu.Unlock()
	q.Messages = append(q.Messages, msg)
	return msg, nil
}

// GetMessages retrieves and makes invisible up to numMessages visible messages.
func (s *Store) GetMessages(queueName string, numMessages, visibilityTimeout int) ([]*Message, error) {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	now := time.Now().UTC()
	q.mu.Lock()
	defer q.mu.Unlock()

	var result []*Message
	for _, msg := range q.Messages {
		if len(result) >= numMessages {
			break
		}
		if msg.IsVisible(now) && !msg.IsExpired(now) {
			msg.Dequeue(now, visibilityTimeout)
			result = append(result, msg)
		}
	}
	return result, nil
}

// PeekMessages retrieves up to numMessages visible messages without changing visibility.
func (s *Store) PeekMessages(queueName string, numMessages int) ([]*Message, error) {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	now := time.Now().UTC()
	q.mu.Lock()
	defer q.mu.Unlock()

	var result []*Message
	for _, msg := range q.Messages {
		if len(result) >= numMessages {
			break
		}
		if msg.IsVisible(now) && !msg.IsExpired(now) {
			result = append(result, msg)
		}
	}
	return result, nil
}

// DeleteMessage deletes a message by ID and PopReceipt.
func (s *Store) DeleteMessage(queueName, messageID, popReceipt string) error {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return fmt.Errorf("queue not found: %s", queueName)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	for i, msg := range q.Messages {
		if msg.ID == messageID {
			if msg.PopReceipt != popReceipt {
				return fmt.Errorf("pop receipt mismatch")
			}
			q.Messages = append(q.Messages[:i], q.Messages[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("message not found: %s", messageID)
}

// ClearMessages removes all messages from a queue.
func (s *Store) ClearMessages(queueName string) error {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return fmt.Errorf("queue not found: %s", queueName)
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	q.Messages = make([]*Message, 0)
	return nil
}

// UpdateMessage updates a message's content and/or visibility timeout.
func (s *Store) UpdateMessage(queueName, messageID, popReceipt string, visibilityTimeout int, newText *string) (*Message, error) {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	now := time.Now().UTC()
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, msg := range q.Messages {
		if msg.ID == messageID {
			if msg.PopReceipt != popReceipt {
				return nil, fmt.Errorf("pop receipt mismatch")
			}
			if newText != nil {
				msg.Text = *newText
			}
			msg.TimeNextVisible = now.Add(time.Duration(visibilityTimeout) * time.Second)
			msg.PopReceipt = generatePopReceipt()
			return msg, nil
		}
	}
	return nil, fmt.Errorf("message not found: %s", messageID)
}

// ExpireMessages removes expired messages from all queues. Called by TTL worker.
func (s *Store) ExpireMessages() {
	s.mu.RLock()
	queues := make([]*Queue, 0, len(s.queues))
	for _, q := range s.queues {
		queues = append(queues, q)
	}
	s.mu.RUnlock()

	now := time.Now().UTC()
	for _, q := range queues {
		q.mu.Lock()
		filtered := make([]*Message, 0, len(q.Messages))
		for _, msg := range q.Messages {
			if !msg.IsExpired(now) {
				filtered = append(filtered, msg)
			}
		}
		q.Messages = filtered
		q.mu.Unlock()
	}
}

// GetAllMessages returns all messages (including invisible) for UI display.
func (s *Store) GetAllMessages(queueName string) ([]*Message, error) {
	s.mu.RLock()
	q := s.queues[queueName]
	s.mu.RUnlock()

	if q == nil {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	now := time.Now().UTC()
	q.mu.Lock()
	defer q.mu.Unlock()

	var result []*Message
	for _, msg := range q.Messages {
		if !msg.IsExpired(now) {
			result = append(result, msg)
		}
	}
	return result, nil
}

func metadataEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

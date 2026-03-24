package blob

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Blob represents a single blob with its data and metadata.
type Blob struct {
	Name         string
	Data         []byte
	ContentType  string
	Metadata     map[string]string
	BlobType     string
	CreatedAt    time.Time
	LastModified time.Time
	ETag         string
}

// Container represents a blob container.
type Container struct {
	Name         string
	Metadata     map[string]string
	Blobs        map[string]*Blob
	mu           sync.RWMutex
	CreatedAt    time.Time
	LastModified time.Time
	ETag         string
}

// ContainerInfo holds container details for list operations.
type ContainerInfo struct {
	Name         string
	Metadata     map[string]string
	LastModified time.Time
	ETag         string
}

// BlobInfo holds blob details for list operations.
type BlobInfo struct {
	Name         string
	ContentType  string
	ContentLen   int64
	BlobType     string
	LastModified time.Time
	ETag         string
	Metadata     map[string]string
}

// Store is the in-memory, thread-safe store for all blob containers.
type Store struct {
	mu         sync.RWMutex
	containers map[string]*Container
}

// NewStore creates a new empty blob store.
func NewStore() *Store {
	return &Store{
		containers: make(map[string]*Container),
	}
}

// CreateContainer creates a new container. Returns (created, conflict).
func (s *Store) CreateContainer(name string, metadata map[string]string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.containers[name]; ok {
		return false, true
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	now := time.Now().UTC()
	s.containers[name] = &Container{
		Name:         name,
		Metadata:     metadata,
		Blobs:        make(map[string]*Blob),
		CreatedAt:    now,
		LastModified: now,
		ETag:         generateETag(),
	}
	return true, false
}

// DeleteContainer removes a container. Returns false if not found.
func (s *Store) DeleteContainer(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.containers[name]; !ok {
		return false
	}
	delete(s.containers, name)
	return true
}

// GetContainer returns a container by name, or nil.
func (s *Store) GetContainer(name string) *Container {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.containers[name]
}

// ListContainers returns containers filtered by prefix, sorted alphabetically.
func (s *Store) ListContainers(prefix string, includeMetadata bool) []ContainerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ContainerInfo
	for name, c := range s.containers {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		info := ContainerInfo{
			Name:         name,
			LastModified: c.LastModified,
			ETag:         c.ETag,
		}
		if includeMetadata {
			c.mu.RLock()
			meta := make(map[string]string, len(c.Metadata))
			for k, v := range c.Metadata {
				meta[k] = v
			}
			c.mu.RUnlock()
			info.Metadata = meta
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// PutBlob stores a blob in a container. Creates or overwrites.
func (s *Store) PutBlob(containerName, blobName string, data []byte, contentType string, blobType string, metadata map[string]string) (*Blob, error) {
	s.mu.RLock()
	c := s.containers[containerName]
	s.mu.RUnlock()

	if c == nil {
		return nil, fmt.Errorf("container not found: %s", containerName)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if metadata == nil {
		metadata = make(map[string]string)
	}
	if blobType == "" {
		blobType = "BlockBlob"
	}

	now := time.Now().UTC()
	blob := &Blob{
		Name:         blobName,
		Data:         data,
		ContentType:  contentType,
		Metadata:     metadata,
		BlobType:     blobType,
		CreatedAt:    now,
		LastModified: now,
		ETag:         generateETag(),
	}

	c.mu.Lock()
	c.Blobs[blobName] = blob
	c.LastModified = now
	c.ETag = generateETag()
	c.mu.Unlock()

	return blob, nil
}

// GetBlob retrieves a blob from a container.
func (s *Store) GetBlob(containerName, blobName string) (*Blob, error) {
	s.mu.RLock()
	c := s.containers[containerName]
	s.mu.RUnlock()

	if c == nil {
		return nil, fmt.Errorf("container not found: %s", containerName)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	blob, ok := c.Blobs[blobName]
	if !ok {
		return nil, fmt.Errorf("blob not found: %s", blobName)
	}
	return blob, nil
}

// GetBlobMetadata retrieves blob metadata.
func (s *Store) GetBlobMetadata(containerName, blobName string) (map[string]string, *Blob, error) {
	blob, err := s.GetBlob(containerName, blobName)
	if err != nil {
		return nil, nil, err
	}
	meta := make(map[string]string, len(blob.Metadata))
	for k, v := range blob.Metadata {
		meta[k] = v
	}
	return meta, blob, nil
}

// ListBlobs returns blobs in a container, optionally filtered by prefix.
func (s *Store) ListBlobs(containerName, prefix, delimiter string, includeMetadata bool) ([]BlobInfo, []string, error) {
	s.mu.RLock()
	c := s.containers[containerName]
	s.mu.RUnlock()

	if c == nil {
		return nil, nil, fmt.Errorf("container not found: %s", containerName)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	prefixSet := make(map[string]bool)
	var blobs []BlobInfo

	for name, b := range c.Blobs {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		// Handle delimiter for virtual directory support
		if delimiter != "" {
			rest := strings.TrimPrefix(name, prefix)
			idx := strings.Index(rest, delimiter)
			if idx >= 0 {
				// This is a "virtual directory" prefix
				pfx := prefix + rest[:idx+len(delimiter)]
				prefixSet[pfx] = true
				continue
			}
		}

		info := BlobInfo{
			Name:         name,
			ContentType:  b.ContentType,
			ContentLen:   int64(len(b.Data)),
			BlobType:     b.BlobType,
			LastModified: b.LastModified,
			ETag:         b.ETag,
		}
		if includeMetadata {
			meta := make(map[string]string, len(b.Metadata))
			for k, v := range b.Metadata {
				meta[k] = v
			}
			info.Metadata = meta
		}
		blobs = append(blobs, info)
	}

	sort.Slice(blobs, func(i, j int) bool {
		return blobs[i].Name < blobs[j].Name
	})

	var prefixes []string
	for p := range prefixSet {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	return blobs, prefixes, nil
}

func generateETag() string {
	return fmt.Sprintf("\"%x\"", time.Now().UnixNano())
}

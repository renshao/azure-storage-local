package web

import (
	"embed"
	"encoding/json"
	"net/http"
	"strings"

	"azure-storage-lite/internal/blob"
	"azure-storage-lite/internal/queue"
)

//go:embed templates/index.html
var templateFS embed.FS

// Server creates the web UI HTTP handler.
func Server(queueStore *queue.Store, blobStore *blob.Store) http.Handler {
	mux := http.NewServeMux()

	// Serve the HTML UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, err := templateFS.ReadFile("templates/index.html")
		if err != nil {
			http.Error(w, "Internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// JSON API: list queues
	mux.HandleFunc("/api/queues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}
		names := queueStore.ListQueues()
		type queueInfo struct {
			Name         string `json:"name"`
			MessageCount int    `json:"messageCount"`
		}
		result := make([]queueInfo, 0, len(names))
		for _, name := range names {
			_, count, ok := queueStore.GetMetadata(name)
			if ok {
				result = append(result, queueInfo{Name: name, MessageCount: count})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// JSON API: get messages for a queue, or clear messages
	mux.HandleFunc("/api/queues/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/queues/")
		parts := strings.SplitN(path, "/", 2)
		queueName := parts[0]

		if len(parts) < 2 || parts[1] != "messages" {
			http.Error(w, "Not found", 404)
			return
		}

		if r.Method == http.MethodDelete {
			queueStore.ClearMessages(queueName)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}

		msgs, err := queueStore.GetAllMessages(queueName)
		if err != nil {
			http.Error(w, "Queue not found", 404)
			return
		}

		type msgInfo struct {
			MessageId       string `json:"messageId"`
			MessageText     string `json:"messageText"`
			InsertionTime   string `json:"insertionTime"`
			ExpirationTime  string `json:"expirationTime"`
			TimeNextVisible string `json:"timeNextVisible"`
			PopReceipt      string `json:"popReceipt"`
			DequeueCount    int    `json:"dequeueCount"`
		}

		result := make([]msgInfo, 0, len(msgs))
		for _, msg := range msgs {
			result = append(result, msgInfo{
				MessageId:       msg.ID,
				MessageText:     msg.Text,
				InsertionTime:   msg.InsertionTime.UTC().Format("2006-01-02T15:04:05Z"),
				ExpirationTime:  msg.ExpirationTime.UTC().Format("2006-01-02T15:04:05Z"),
				TimeNextVisible: msg.TimeNextVisible.UTC().Format("2006-01-02T15:04:05Z"),
				PopReceipt:      msg.PopReceipt,
				DequeueCount:    msg.DequeueCount,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// JSON API: list containers
	mux.HandleFunc("/api/containers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}
		containers := blobStore.ListContainers("", false)
		type containerInfo struct {
			Name      string `json:"name"`
			BlobCount int    `json:"blobCount"`
		}
		result := make([]containerInfo, 0, len(containers))
		for _, c := range containers {
			cont := blobStore.GetContainer(c.Name)
			count := 0
			if cont != nil {
				blobs, _, _ := blobStore.ListBlobs(c.Name, "", "", false)
				count = len(blobs)
			}
			result = append(result, containerInfo{Name: c.Name, BlobCount: count})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// JSON API: list blobs in a container
	mux.HandleFunc("/api/containers/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
		parts := strings.SplitN(path, "/", 2)
		containerName := parts[0]

		if len(parts) < 2 || parts[1] != "blobs" {
			http.Error(w, "Not found", 404)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}

		blobs, _, err := blobStore.ListBlobs(containerName, "", "", true)
		if err != nil {
			http.Error(w, "Container not found", 404)
			return
		}

		type blobInfo struct {
			Name         string            `json:"name"`
			ContentType  string            `json:"contentType"`
			Size         int64             `json:"size"`
			BlobType     string            `json:"blobType"`
			LastModified string            `json:"lastModified"`
			Metadata     map[string]string `json:"metadata,omitempty"`
		}

		result := make([]blobInfo, 0, len(blobs))
		for _, b := range blobs {
			result = append(result, blobInfo{
				Name:         b.Name,
				ContentType:  b.ContentType,
				Size:         b.ContentLen,
				BlobType:     b.BlobType,
				LastModified: b.LastModified.UTC().Format("2006-01-02T15:04:05Z"),
				Metadata:     b.Metadata,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	return mux
}

package web

import (
	"embed"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/azure-storage-local/internal/queue"
)

//go:embed templates/index.html
var templateFS embed.FS

// Server creates the web UI HTTP handler.
func Server(store *queue.Store) http.Handler {
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
		names := store.ListQueues()
		type queueInfo struct {
			Name         string `json:"name"`
			MessageCount int    `json:"messageCount"`
		}
		result := make([]queueInfo, 0, len(names))
		for _, name := range names {
			_, count, ok := store.GetMetadata(name)
			if ok {
				result = append(result, queueInfo{Name: name, MessageCount: count})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// JSON API: get messages for a queue, or clear messages
	mux.HandleFunc("/api/queues/", func(w http.ResponseWriter, r *http.Request) {
		// Parse: /api/queues/{name}/messages
		path := strings.TrimPrefix(r.URL.Path, "/api/queues/")
		parts := strings.SplitN(path, "/", 2)
		queueName := parts[0]

		if len(parts) < 2 || parts[1] != "messages" {
			http.Error(w, "Not found", 404)
			return
		}

		if r.Method == http.MethodDelete {
			store.ClearMessages(queueName)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}

		msgs, err := store.GetAllMessages(queueName)
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

	return mux
}

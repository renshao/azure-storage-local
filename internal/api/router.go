package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"azure-storage-lite/internal/queue"
)

// Router creates the HTTP handler for the Azure Queue Storage API.
func Router(store *queue.Store) http.Handler {
	mux := http.NewServeMux()

	// All routes are under /{account}/...
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 4)

		if len(parts) < 1 || parts[0] == "" {
			writeError(w, http.StatusBadRequest, "InvalidUri", "The request URI is invalid.")
			return
		}

		account := parts[0]
		if account != AccountName {
			writeError(w, http.StatusForbidden, "AuthenticationFailed", "Account not found.")
			return
		}

		// Route: /{account}?comp=list (List Queues)
		comp := r.URL.Query().Get("comp")
		if (len(parts) < 2 || parts[1] == "") && comp == "list" {
			if r.Method == http.MethodGet {
				listQueues(w, r, store)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "The HTTP method is not allowed.")
			}
			return
		}

		if len(parts) < 2 || parts[1] == "" {
			writeError(w, http.StatusBadRequest, "InvalidUri", "Queue name is required.")
			return
		}

		queueName := parts[1]

		// Route: /{account}/{queue}/messages/{messageId}
		if len(parts) >= 4 && parts[2] == "messages" && parts[3] != "" {
			messageID := parts[3]
			handleMessageByID(w, r, store, queueName, messageID)
			return
		}

		// Route: /{account}/{queue}/messages
		if len(parts) >= 3 && parts[2] == "messages" {
			handleMessages(w, r, store, queueName)
			return
		}

		// Route: /{account}/{queue}?comp=metadata
		if comp == "metadata" {
			handleQueueMetadata(w, r, store, queueName)
			return
		}

		// Route: /{account}/{queue}
		handleQueue(w, r, store, queueName)
	})

	return AuthMiddleware(mux)
}

func handleQueue(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	switch r.Method {
	case http.MethodPut:
		createQueue(w, r, store, queueName)
	case http.MethodDelete:
		deleteQueue(w, r, store, queueName)
	default:
		writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "The HTTP method is not allowed.")
	}
}

func handleQueueMetadata(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		getQueueMetadata(w, r, store, queueName)
	case http.MethodPut:
		setQueueMetadata(w, r, store, queueName)
	default:
		writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "The HTTP method is not allowed.")
	}
}

func handleMessages(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	switch r.Method {
	case http.MethodPost:
		putMessage(w, r, store, queueName)
	case http.MethodGet:
		if r.URL.Query().Get("peekonly") == "true" {
			peekMessages(w, r, store, queueName)
		} else {
			getMessages(w, r, store, queueName)
		}
	case http.MethodDelete:
		clearMessages(w, r, store, queueName)
	default:
		writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "The HTTP method is not allowed.")
	}
}

func handleMessageByID(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName, messageID string) {
	switch r.Method {
	case http.MethodDelete:
		deleteMessage(w, r, store, queueName, messageID)
	case http.MethodPut:
		updateMessage(w, r, store, queueName, messageID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "The HTTP method is not allowed.")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	resp := ErrorResponse{Code: code, Message: message}
	data, _ := xml.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(w, "%s%s", xml.Header, string(data))
}

func writeXML(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	data, _ := xml.MarshalIndent(v, "", "  ")
	fmt.Fprintf(w, "%s%s", xml.Header, string(data))
}

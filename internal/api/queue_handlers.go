package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"azure-storage-lite/internal/queue"
)

func createQueue(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	metadata := extractMetadata(r)
	created, conflict := store.CreateQueue(queueName, metadata)

	if conflict {
		writeError(w, http.StatusConflict, "QueueAlreadyExists",
			"The specified queue already exists with different metadata.")
		return
	}

	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteQueue(w http.ResponseWriter, _ *http.Request, store *queue.Store, queueName string) {
	if !store.DeleteQueue(queueName) {
		writeError(w, http.StatusNotFound, "QueueNotFound", "The specified queue does not exist.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func getQueueMetadata(w http.ResponseWriter, _ *http.Request, store *queue.Store, queueName string) {
	metadata, count, ok := store.GetMetadata(queueName)
	if !ok {
		writeError(w, http.StatusNotFound, "QueueNotFound", "The specified queue does not exist.")
		return
	}

	for k, v := range metadata {
		w.Header().Set("x-ms-meta-"+k, v)
	}
	w.Header().Set("x-ms-approximate-messages-count", fmt.Sprintf("%d", count))
	w.WriteHeader(http.StatusOK)
}

func setQueueMetadata(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	metadata := extractMetadata(r)
	if !store.SetMetadata(queueName, metadata) {
		writeError(w, http.StatusNotFound, "QueueNotFound", "The specified queue does not exist.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func extractMetadata(r *http.Request) map[string]string {
	metadata := make(map[string]string)
	for key, values := range r.Header {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "x-ms-meta-") {
			metaKey := strings.TrimPrefix(lower, "x-ms-meta-")
			if len(values) > 0 {
				metadata[metaKey] = values[0]
			}
		}
	}
	return metadata
}

func listQueues(w http.ResponseWriter, r *http.Request, store *queue.Store) {
	prefix := r.URL.Query().Get("prefix")
	marker := r.URL.Query().Get("marker")
	maxResults := parseIntParam(r, "maxresults", 5000)
	includeMetadata := r.URL.Query().Get("include") == "metadata"

	allQueues := store.ListQueuesWithDetails(prefix, includeMetadata)

	// Apply marker: skip queues until we pass the marker
	startIdx := 0
	if marker != "" {
		for i, q := range allQueues {
			if q.Name >= marker {
				startIdx = i
				break
			}
		}
	}

	// Paginate
	endIdx := startIdx + maxResults
	if endIdx > len(allQueues) {
		endIdx = len(allQueues)
	}
	page := allQueues[startIdx:endIdx]

	nextMarker := ""
	if endIdx < len(allQueues) {
		nextMarker = allQueues[endIdx].Name
	}

	// Build response
	resp := EnumerationResults{
		ServiceEndpoint: fmt.Sprintf("http://%s/devstoreaccount1/", r.Host),
		NextMarker:      nextMarker,
	}
	if prefix != "" {
		resp.Prefix = prefix
	}
	if marker != "" {
		resp.Marker = marker
	}
	if r.URL.Query().Get("maxresults") != "" {
		resp.MaxResults = maxResults
	}

	for _, q := range page {
		eq := EnumerationQueue{Name: q.Name}
		if includeMetadata && len(q.Metadata) > 0 {
			meta := &EnumerationMeta{}
			keys := make([]string, 0, len(q.Metadata))
			for k := range q.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				meta.Items = append(meta.Items, MetadataItem{
					XMLName: xml.Name{Local: k},
					Value:   q.Metadata[k],
				})
			}
			eq.Metadata = meta
		}
		resp.Queues.Queues = append(resp.Queues.Queues, eq)
	}

	writeXML(w, http.StatusOK, resp)
}

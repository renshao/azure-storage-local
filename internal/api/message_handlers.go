package api

import (
	"encoding/xml"
	"io"
	"net/http"
	"strconv"

	"github.com/azure-storage-local/internal/queue"
)

func putMessage(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "InvalidInput", "Could not read request body.")
		return
	}
	defer r.Body.Close()

	var req PutMessageRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "InvalidXmlDocument", "The XML request body is invalid.")
		return
	}

	visibilityTimeout := parseIntParam(r, "visibilitytimeout", 0)
	ttl := parseIntParam(r, "messagettl", 0) // 0 means default (7 days) in store

	msg, err := store.PutMessage(queueName, req.MessageText, visibilityTimeout, ttl)
	if err != nil {
		writeError(w, http.StatusNotFound, "QueueNotFound", "The specified queue does not exist.")
		return
	}

	resp := QueueMessagesList{
		Messages: []QueueMessageResponse{
			{
				MessageId:       msg.ID,
				InsertionTime:   FormatRFC1123(msg.InsertionTime),
				ExpirationTime:  FormatRFC1123(msg.ExpirationTime),
				PopReceipt:      msg.PopReceipt,
				TimeNextVisible: FormatRFC1123(msg.TimeNextVisible),
			},
		},
	}
	writeXML(w, http.StatusCreated, resp)
}

func getMessages(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	numMessages := parseIntParam(r, "numofmessages", 1)
	if numMessages < 1 || numMessages > 32 {
		writeError(w, http.StatusBadRequest, "OutOfRangeQueryParameterValue",
			"numofmessages must be between 1 and 32.")
		return
	}

	visibilityTimeout := parseIntParam(r, "visibilitytimeout", 30)
	if visibilityTimeout < 1 || visibilityTimeout > 7*24*3600 {
		writeError(w, http.StatusBadRequest, "OutOfRangeQueryParameterValue",
			"visibilitytimeout must be between 1 and 604800.")
		return
	}

	msgs, err := store.GetMessages(queueName, numMessages, visibilityTimeout)
	if err != nil {
		writeError(w, http.StatusNotFound, "QueueNotFound", "The specified queue does not exist.")
		return
	}

	resp := QueueMessagesList{}
	for _, msg := range msgs {
		resp.Messages = append(resp.Messages, QueueMessageResponse{
			MessageId:       msg.ID,
			InsertionTime:   FormatRFC1123(msg.InsertionTime),
			ExpirationTime:  FormatRFC1123(msg.ExpirationTime),
			PopReceipt:      msg.PopReceipt,
			TimeNextVisible: FormatRFC1123(msg.TimeNextVisible),
			DequeueCount:    msg.DequeueCount,
			MessageText:     msg.Text,
		})
	}
	writeXML(w, http.StatusOK, resp)
}

func peekMessages(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName string) {
	numMessages := parseIntParam(r, "numofmessages", 1)
	if numMessages < 1 || numMessages > 32 {
		writeError(w, http.StatusBadRequest, "OutOfRangeQueryParameterValue",
			"numofmessages must be between 1 and 32.")
		return
	}

	msgs, err := store.PeekMessages(queueName, numMessages)
	if err != nil {
		writeError(w, http.StatusNotFound, "QueueNotFound", "The specified queue does not exist.")
		return
	}

	resp := QueueMessagesList{}
	for _, msg := range msgs {
		resp.Messages = append(resp.Messages, QueueMessageResponse{
			MessageId:      msg.ID,
			InsertionTime:  FormatRFC1123(msg.InsertionTime),
			ExpirationTime: FormatRFC1123(msg.ExpirationTime),
			DequeueCount:   msg.DequeueCount,
			MessageText:    msg.Text,
		})
	}
	writeXML(w, http.StatusOK, resp)
}

func deleteMessage(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName, messageID string) {
	popReceipt := r.URL.Query().Get("popreceipt")
	if popReceipt == "" {
		writeError(w, http.StatusBadRequest, "MissingRequiredQueryParameter",
			"popreceipt query parameter is required.")
		return
	}

	err := store.DeleteMessage(queueName, messageID, popReceipt)
	if err != nil {
		if err.Error() == "pop receipt mismatch" {
			writeError(w, http.StatusBadRequest, "PopReceiptMismatch",
				"The specified pop receipt did not match the pop receipt for a dequeued message.")
			return
		}
		writeError(w, http.StatusNotFound, "MessageNotFound", "The specified message does not exist.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func clearMessages(w http.ResponseWriter, _ *http.Request, store *queue.Store, queueName string) {
	err := store.ClearMessages(queueName)
	if err != nil {
		writeError(w, http.StatusNotFound, "QueueNotFound", "The specified queue does not exist.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func updateMessage(w http.ResponseWriter, r *http.Request, store *queue.Store, queueName, messageID string) {
	popReceipt := r.URL.Query().Get("popreceipt")
	if popReceipt == "" {
		writeError(w, http.StatusBadRequest, "MissingRequiredQueryParameter",
			"popreceipt query parameter is required.")
		return
	}

	visibilityTimeoutStr := r.URL.Query().Get("visibilitytimeout")
	if visibilityTimeoutStr == "" {
		writeError(w, http.StatusBadRequest, "MissingRequiredQueryParameter",
			"visibilitytimeout query parameter is required.")
		return
	}
	visibilityTimeout := parseIntParam(r, "visibilitytimeout", 0)

	var newText *string
	body, err := io.ReadAll(r.Body)
	if err == nil && len(body) > 0 {
		var req PutMessageRequest
		if err := xml.Unmarshal(body, &req); err == nil {
			newText = &req.MessageText
		}
	}

	msg, err := store.UpdateMessage(queueName, messageID, popReceipt, visibilityTimeout, newText)
	if err != nil {
		if err.Error() == "pop receipt mismatch" {
			writeError(w, http.StatusBadRequest, "PopReceiptMismatch",
				"The specified pop receipt did not match the pop receipt for a dequeued message.")
			return
		}
		writeError(w, http.StatusNotFound, "MessageNotFound", "The specified message does not exist.")
		return
	}

	w.Header().Set("x-ms-popreceipt", msg.PopReceipt)
	w.Header().Set("x-ms-time-next-visible", FormatRFC1123(msg.TimeNextVisible))
	w.WriteHeader(http.StatusNoContent)
}

func parseIntParam(r *http.Request, name string, defaultVal int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

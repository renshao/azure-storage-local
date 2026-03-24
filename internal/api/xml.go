package api

import (
	"encoding/xml"
	"time"
)

// PutMessageRequest is the XML body for Put Message.
type PutMessageRequest struct {
	XMLName     xml.Name `xml:"QueueMessage"`
	MessageText string   `xml:"MessageText"`
}

// QueueMessageResponse represents a single message in XML responses.
type QueueMessageResponse struct {
	XMLName         xml.Name `xml:"QueueMessage"`
	MessageId       string   `xml:"MessageId"`
	InsertionTime   string   `xml:"InsertionTime"`
	ExpirationTime  string   `xml:"ExpirationTime"`
	PopReceipt      string   `xml:"PopReceipt,omitempty"`
	TimeNextVisible string   `xml:"TimeNextVisible,omitempty"`
	DequeueCount    int      `xml:"DequeueCount"`
	MessageText     string   `xml:"MessageText,omitempty"`
}

// QueueMessagesList is the XML wrapper for message list responses.
type QueueMessagesList struct {
	XMLName  xml.Name               `xml:"QueueMessagesList"`
	Messages []QueueMessageResponse `xml:"QueueMessage"`
}

// ErrorResponse is the XML error body.
type ErrorResponse struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

// EnumerationResults is the XML response for List Queues.
type EnumerationResults struct {
	XMLName         xml.Name          `xml:"EnumerationResults"`
	ServiceEndpoint string            `xml:"ServiceEndpoint,attr"`
	Prefix          string            `xml:"Prefix,omitempty"`
	Marker          string            `xml:"Marker,omitempty"`
	MaxResults      int               `xml:"MaxResults,omitempty"`
	Queues          EnumerationQueues `xml:"Queues"`
	NextMarker      string            `xml:"NextMarker"`
}

// EnumerationQueues wraps the list of queues.
type EnumerationQueues struct {
	Queues []EnumerationQueue `xml:"Queue"`
}

// EnumerationQueue represents a single queue in List Queues response.
type EnumerationQueue struct {
	Name     string              `xml:"Name"`
	Metadata *EnumerationMeta    `xml:"Metadata,omitempty"`
}

// EnumerationMeta holds queue metadata as XML elements.
type EnumerationMeta struct {
	Items []MetadataItem
}

// MetadataItem is a single metadata key-value pair.
type MetadataItem struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// MarshalXML custom-marshals metadata as dynamic element names.
func (m EnumerationMeta) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	e.EncodeToken(start)
	for _, item := range m.Items {
		e.EncodeElement(item.Value, xml.StartElement{Name: item.XMLName})
	}
	e.EncodeToken(start.End())
	return nil
}

// FormatRFC1123 formats a time as RFC 1123 with "GMT" suffix (Azure format).
func FormatRFC1123(t time.Time) string {
	return t.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
}

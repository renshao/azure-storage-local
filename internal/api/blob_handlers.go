package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"azure-storage-lite/internal/blob"
)

// BlobRouter creates the HTTP handler for the Azure Blob Storage API.
func BlobRouter(store *blob.Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 3)

		if len(parts) < 1 || parts[0] == "" {
			writeError(w, http.StatusBadRequest, "InvalidUri", "The request URI is invalid.")
			return
		}

		account := parts[0]
		if account != AccountName {
			writeError(w, http.StatusForbidden, "AuthenticationFailed", "Account not found.")
			return
		}

		comp := r.URL.Query().Get("comp")
		restype := r.URL.Query().Get("restype")

		// Route: /{account}?comp=list — List Containers
		if (len(parts) < 2 || parts[1] == "") && comp == "list" {
			if r.Method == http.MethodGet {
				listContainers(w, r, store)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "Method not allowed.")
			}
			return
		}

		// Route: /{account}/?restype=service&comp=userdelegationkey — Get User Delegation Key
		if (len(parts) < 2 || parts[1] == "") && restype == "service" && comp == "userdelegationkey" {
			if r.Method == http.MethodPost {
				getUserDelegationKey(w, r)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "Method not allowed.")
			}
			return
		}

		if len(parts) < 2 || parts[1] == "" {
			writeError(w, http.StatusBadRequest, "InvalidUri", "Container name is required.")
			return
		}

		containerName := parts[1]

		// Route: /{account}/{container}?restype=container&comp=list — List Blobs
		if restype == "container" && comp == "list" {
			if r.Method == http.MethodGet {
				listBlobs(w, r, store, containerName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "Method not allowed.")
			}
			return
		}

		// Route: /{account}/{container}?restype=container — Create/Delete Container
		if restype == "container" {
			switch r.Method {
			case http.MethodPut:
				createContainer(w, r, store, containerName)
			case http.MethodDelete:
				deleteContainer(w, r, store, containerName)
			default:
				writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "Method not allowed.")
			}
			return
		}

		// Blob operations require /{account}/{container}/{blob...}
		if len(parts) < 3 || parts[2] == "" {
			writeError(w, http.StatusBadRequest, "InvalidUri", "Blob name is required.")
			return
		}
		blobName := parts[2]

		// Route: /{account}/{container}/{blob}?comp=metadata — Get Blob Metadata
		if comp == "metadata" {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				getBlobMetadata(w, r, store, containerName, blobName)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "Method not allowed.")
			}
			return
		}

		// Route: /{account}/{container}/{blob} — Put/Get Blob
		switch r.Method {
		case http.MethodPut:
			putBlob(w, r, store, containerName, blobName)
		case http.MethodGet:
			getBlob(w, r, store, containerName, blobName)
		case http.MethodHead:
			getBlobProperties(w, r, store, containerName, blobName)
		default:
			writeError(w, http.StatusMethodNotAllowed, "UnsupportedHttpVerb", "Method not allowed.")
		}
	})

	return AuthMiddleware(mux)
}

// --- Container Handlers ---

func createContainer(w http.ResponseWriter, r *http.Request, store *blob.Store, containerName string) {
	metadata := extractMetadata(r)
	created, conflict := store.CreateContainer(containerName, metadata)

	if conflict {
		writeError(w, http.StatusConflict, "ContainerAlreadyExists",
			"The specified container already exists.")
		return
	}
	if created {
		w.Header().Set("ETag", fmt.Sprintf("\"%x\"", 0))
		w.Header().Set("Last-Modified", FormatRFC1123(timeNow()))
		w.WriteHeader(http.StatusCreated)
	}
}

func deleteContainer(w http.ResponseWriter, _ *http.Request, store *blob.Store, containerName string) {
	if !store.DeleteContainer(containerName) {
		writeError(w, http.StatusNotFound, "ContainerNotFound",
			"The specified container does not exist.")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// --- Blob XML types ---

type blobEnumerationResults struct {
	XMLName         xml.Name            `xml:"EnumerationResults"`
	ServiceEndpoint string              `xml:"ServiceEndpoint,attr"`
	ContainerName   string              `xml:"ContainerName,attr"`
	Prefix          string              `xml:"Prefix,omitempty"`
	Marker          string              `xml:"Marker,omitempty"`
	MaxResults      int                 `xml:"MaxResults,omitempty"`
	Delimiter       string              `xml:"Delimiter,omitempty"`
	Blobs           blobEnumBlobs       `xml:"Blobs"`
	NextMarker      string              `xml:"NextMarker"`
}

type blobEnumBlobs struct {
	Blobs      []blobEnumBlob      `xml:"Blob,omitempty"`
	BlobPrefix []blobEnumPrefix    `xml:"BlobPrefix,omitempty"`
}

type blobEnumBlob struct {
	Name       string              `xml:"Name"`
	Properties blobEnumProperties  `xml:"Properties"`
	Metadata   *EnumerationMeta    `xml:"Metadata,omitempty"`
}

type blobEnumProperties struct {
	LastModified  string `xml:"Last-Modified"`
	Etag          string `xml:"Etag"`
	ContentLength int64  `xml:"Content-Length"`
	ContentType   string `xml:"Content-Type"`
	BlobType      string `xml:"BlobType"`
}

type blobEnumPrefix struct {
	Name string `xml:"Name"`
}

// --- Container list XML types ---

type containerEnumerationResults struct {
	XMLName         xml.Name                 `xml:"EnumerationResults"`
	ServiceEndpoint string                   `xml:"ServiceEndpoint,attr"`
	Prefix          string                   `xml:"Prefix,omitempty"`
	Marker          string                   `xml:"Marker,omitempty"`
	MaxResults      int                      `xml:"MaxResults,omitempty"`
	Containers      containerEnumContainers  `xml:"Containers"`
	NextMarker      string                   `xml:"NextMarker"`
}

type containerEnumContainers struct {
	Containers []containerEnumContainer `xml:"Container"`
}

type containerEnumContainer struct {
	Name       string                  `xml:"Name"`
	Properties containerEnumProperties `xml:"Properties"`
	Metadata   *EnumerationMeta        `xml:"Metadata,omitempty"`
}

type containerEnumProperties struct {
	LastModified string `xml:"Last-Modified"`
	Etag         string `xml:"Etag"`
}

// --- List Containers Handler ---

func listContainers(w http.ResponseWriter, r *http.Request, store *blob.Store) {
	prefix := r.URL.Query().Get("prefix")
	marker := r.URL.Query().Get("marker")
	maxResults := parseIntParam(r, "maxresults", 5000)
	includeMetadata := strings.Contains(r.URL.Query().Get("include"), "metadata")

	containers := store.ListContainers(prefix, includeMetadata)

	// Apply marker
	startIdx := 0
	if marker != "" {
		for i, c := range containers {
			if c.Name >= marker {
				startIdx = i
				break
			}
		}
	}

	endIdx := startIdx + maxResults
	if endIdx > len(containers) {
		endIdx = len(containers)
	}
	page := containers[startIdx:endIdx]

	nextMarker := ""
	if endIdx < len(containers) {
		nextMarker = containers[endIdx].Name
	}

	resp := containerEnumerationResults{
		ServiceEndpoint: fmt.Sprintf("http://%s/devstoreaccount1", r.Host),
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

	for _, c := range page {
		ec := containerEnumContainer{
			Name: c.Name,
			Properties: containerEnumProperties{
				LastModified: FormatRFC1123(c.LastModified),
				Etag:         c.ETag,
			},
		}
		if includeMetadata && len(c.Metadata) > 0 {
			meta := buildEnumerationMeta(c.Metadata)
			ec.Metadata = meta
		}
		resp.Containers.Containers = append(resp.Containers.Containers, ec)
	}

	writeXML(w, http.StatusOK, resp)
}

// --- List Blobs Handler ---

func listBlobs(w http.ResponseWriter, r *http.Request, store *blob.Store, containerName string) {
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	marker := r.URL.Query().Get("marker")
	maxResults := parseIntParam(r, "maxresults", 5000)
	includeMetadata := strings.Contains(r.URL.Query().Get("include"), "metadata")

	blobs, prefixes, err := store.ListBlobs(containerName, prefix, delimiter, includeMetadata)
	if err != nil {
		writeError(w, http.StatusNotFound, "ContainerNotFound",
			"The specified container does not exist.")
		return
	}

	// Apply marker
	startIdx := 0
	if marker != "" {
		for i, b := range blobs {
			if b.Name >= marker {
				startIdx = i
				break
			}
		}
	}

	endIdx := startIdx + maxResults
	if endIdx > len(blobs) {
		endIdx = len(blobs)
	}
	page := blobs[startIdx:endIdx]

	nextMarker := ""
	if endIdx < len(blobs) {
		nextMarker = blobs[endIdx].Name
	}

	resp := blobEnumerationResults{
		ServiceEndpoint: fmt.Sprintf("http://%s/devstoreaccount1", r.Host),
		ContainerName:   containerName,
		NextMarker:      nextMarker,
	}
	if prefix != "" {
		resp.Prefix = prefix
	}
	if marker != "" {
		resp.Marker = marker
	}
	if delimiter != "" {
		resp.Delimiter = delimiter
	}
	if r.URL.Query().Get("maxresults") != "" {
		resp.MaxResults = maxResults
	}

	for _, b := range page {
		eb := blobEnumBlob{
			Name: b.Name,
			Properties: blobEnumProperties{
				LastModified:  FormatRFC1123(b.LastModified),
				Etag:          b.ETag,
				ContentLength: b.ContentLen,
				ContentType:   b.ContentType,
				BlobType:      b.BlobType,
			},
		}
		if includeMetadata && len(b.Metadata) > 0 {
			meta := buildEnumerationMeta(b.Metadata)
			eb.Metadata = meta
		}
		resp.Blobs.Blobs = append(resp.Blobs.Blobs, eb)
	}

	for _, p := range prefixes {
		resp.Blobs.BlobPrefix = append(resp.Blobs.BlobPrefix, blobEnumPrefix{Name: p})
	}

	writeXML(w, http.StatusOK, resp)
}

// --- Blob Handlers ---

func putBlob(w http.ResponseWriter, r *http.Request, store *blob.Store, containerName, blobName string) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "InvalidInput", "Could not read request body.")
		return
	}
	defer r.Body.Close()

	contentType := r.Header.Get("x-ms-blob-content-type")
	if contentType == "" {
		contentType = r.Header.Get("Content-Type")
	}
	blobType := r.Header.Get("x-ms-blob-type")
	metadata := extractMetadata(r)

	b, err := store.PutBlob(containerName, blobName, data, contentType, blobType, metadata)
	if err != nil {
		writeError(w, http.StatusNotFound, "ContainerNotFound",
			"The specified container does not exist.")
		return
	}

	w.Header().Set("ETag", b.ETag)
	w.Header().Set("Last-Modified", FormatRFC1123(b.LastModified))
	w.Header().Set("Content-MD5", "")
	w.WriteHeader(http.StatusCreated)
}

func getBlob(w http.ResponseWriter, r *http.Request, store *blob.Store, containerName, blobName string) {
	b, err := store.GetBlob(containerName, blobName)
	if err != nil {
		if strings.Contains(err.Error(), "container not found") {
			writeError(w, http.StatusNotFound, "ContainerNotFound",
				"The specified container does not exist.")
		} else {
			writeError(w, http.StatusNotFound, "BlobNotFound",
				"The specified blob does not exist.")
		}
		return
	}

	w.Header().Set("Content-Type", b.ContentType)
	w.Header().Set("ETag", b.ETag)
	w.Header().Set("Last-Modified", FormatRFC1123(b.LastModified))
	w.Header().Set("x-ms-blob-type", b.BlobType)
	w.Header().Set("x-ms-creation-time", FormatRFC1123(b.CreatedAt))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.Data)))

	for k, v := range b.Metadata {
		w.Header().Set("x-ms-meta-"+k, v)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b.Data)
}

func getBlobProperties(w http.ResponseWriter, _ *http.Request, store *blob.Store, containerName, blobName string) {
	b, err := store.GetBlob(containerName, blobName)
	if err != nil {
		if strings.Contains(err.Error(), "container not found") {
			writeError(w, http.StatusNotFound, "ContainerNotFound",
				"The specified container does not exist.")
		} else {
			writeError(w, http.StatusNotFound, "BlobNotFound",
				"The specified blob does not exist.")
		}
		return
	}

	w.Header().Set("Content-Type", b.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.Data)))
	w.Header().Set("ETag", b.ETag)
	w.Header().Set("Last-Modified", FormatRFC1123(b.LastModified))
	w.Header().Set("x-ms-blob-type", b.BlobType)
	w.Header().Set("x-ms-creation-time", FormatRFC1123(b.CreatedAt))

	for k, v := range b.Metadata {
		w.Header().Set("x-ms-meta-"+k, v)
	}

	w.WriteHeader(http.StatusOK)
}

func getBlobMetadata(w http.ResponseWriter, _ *http.Request, store *blob.Store, containerName, blobName string) {
	meta, b, err := store.GetBlobMetadata(containerName, blobName)
	if err != nil {
		if strings.Contains(err.Error(), "container not found") {
			writeError(w, http.StatusNotFound, "ContainerNotFound",
				"The specified container does not exist.")
		} else {
			writeError(w, http.StatusNotFound, "BlobNotFound",
				"The specified blob does not exist.")
		}
		return
	}

	for k, v := range meta {
		w.Header().Set("x-ms-meta-"+k, v)
	}
	w.Header().Set("ETag", b.ETag)
	w.Header().Set("Last-Modified", FormatRFC1123(b.LastModified))
	w.WriteHeader(http.StatusOK)
}

// --- Helpers ---

func buildEnumerationMeta(metadata map[string]string) *EnumerationMeta {
	meta := &EnumerationMeta{}
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		meta.Items = append(meta.Items, MetadataItem{
			XMLName: xml.Name{Local: k},
			Value:   metadata[k],
		})
	}
	return meta
}

func timeNow() time.Time {
	return time.Now().UTC()
}

// --- User Delegation Key ---

// keyInfoRequest is the XML request body for Get User Delegation Key.
type keyInfoRequest struct {
	XMLName          xml.Name `xml:"KeyInfo"`
	Start            string   `xml:"Start"`
	Expiry           string   `xml:"Expiry"`
	DelegatedUserTid string   `xml:"DelegatedUserTid,omitempty"`
}

// userDelegationKeyResponse is the XML response for Get User Delegation Key.
type userDelegationKeyResponse struct {
	XMLName              xml.Name `xml:"UserDelegationKey"`
	SignedOid            string   `xml:"SignedOid"`
	SignedTid            string   `xml:"SignedTid"`
	SignedStart          string   `xml:"SignedStart"`
	SignedExpiry         string   `xml:"SignedExpiry"`
	SignedService        string   `xml:"SignedService"`
	SignedVersion        string   `xml:"SignedVersion"`
	SignedDelegatedUserTid string `xml:"SignedDelegatedUserTid,omitempty"`
	Value                string   `xml:"Value"`
}

// Stable fake GUIDs for the emulator's "user identity"
const (
	emulatorSignedOid = "00000000-0000-0000-0000-000000000001"
	emulatorSignedTid = "00000000-0000-0000-0000-000000000002"
)

func getUserDelegationKey(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "InvalidInput", "Could not read request body.")
		return
	}
	defer r.Body.Close()

	var req keyInfoRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "InvalidXmlDocument", "The XML request body is invalid.")
		return
	}

	if req.Start == "" || req.Expiry == "" {
		writeError(w, http.StatusBadRequest, "InvalidInput", "Start and Expiry are required.")
		return
	}

	// Generate deterministic key: HMAC-SHA256(accountKey, start|expiry)
	accountKeyBytes, _ := base64.StdEncoding.DecodeString(AccountKey)
	mac := hmac.New(sha256.New, accountKeyBytes)
	mac.Write([]byte(req.Start + "|" + req.Expiry))
	keyValue := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	resp := userDelegationKeyResponse{
		SignedOid:     emulatorSignedOid,
		SignedTid:     emulatorSignedTid,
		SignedStart:   req.Start,
		SignedExpiry:  req.Expiry,
		SignedService: "b",
		SignedVersion: APIVersion,
		Value:         keyValue,
	}
	if req.DelegatedUserTid != "" {
		resp.SignedDelegatedUserTid = req.DelegatedUserTid
	}

	writeXML(w, http.StatusOK, resp)
}

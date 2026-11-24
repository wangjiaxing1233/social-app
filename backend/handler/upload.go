package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"networkai/service"

	"github.com/pborman/uuid"
)

// maxUploadBytes caps the size of a direct upload (100 MB).
const maxUploadBytes = 100 << 20

// uploadURLHandler starts a direct upload. Request body:
//
//	{"filename": "cat.jpg", "content_type": "image/jpeg"}
//
// The response has a signed URL. The client PUTs the file there with the
// returned Content-Type header, then calls /posts/complete.
func uploadURLHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one upload URL request")

	var body struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)).Decode(&body); err != nil {
		http.Error(w, "Cannot decode upload request from client", http.StatusBadRequest)
		return
	}

	mediaType, ok := mediaTypes[strings.ToLower(filepath.Ext(body.Filename))]
	if !ok {
		http.Error(w, "Unsupported file type", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(body.ContentType, mediaType+"/") {
		http.Error(w, "content_type must match the file type, for example image/jpeg for a .jpg file", http.StatusBadRequest)
		return
	}

	ticket, err := service.CreateUploadURL(usernameFromRequest(r), body.ContentType)
	if err != nil {
		http.Error(w, "Failed to create upload URL", http.StatusInternalServerError)
		fmt.Printf("Failed to create upload URL %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

// completeUploadHandler creates the post once the file is in the bucket.
// Request body:
//
//	{"post_id": "<post_id from /posts/upload-url>", "message": "my caption"}
func completeUploadHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one complete upload request")

	var body struct {
		PostID  string `json:"post_id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)).Decode(&body); err != nil {
		http.Error(w, "Cannot decode complete request from client", http.StatusBadRequest)
		return
	}
	if uuid.Parse(body.PostID) == nil {
		http.Error(w, "post_id is not valid", http.StatusBadRequest)
		return
	}

	post, err := service.CompleteUpload(usernameFromRequest(r), body.PostID, body.Message, maxUploadBytes)
	switch {
	case errors.Is(err, service.ErrUploadNotFound):
		http.Error(w, "Uploaded file not found. PUT the file to upload_url first", http.StatusNotFound)
		return
	case errors.Is(err, service.ErrUploadTooLarge):
		http.Error(w, "Uploaded file is too large (limit 100 MB)", http.StatusRequestEntityTooLarge)
		return
	case errors.Is(err, service.ErrUploadAlreadyCompleted):
		http.Error(w, "This upload was already completed", http.StatusConflict)
		return
	case err != nil:
		http.Error(w, "Failed to save post to backend", http.StatusInternalServerError)
		fmt.Printf("Failed to complete upload %v\n", err)
		return
	}

	fmt.Println("Post is saved successfully.")
	writeJSON(w, http.StatusCreated, post)
}

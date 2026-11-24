package service

import (
	"errors"
	"strings"
	"time"

	"networkai/backend"
	"networkai/constants"
	"networkai/model"

	"github.com/pborman/uuid"
)

// UploadURLTTL is how long a signed upload URL stays valid.
const UploadURLTTL = 15 * time.Minute

var (
	ErrUploadNotFound         = errors.New("uploaded file not found")
	ErrUploadTooLarge         = errors.New("uploaded file is too large")
	ErrUploadAlreadyCompleted = errors.New("post already exists")
)

// UploadTicket tells the client where and how to upload the file.
type UploadTicket struct {
	PostID    string            `json:"post_id"`
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt int64             `json:"expires_at"`
}

// uploadObjectName puts the username in the object path, so a user can only
// complete uploads that were started with their own token.
func uploadObjectName(username, postID string) string {
	return "uploads/" + username + "/" + postID
}

// CreateUploadURL starts a direct upload: it picks the post id and signs a
// URL that the client can PUT the file to.
func CreateUploadURL(username, contentType string) (*UploadTicket, error) {
	postID := uuid.New()
	uploadURL, err := backend.GCSBackend.SignedPutURL(uploadObjectName(username, postID), contentType, UploadURLTTL)
	if err != nil {
		return nil, err
	}
	return &UploadTicket{
		PostID:    postID,
		UploadURL: uploadURL,
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: time.Now().Add(UploadURLTTL).Unix(),
	}, nil
}

// CompleteUpload creates the post after the client has uploaded the file to
// the signed URL.
func CompleteUpload(username, postID, message string, maxBytes int64) (*model.Post, error) {
	exists, _, _, err := backend.ESBackend.GetFromES(constants.POST_INDEX, postID, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUploadAlreadyCompleted
	}

	mediaLink, contentType, err := backend.GCSBackend.FinalizeUpload(uploadObjectName(username, postID), maxBytes)
	switch {
	case errors.Is(err, backend.ErrObjectNotFound):
		return nil, ErrUploadNotFound
	case errors.Is(err, backend.ErrObjectTooLarge):
		return nil, ErrUploadTooLarge
	case err != nil:
		return nil, err
	}

	post := &model.Post{
		Id:      postID,
		User:    username,
		Message: message,
		Url:     mediaLink,
		Type:    mediaTypeOf(contentType),
	}
	if err := savePostDocument(post); err != nil {
		return nil, err
	}
	return post, nil
}

func mediaTypeOf(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	default:
		return "unknown"
	}
}

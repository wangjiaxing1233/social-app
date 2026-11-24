package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
)

var (
	ErrObjectNotFound = errors.New("object not found")
	ErrObjectTooLarge = errors.New("object is too large")
)

// SignedPutURL returns a V4 signed URL that lets a client upload one object
// straight to the bucket with an HTTP PUT, without the file passing through
// this server. The client must send exactly this Content-Type.
//
// Without a service account key file, the client library signs through the
// IAM Credentials API, so the service account this server runs as needs the
// iam.serviceAccounts.signBlob permission on itself
// (roles/iam.serviceAccountTokenCreator).
func (backend *GoogleCloudStorageBackend) SignedPutURL(objectName, contentType string, ttl time.Duration) (string, error) {
	return backend.client.Bucket(backend.bucket).SignedURL(objectName, &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      "PUT",
		ContentType: contentType,
		Expires:     time.Now().Add(ttl),
	})
}

// FinalizeUpload checks an object that a client uploaded with a signed URL,
// deletes it if it is larger than maxBytes, and otherwise makes it publicly
// readable like objects saved by SaveToGCS.
func (backend *GoogleCloudStorageBackend) FinalizeUpload(objectName string, maxBytes int64) (mediaLink, contentType string, err error) {
	ctx := context.Background()
	object := backend.client.Bucket(backend.bucket).Object(objectName)

	attrs, err := object.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return "", "", ErrObjectNotFound
	}
	if err != nil {
		return "", "", err
	}

	if attrs.Size > maxBytes {
		if err := object.Delete(ctx); err != nil {
			fmt.Printf("Failed to delete oversized upload %s: %v\n", objectName, err)
		}
		return "", "", ErrObjectTooLarge
	}

	if err := object.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		return "", "", err
	}

	fmt.Printf("Direct upload is finalized in GCS: %s\n", attrs.MediaLink)
	return attrs.MediaLink, attrs.ContentType, nil
}

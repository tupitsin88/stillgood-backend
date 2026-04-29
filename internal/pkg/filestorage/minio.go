package filestorage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileStorage struct {
	client        *minio.Client
	bucketName    string
	endpoint      string
	useSSL        bool
	publicBaseURL string
}

func NewFileStorage(endpoint, accessKey, secretKey, bucketName string, useSSL bool, publicBaseURL string) (*FileStorage, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return &FileStorage{
		client:        minioClient,
		bucketName:    bucketName,
		endpoint:      endpoint,
		useSSL:        useSSL,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}, nil
}

func (fs *FileStorage) Upload(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	contentType := fileHeader.Header.Get("Content-Type")

	return fs.uploadReader(file, fileHeader.Size, ext, contentType, "")
}

func (fs *FileStorage) UploadBytes(content []byte, ext string, contentType string) (string, error) {
	return fs.UploadBytesWithPrefix(content, ext, contentType, "")
}

func (fs *FileStorage) UploadBytesWithPrefix(content []byte, ext string, contentType string, prefix string) (string, error) {
	reader := bytes.NewReader(content)
	return fs.uploadReader(reader, int64(len(content)), ext, contentType, prefix)
}

func (fs *FileStorage) uploadReader(reader io.Reader, size int64, ext string, contentType string, prefix string) (string, error) {
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	if cleanPrefix := strings.Trim(prefix, "/"); cleanPrefix != "" {
		filename = cleanPrefix + "/" + filename
	}

	_, err := fs.client.PutObject(context.Background(), fs.bucketName, filename, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}

	return fs.publicURL(filename), nil
}

func (fs *FileStorage) publicURL(filename string) string {
	if fs.publicBaseURL != "" {
		return fmt.Sprintf("%s/%s/%s", fs.publicBaseURL, fs.bucketName, filename)
	}

	protocol := "http"
	if fs.useSSL {
		protocol = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", protocol, fs.endpoint, fs.bucketName, filename)
}

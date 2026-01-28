package filestorage

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileStorage struct {
	client     *minio.Client
	bucketName string
	endpoint   string
	useSSL     bool
}

func NewFileStorage(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*FileStorage, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return &FileStorage{
		client:     minioClient,
		bucketName: bucketName,
		endpoint:   endpoint,
		useSSL:     useSSL,
	}, nil
}

func (fs *FileStorage) Upload(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	ext := filepath.Ext(fileHeader.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	contentType := fileHeader.Header.Get("Content-Type")

	_, err = fs.client.PutObject(context.Background(), fs.bucketName, filename, file, fileHeader.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}

	protocol := "http"
	if fs.useSSL {
		protocol = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", protocol, fs.endpoint, fs.bucketName, filename), nil
}

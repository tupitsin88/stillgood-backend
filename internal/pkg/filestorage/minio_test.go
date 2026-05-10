package filestorage

import "testing"

func TestPublicURLUsesPublicBaseURL(t *testing.T) {
	storage := &FileStorage{
		bucketName:    "food-images",
		endpoint:      "minio:9000",
		publicBaseURL: "https://files.stillgood.tech",
	}

	got := storage.publicURL("restaurants/logo.png")
	want := "https://files.stillgood.tech/food-images/restaurants/logo.png"
	if got != want {
		t.Fatalf("publicURL() = %q, want %q", got, want)
	}
}

func TestPublicURLFallsBackToEndpoint(t *testing.T) {
	storage := &FileStorage{
		bucketName: "food-images",
		endpoint:   "minio:9000",
		useSSL:     true,
	}

	got := storage.publicURL("restaurants/cover.jpg")
	want := "https://minio:9000/food-images/restaurants/cover.jpg"
	if got != want {
		t.Fatalf("publicURL() = %q, want %q", got, want)
	}
}

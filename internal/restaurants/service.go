package restaurants

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gen2brain/heic"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/filestorage"
)

type Service interface {
	GetList(params ListParams) ([]domain.Restaurant, int64, error)
	GetByID(id string) (*domain.Restaurant, error)
	CreateRestaurant(partnerID string, req CreateRestaurantRequest) (*domain.Restaurant, error)
	GetPartnerRestaurant(partnerID string) (*domain.Restaurant, error)
	UpdatePartnerRestaurant(partnerID string, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error)
	GetOfferMetaByRestaurantIDs(restaurantIDs []uuid.UUID) (map[uuid.UUID]OfferMeta, error)
	IsApprovedPartner(userID string) (bool, error)
	UploadImage(file *multipart.FileHeader) (string, error)
}

type ListParams struct {
	Lat        *float64
	Lng        *float64
	Radius     *int
	CategoryID *uuid.UUID
	Limit      int
	Offset     int
}

var ErrInvalidImageFormat = errors.New("invalid image format")
var ErrImageTooLarge = errors.New("image is too large")
var ErrImageProcessingFailed = errors.New("image processing failed")
var ErrStorageUnavailable = errors.New("file storage is unavailable")
var ErrRestaurantAlreadyExists = errors.New("restaurant already exists")
var ErrPartnerNotApproved = errors.New("partner is not approved")

const (
	maxUploadImageSizeBytes   = 10 << 20 // 10MB
	maxUploadRequestBodyBytes = maxUploadImageSizeBytes + (1 << 20)
	jpegCompressionQuality    = 80
)

type service struct {
	repo        Repository
	fileStorage *filestorage.FileStorage
}

func NewService(repo Repository, fileStorage *filestorage.FileStorage) Service {
	return &service{
		repo:        repo,
		fileStorage: fileStorage,
	}
}

func (s *service) GetList(params ListParams) ([]domain.Restaurant, int64, error) {
	return s.repo.GetList(params)
}

func (s *service) GetByID(id string) (*domain.Restaurant, error) {
	return s.repo.GetByID(id)
}

func (s *service) CreateRestaurant(partnerID string, req CreateRestaurantRequest) (*domain.Restaurant, error) {
	uid, err := uuid.Parse(strings.TrimSpace(partnerID))
	if err != nil {
		return nil, ErrPartnerNotApproved
	}

	isApprovedPartner, err := s.repo.IsApprovedPartner(uid)
	if err != nil {
		return nil, err
	}
	if !isApprovedPartner {
		return nil, ErrPartnerNotApproved
	}

	if _, err := s.repo.GetByPartnerID(uid); err == nil {
		return nil, ErrRestaurantAlreadyExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	companyName := strings.TrimSpace(req.CompanyName)
	inn := strings.TrimSpace(req.Inn)
	address := strings.TrimSpace(req.Address)
	workingHours := ""
	if req.WorkingHours != nil {
		workingHours = strings.TrimSpace(*req.WorkingHours)
	}

	restaurant := &domain.Restaurant{
		PartnerID:    uid,
		Name:         name,
		CompanyName:  companyName,
		Inn:          inn,
		Address:      address,
		Description:  trimOptionalString(req.Description),
		ImageURL:     trimOptionalString(req.ImageURL),
		Phone:        trimOptionalString(req.Phone),
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		IsActive:     true,
		WorkingHours: workingHours,
	}

	if err := s.repo.CreateForPartner(restaurant); err != nil {
		return nil, err
	}

	return restaurant, nil
}

func (s *service) GetPartnerRestaurant(partnerID string) (*domain.Restaurant, error) {
	uid, err := uuid.Parse(partnerID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByPartnerID(uid)
}

func (s *service) UpdatePartnerRestaurant(partnerID string, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error) {
	uid, err := uuid.Parse(partnerID)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdatePartnerProfile(uid, req)
}

func (s *service) GetOfferMetaByRestaurantIDs(restaurantIDs []uuid.UUID) (map[uuid.UUID]OfferMeta, error) {
	return s.repo.GetOfferMetaByRestaurantIDs(restaurantIDs)
}

func (s *service) IsApprovedPartner(userID string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	return s.repo.IsApprovedPartner(uid)
}

func (s *service) UploadImage(file *multipart.FileHeader) (string, error) {
	if s.fileStorage == nil {
		return "", ErrStorageUnavailable
	}

	if file == nil {
		return "", ErrInvalidImageFormat
	}

	if file.Size == 0 || file.Size > maxUploadImageSizeBytes {
		return "", ErrImageTooLarge
	}

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	content, err := io.ReadAll(io.LimitReader(src, maxUploadImageSizeBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(content)) > maxUploadImageSizeBytes {
		return "", ErrImageTooLarge
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	normalizedExt, _, isHEIC := normalizeImageFormat(ext, content)
	if normalizedExt == "" {
		return "", ErrInvalidImageFormat
	}

	if isHEIC {
		compressed, err := compressHEICToJPEG(content)
		if err != nil {
			return "", ErrImageProcessingFailed
		}
		return s.fileStorage.UploadBytes(compressed, ".jpg", "image/jpeg")
	}

	if normalizedExt == ".png" {
		processed, outExt, outContentType, err := processPNG(content)
		if err != nil {
			return "", ErrImageProcessingFailed
		}
		return s.fileStorage.UploadBytes(processed, outExt, outContentType)
	}

	compressed, err := compressToJPEG(content)
	if err != nil {
		return "", ErrImageProcessingFailed
	}

	return s.fileStorage.UploadBytes(compressed, ".jpg", "image/jpeg")
}

func normalizeImageFormat(ext string, content []byte) (normalizedExt string, contentType string, isHEIC bool) {
	ext = strings.ToLower(ext)
	detectedType := http.DetectContentType(content)
	switch detectedType {
	case "image/jpeg":
		return ".jpg", "image/jpeg", false
	case "image/png":
		return ".png", "image/png", false
	}

	if isHEICContent(content) || strings.Contains(detectedType, "heic") || strings.Contains(detectedType, "heif") || (isHEICExtension(ext) && detectedType == "application/octet-stream") {
		if ext == ".heif" {
			return ".heif", "image/heif", true
		}
		return ".heic", "image/heic", true
	}

	return "", "", false
}

func isHEICExtension(ext string) bool {
	switch ext {
	case ".heic", ".heif":
		return true
	default:
		return false
	}
}

func isHEICContent(content []byte) bool {
	if len(content) < 12 {
		return false
	}

	if string(content[4:8]) != "ftyp" {
		return false
	}

	brand := string(content[8:12])
	switch brand {
	case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
		return true
	default:
		return false
	}
}

func compressToJPEG(content []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	return encodeToJPEG(img)
}

func compressHEICToJPEG(content []byte) ([]byte, error) {
	img, err := heic.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	return encodeToJPEG(img)
}

func processPNG(content []byte) ([]byte, string, string, error) {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", "", err
	}

	if hasTransparency(img) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", "", err
		}
		return buf.Bytes(), ".png", "image/png", nil
	}

	compressed, err := encodeToJPEG(img)
	if err != nil {
		return nil, "", "", err
	}

	return compressed, ".jpg", "image/jpeg", nil
}

func encodeToJPEG(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	background := image.NewRGBA(bounds)
	draw.Draw(background, bounds, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(background, bounds, img, bounds.Min, draw.Over)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, background, &jpeg.Options{Quality: jpegCompressionQuality}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func hasTransparency(img image.Image) bool {
	if opaqueImg, ok := img.(interface{ Opaque() bool }); ok {
		return !opaqueImg.Opaque()
	}

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0xffff {
				return true
			}
		}
	}

	return false
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

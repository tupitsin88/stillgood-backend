package offers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/filestorage"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gen2brain/heic"
	"github.com/google/uuid"
)

const (
	maxOfferImageSizeBytes = 10 << 20 // 10MB
	jpegQuality            = 80
	offerImagePrefix       = "offers/images"
)

var (
	ErrOfferFileStorageUnavailable = errors.New("file storage is unavailable")
	ErrOfferInvalidImageFormat     = errors.New("invalid image format")
	ErrOfferImageTooLarge          = errors.New("image is too large")
	ErrOfferUnsupportedImageFormat = errors.New("unsupported image format")
	ErrOfferImageProcessingFailed  = errors.New("image processing failed")
)

type Repository interface {
	GetPartnerByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetRestaurantByPartnerID(ctx context.Context, partnerID uuid.UUID) (*domain.Restaurant, error)
	Create(ctx context.Context, offer *domain.Offer) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Offer, error)
	Update(ctx context.Context, offer *domain.Offer) error
	GetPartnerOffers(ctx context.Context, restaurantID uuid.UUID, limit, offset int) ([]domain.Offer, int64, error)
	GetPublicOffers(ctx context.Context, params FilterParams) ([]domain.Offer, int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type OfferService struct {
	repo        Repository
	fileStorage *filestorage.FileStorage
}

func NewOfferService(repo Repository, fileStorage *filestorage.FileStorage) *OfferService {
	return &OfferService{
		repo:        repo,
		fileStorage: fileStorage,
	}
}

func (s *OfferService) CreateOffer(ctx context.Context, partnerID uuid.UUID, req CreateOfferRequest) (*domain.Offer, error) {
	partner, err := s.repo.GetPartnerByID(ctx, partnerID)
	if err != nil || partner.PartnerStatus != "APPROVED" {
		return nil, fmt.Errorf("PARTNER_NOT_APPROVED")
	}
	restaurant, err := s.repo.GetRestaurantByPartnerID(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("RESTAURANT_NOT_FOUND")
	}
	if req.Price > req.OriginalPrice {
		return nil, fmt.Errorf("INVALID_PRICE")
	}
	if req.PickupEnd.Before(req.PickupStart) {
		return nil, fmt.Errorf("INVALID_TIME_RANGE")
	}
	if req.PickupStart.Before(time.Now().Add(-5 * time.Minute)) {
		return nil, fmt.Errorf("START_TIME_IN_PAST")
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("INVALID_QUANTITY")
	}

	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("INVALID_CATEGORY_ID")
	}
	imgUrl := req.ImageURL
	offer := &domain.Offer{
		RestaurantID:      restaurant.ID,
		Title:             req.Title,
		Description:       req.Description,
		CategoryID:        catID,
		Price:             req.Price,
		OriginalPrice:     req.OriginalPrice,
		QuantityAvailable: req.Quantity,
		QuantityTotal:     req.Quantity,
		PickupStart:       req.PickupStart,
		PickupEnd:         req.PickupEnd,
		ImageURL:          &imgUrl,
		IsActive:          true,
		CreatedAt:         time.Now(),
	}

	if err := s.repo.Create(ctx, offer); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, offer.ID)
}

func (s *OfferService) UpdateOffer(ctx context.Context, id, partnerID uuid.UUID, req UpdateOfferRequest) (*domain.Offer, error) {
	offer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("OFFER_NOT_FOUND")
	}

	if offer.Restaurant.PartnerID != partnerID {
		return nil, fmt.Errorf("FORBIDDEN")
	}

	if req.Title != nil {
		offer.Title = *req.Title
	}
	if req.Description != nil {
		offer.Description = *req.Description
	}
	if req.Price != nil {
		if *req.Price < 0 {
			return nil, fmt.Errorf("PRICE_MUST_BE_POSITIVE")
		}
		if *req.Price > offer.OriginalPrice {
			return nil, fmt.Errorf("PRICE_TOO_HIGH")
		}
		offer.Price = *req.Price
	}
	if req.OriginalPrice != nil {
		offer.OriginalPrice = *req.OriginalPrice
	}
	if req.Quantity != nil {
		if *req.Quantity <= 0 {
			return nil, fmt.Errorf("INVALID_QUANTITY")
		}
		reservedQuantity := offer.QuantityTotal - offer.QuantityAvailable
		if *req.Quantity < reservedQuantity {
			return nil, fmt.Errorf("INVALID_QUANTITY")
		}
		offer.QuantityTotal = *req.Quantity
		offer.QuantityAvailable = *req.Quantity - reservedQuantity
	}
	if req.IsActive != nil {
		offer.IsActive = *req.IsActive
	}
	if req.PickupStart != nil {
		offer.PickupStart = *req.PickupStart
	}
	if req.PickupEnd != nil {
		offer.PickupEnd = *req.PickupEnd
	}
	if req.ImageURL != nil {
		offer.ImageURL = req.ImageURL
	}
	if req.CategoryID != nil {
		catID, err := uuid.Parse(strings.TrimSpace(*req.CategoryID))
		if err != nil {
			return nil, fmt.Errorf("INVALID_CATEGORY_ID")
		}
		offer.CategoryID = catID
	}

	if err := s.repo.Update(ctx, offer); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, offer.ID)
}

func (s *OfferService) GetPartnerOffers(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]OfferDetailDTO, int64, error) {
	restaurant, err := s.repo.GetRestaurantByPartnerID(ctx, partnerID)
	if err != nil {
		return nil, 0, fmt.Errorf("RESTAURANT_NOT_FOUND")
	}

	offers, total, err := s.repo.GetPartnerOffers(ctx, restaurant.ID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]OfferDetailDTO, len(offers))
	for i, o := range offers {
		dtos[i] = s.mapToDetailDTO(&o)
	}
	return dtos, total, nil
}

func (s *OfferService) GetPublicOffers(ctx context.Context, params FilterParams) ([]OfferPreviewDTO, int64, error) {
	offers, total, err := s.repo.GetPublicOffers(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]OfferPreviewDTO, len(offers))
	for i, o := range offers {
		dtos[i] = s.mapToPreviewDTO(&o)
	}
	return dtos, total, nil
}

func (s *OfferService) GetOfferByID(ctx context.Context, id uuid.UUID) (*OfferDetailDTO, error) {
	offer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("OFFER_NOT_FOUND")
	}
	if !offer.IsActive || offer.QuantityAvailable <= 0 {
		return nil, fmt.Errorf("OFFER_NOT_AVAILABLE")
	}
	dto := s.mapToDetailDTO(offer)
	return &dto, nil
}

func (s *OfferService) DeleteOffer(ctx context.Context, id, partnerID uuid.UUID) error {
	offer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("OFFER_NOT_FOUND")
	}
	if offer.Restaurant.PartnerID != partnerID {
		return fmt.Errorf("FORBIDDEN")
	}
	return s.repo.Delete(ctx, id)
}

func (s *OfferService) mapToDetailDTO(o *domain.Offer) OfferDetailDTO {
	discount := 0
	if o.OriginalPrice > 0 {
		discount = int(math.Round((1 - o.Price/o.OriginalPrice) * 100))
	}
	return OfferDetailDTO{
		ID:                o.ID.String(),
		Title:             o.Title,
		Description:       o.Description,
		Price:             o.Price,
		OriginalPrice:     o.OriginalPrice,
		Discount:          discount,
		ImageURL:          o.ImageURL,
		QuantityAvailable: o.QuantityAvailable,
		QuantityTotal:     o.QuantityTotal,
		PickupStart:       o.PickupStart,
		PickupEnd:         o.PickupEnd,
		IsActive:          o.IsActive,
		Category:          CategoryDTO{ID: o.CategoryID.String(), Name: o.Category.Name},
		Restaurant: RestaurantShortDTO{
			ID:        o.Restaurant.ID.String(),
			Name:      o.Restaurant.Name,
			Address:   o.Restaurant.Address,
			Latitude:  o.Restaurant.Latitude,
			Longitude: o.Restaurant.Longitude,
			Phone:     o.Restaurant.Phone,
		},
	}
}

func (s *OfferService) mapToPreviewDTO(o *domain.Offer) OfferPreviewDTO {
	discount := 0
	if o.OriginalPrice > 0 {
		discount = int(math.Round((1 - o.Price/o.OriginalPrice) * 100))
	}

	return OfferPreviewDTO{
		ID:                o.ID.String(),
		Title:             o.Title,
		Price:             o.Price,
		OriginalPrice:     o.OriginalPrice,
		Discount:          discount,
		ImageURL:          o.ImageURL,
		Distance:          o.Distance,
		PickupStart:       o.PickupStart,
		PickupEnd:         o.PickupEnd,
		QuantityAvailable: o.QuantityAvailable,
		Category:          CategoryDTO{ID: o.CategoryID.String(), Name: o.Category.Name},
		Restaurant: RestaurantShortDTO{
			ID:        o.Restaurant.ID.String(),
			Name:      o.Restaurant.Name,
			Address:   o.Restaurant.Address,
			Latitude:  o.Restaurant.Latitude,
			Longitude: o.Restaurant.Longitude,
			Phone:     o.Restaurant.Phone,
		},
	}
}

func (s *OfferService) UploadImage(file *multipart.FileHeader) (string, error) {
	if s.fileStorage == nil {
		return "", ErrOfferFileStorageUnavailable
	}

	if file == nil {
		return "", ErrOfferInvalidImageFormat
	}

	if file.Size == 0 || file.Size > maxOfferImageSizeBytes {
		return "", ErrOfferImageTooLarge
	}

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	content, err := io.ReadAll(io.LimitReader(src, maxOfferImageSizeBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(content)) > maxOfferImageSizeBytes {
		return "", ErrOfferImageTooLarge
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	normalizedExt, _, isHEIC := s.normalizeFormat(ext, content)
	if normalizedExt == "" {
		return "", ErrOfferUnsupportedImageFormat
	}

	var processed []byte
	outExt := ".jpg"
	var contentType string
	if isHEIC {
		processed, err = s.compressHEICToJPEG(content)
		contentType = "image/jpeg"
	} else if normalizedExt == ".png" {
		processed, outExt, contentType, err = s.processPNG(content)
	} else {
		processed, err = s.compressToJPEG(content)
		contentType = "image/jpeg"
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOfferImageProcessingFailed, err)
	}
	return s.fileStorage.UploadBytesWithPrefix(processed, outExt, contentType, offerImagePrefix)
}

func (s *OfferService) normalizeFormat(ext string, content []byte) (string, string, bool) {
	detectedType := http.DetectContentType(content)
	switch detectedType {
	case "image/jpeg":
		return ".jpg", "image/jpeg", false
	case "image/png":
		return ".png", "image/png", false
	}
	if s.isHEICContent(content) || strings.Contains(detectedType, "heic") {
		return ".heic", "image/heic", true
	}
	return "", "", false
}

func (s *OfferService) isHEICContent(content []byte) bool {
	if len(content) < 12 {
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

func (s *OfferService) compressToJPEG(content []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	return s.encodeToJPEG(img)
}

func (s *OfferService) compressHEICToJPEG(content []byte) ([]byte, error) {
	img, err := heic.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	return s.encodeToJPEG(img)
}

func (s *OfferService) processPNG(content []byte) ([]byte, string, string, error) {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", "", err
	}
	if s.hasTransparency(img) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", "", err
		}
		return buf.Bytes(), ".png", "image/png", nil
	}
	compressed, err := s.encodeToJPEG(img)
	return compressed, ".jpg", "image/jpeg", err
}

func (s *OfferService) encodeToJPEG(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	background := image.NewRGBA(bounds)
	draw.Draw(background, bounds, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(background, bounds, img, bounds.Min, draw.Over)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, background, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *OfferService) hasTransparency(img image.Image) bool {
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

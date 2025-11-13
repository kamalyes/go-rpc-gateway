/*
 * Example: Ads Link Service Integration with go-rpc-gateway
 * This file demonstrates how to integrate engine-ads-link-service into go-rpc-gateway
 * using ONLY gRPC service registration and go-rpc-gateway's native API registration.
 * 
 * Structure:
 * services/adslink/
 * ├── models/
 * │   └── link.go                  # Data models
 * ├── service/
 * │   └── link.go                  # Business logic
 * ├── grpc/
 * │   ├── server.go                # gRPC service implementation
 * │   └── gateway.go               # gRPC gateway HTTP mapping
 * └── middleware.go                # Service-specific middleware
 * 
 * NO Gin, NO custom HTTP handlers - only gRPC + gRPC-Gateway
 */

package adslink

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kamalyes/go-rpc-gateway/gateway"
	pb "github.com/Divine-Dragon-Voyage/commonapis/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// ==================== Data Models ====================

// LinkModel represents a short URL link
type LinkModel struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	URL        string         `gorm:"index;not null" json:"url"`
	ShortCode  string         `gorm:"unique;not null;index" json:"short_code"`
	Title      string         `gorm:"type:varchar(255)" json:"title"`
	Description string        `gorm:"type:text" json:"description"`
	ClickCount int64          `gorm:"default:0" json:"click_count"`
	ExpiresAt  *time.Time     `json:"expires_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (LinkModel) TableName() string {
	return "links"
}

// ==================== Service Layer ====================

// LinkService defines business operations
type LinkService interface {
	CreateLink(ctx context.Context, link *LinkModel) (*LinkModel, error)
	GetLink(ctx context.Context, shortCode string) (*LinkModel, error)
	ListLinks(ctx context.Context, page, pageSize int) ([]*LinkModel, int64, error)
	UpdateLink(ctx context.Context, link *LinkModel) error
	DeleteLink(ctx context.Context, id uint) error
	GetOrCreateLink(ctx context.Context, originalURL string) (*LinkModel, error)
}

type linkService struct {
	db *gorm.DB
}

// NewLinkService creates a new link service
func NewLinkService(db *gorm.DB) LinkService {
	return &linkService{db: db}
}

func (s *linkService) CreateLink(ctx context.Context, link *LinkModel) (*LinkModel, error) {
	if err := s.db.WithContext(ctx).Create(link).Error; err != nil {
		return nil, err
	}
	return link, nil
}

func (s *linkService) GetLink(ctx context.Context, shortCode string) (*LinkModel, error) {
	var link LinkModel
	if err := s.db.WithContext(ctx).Where("short_code = ?", shortCode).First(&link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	// Increment click count
	s.db.WithContext(ctx).Model(&link).Update("click_count", gorm.Expr("click_count + ?", 1))

	return &link, nil
}

func (s *linkService) ListLinks(ctx context.Context, page, pageSize int) ([]*LinkModel, int64, error) {
	var links []*LinkModel
	var total int64

	// Get total count
	if err := s.db.WithContext(ctx).Model(&LinkModel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	if err := s.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&links).Error; err != nil {
		return nil, 0, err
	}

	return links, total, nil
}

func (s *linkService) UpdateLink(ctx context.Context, link *LinkModel) error {
	return s.db.WithContext(ctx).Save(link).Error
}

func (s *linkService) DeleteLink(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&LinkModel{}, id).Error
}

func (s *linkService) GetOrCreateLink(ctx context.Context, originalURL string) (*LinkModel, error) {
	// Try to find existing
	var link LinkModel
	if err := s.db.WithContext(ctx).Where("url = ?", originalURL).First(&link).Error; err == nil {
		return &link, nil
	}

	// Create new
	newLink := &LinkModel{
		URL:       originalURL,
		ShortCode: generateShortCode(),
	}
	return s.CreateLink(ctx, newLink)
}

// ==================== HTTP Handler ====================

// LinkHandler handles gRPC-Gateway HTTP requests
type LinkHandler struct {
	service LinkService
}

// NewLinkHandler creates a new link handler
func NewLinkHandler(service LinkService) *LinkHandler {
	return &LinkHandler{service: service}
}

// Note: No request/response structs needed here!
// They are defined in commonapis/pb/link.proto and auto-generated

// ==================== Service Integration ====================

// InitializeAdsLinkService initializes and registers the ads link service
func InitializeAdsLinkService(gw *gateway.Gateway) error {
	// Get database connection from gateway
	db := gw.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Auto migrate models
	if err := db.AutoMigrate(&LinkModel{}); err != nil {
		return fmt.Errorf("failed to migrate models: %w", err)
	}

	// Initialize service
	linkService := NewLinkService(db)
	linkHandler := NewLinkHandler(linkService)

	// Register HTTP routes
	// Note: This is a simplified example. In practice, use a proper router like Gin
	gw.RegisterHTTPRoute("/api/v1/links", func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost:
			linkHandler.CreateLink(c)
		case http.MethodGet:
			linkHandler.ListLinks(c)
		}
	})

	gw.RegisterHTTPRoute("/api/v1/links/:short_code", func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet:
			linkHandler.GetLink(c)
		}
	})

	return nil
}

// ==================== Helper Functions ====================

import "fmt"

func generateShortCode() string {
	// Generate a random short code (implementation)
	return fmt.Sprintf("link_%d", time.Now().Unix())
}

// PaginatedResponse defines paginated response
type PaginatedResponse struct {
	Data     interface{} `json:"data"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ErrorResponse defines error response
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

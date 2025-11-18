package service

import (
	"errors"

	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
	"github.com/adamscao/videoshare/internal/utils"
)

type AuthService struct{}

func NewAuthService() *AuthService {
	return &AuthService{}
}

// CreateAdmin creates a new admin user
func (s *AuthService) CreateAdmin(username, password string) error {
	hash, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	admin := &models.Admin{
		Username:     username,
		PasswordHash: hash,
	}

	return database.DB.Create(admin).Error
}

// AuthenticateAdmin verifies admin credentials
func (s *AuthService) AuthenticateAdmin(username, password string) (*models.Admin, error) {
	var admin models.Admin
	if err := database.DB.Where("username = ?", username).First(&admin).Error; err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !utils.CheckPassword(password, admin.PasswordHash) {
		return nil, errors.New("invalid credentials")
	}

	return &admin, nil
}

// GetAdminByID retrieves admin by ID
func (s *AuthService) GetAdminByID(id uint) (*models.Admin, error) {
	var admin models.Admin
	if err := database.DB.First(&admin, id).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

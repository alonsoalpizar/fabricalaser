package repository

import (
	"errors"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound      = errors.New("usuario no encontrado")
	ErrUserAlreadyExists = errors.New("ya existe una cuenta con esta cédula")
	ErrEmailAlreadyUsed  = errors.New("ya existe una cuenta con este email")
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		db: database.Get(),
	}
}

// FindByID finds a user by ID
func (r *UserRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// FindByCedula finds a user by cedula
func (r *UserRepository) FindByCedula(cedula string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("cedula = ?", cedula).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// FindByCedulaWithPassword finds a user by cedula that has a password set
func (r *UserRepository) FindByCedulaWithPassword(cedula string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("cedula = ? AND password_hash IS NOT NULL", cedula).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// FindByEmail finds a user by email that has a password set
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ? AND password_hash IS NOT NULL", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// ExistsByCedulaWithPassword checks if a user with password exists for the given cedula
func (r *UserRepository) ExistsByCedulaWithPassword(cedula string) (bool, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Where("cedula = ? AND password_hash IS NOT NULL", cedula).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsByEmailWithPassword checks if a user with password exists for the given email
func (r *UserRepository) ExistsByEmailWithPassword(email string) (bool, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Where("email = ? AND password_hash IS NOT NULL", email).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create creates a new user
func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// Update updates an existing user
func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// UpdateLastLogin updates the ultimo_login timestamp
func (r *UserRepository) UpdateLastLogin(id uint) error {
	now := time.Now()
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("ultimo_login", now).Error
}

// SetPassword sets the password for a user (used for establecer-password)
func (r *UserRepository) SetPassword(id uint, passwordHash, email, telefono, cedulaType string) error {
	updates := map[string]interface{}{
		"password_hash": passwordHash,
		"cedula_type":   cedulaType,
		"activo":        true,
		"ultimo_login":  time.Now(),
	}

	if email != "" {
		updates["email"] = email
	}
	if telefono != "" {
		updates["telefono"] = telefono
	}

	return r.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

// IncrementQuotesUsed increments the quotes_used counter
func (r *UserRepository) IncrementQuotesUsed(id uint) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).
		UpdateColumn("quotes_used", gorm.Expr("quotes_used + ?", 1)).Error
}

// UpdateMetadata updates only the metadata field for a user
func (r *UserRepository) UpdateMetadata(id uint, metadata interface{}) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("metadata", metadata).Error
}

// UpdateProfile updates user profile fields
func (r *UserRepository) UpdateProfile(id uint, email, telefono, direccion, provincia, canton, distrito *string) error {
	updates := make(map[string]interface{})

	if email != nil {
		updates["email"] = *email
	}
	if telefono != nil {
		updates["telefono"] = telefono
	}
	if direccion != nil {
		updates["direccion"] = direccion
	}
	if provincia != nil {
		updates["provincia"] = provincia
	}
	if canton != nil {
		updates["canton"] = canton
	}
	if distrito != nil {
		updates["distrito"] = distrito
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

// ListAll returns all users with pagination and filters (for admin)
func (r *UserRepository) ListAll(limit, offset int, search, role string, isActive *bool) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := r.db.Model(&models.User{})

	// Apply filters
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("nombre ILIKE ? OR cedula ILIKE ? OR email ILIKE ?", searchPattern, searchPattern, searchPattern)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if isActive != nil {
		query = query.Where("activo = ?", *isActive)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// Delete soft deletes a user (or hard delete if needed)
func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

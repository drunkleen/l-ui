package service

import (
	"errors"
	"time"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/util/random"
	"gorm.io/gorm"
)

var (
	ErrTokenExpired     = errors.New("registration token has expired")
	ErrTokenConsumed    = errors.New("registration token has already been used")
	ErrTokenNotFound    = errors.New("registration token not found")
	ErrTokenInvalid     = errors.New("registration token is invalid")
	ErrTokenNameTooLong = errors.New("node name exceeds 128 characters")
	ErrTokenAddrTooLong = errors.New("node address exceeds 128 characters")
)

const defaultTokenTTL = 24 * time.Hour

type RegistrationService struct{}

func (s *RegistrationService) GenerateToken(nodeName, nodeAddress string, ttl time.Duration) (*model.NodeRegistrationToken, error) {
	if len(nodeName) > 128 {
		return nil, ErrTokenNameTooLong
	}
	if len(nodeAddress) > 128 {
		return nil, ErrTokenAddrTooLong
	}
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}

	db := database.GetDB()
	token := &model.NodeRegistrationToken{
		Token:       random.Seq(48),
		NodeName:    nodeName,
		NodeAddress: nodeAddress,
		ExpiresAt:   time.Now().Add(ttl).UnixMilli(),
	}
	if err := db.Create(token).Error; err != nil {
		return nil, err
	}
	return token, nil
}

func (s *RegistrationService) ValidateToken(tokenStr string) (*model.NodeRegistrationToken, error) {
	if tokenStr == "" {
		return nil, ErrTokenInvalid
	}
	db := database.GetDB()
	var token model.NodeRegistrationToken
	if err := db.Where("token = ?", tokenStr).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	if token.ConsumedAt > 0 {
		return nil, ErrTokenConsumed
	}
	if token.ExpiresAt > 0 && time.Now().UnixMilli() > token.ExpiresAt {
		return nil, ErrTokenExpired
	}
	return &token, nil
}

func (s *RegistrationService) ConsumeToken(tokenStr string, nodeID int) error {
	if tokenStr == "" {
		return ErrTokenInvalid
	}
	token, err := s.ValidateToken(tokenStr)
	if err != nil {
		return err
	}
	db := database.GetDB()
	return db.Model(token).Updates(map[string]any{
		"consumed_by_node_id": nodeID,
		"consumed_at":         time.Now().UnixMilli(),
	}).Error
}

func (s *RegistrationService) ListTokens() ([]model.NodeRegistrationToken, error) {
	db := database.GetDB()
	var tokens []model.NodeRegistrationToken
	if err := db.Order("created_at desc").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *RegistrationService) DeleteToken(id int) error {
	db := database.GetDB()
	result := db.Delete(&model.NodeRegistrationToken{}, id)
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

func (s *RegistrationService) CleanupExpired() (int64, error) {
	db := database.GetDB()
	now := time.Now().UnixMilli()
	result := db.Where("expires_at > 0 AND expires_at < ?", now).Delete(&model.NodeRegistrationToken{})
	return result.RowsAffected, result.Error
}

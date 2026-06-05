package inbound

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

var (
	ErrTagExists  = errors.New("tag exists")
	ErrNotFound   = errors.New("not found")
	ErrInvalidTag = errors.New("invalid tag")
	ErrPortInUse  = errors.New("port in use")
)

// ConfigWriter is satisfied by *singbox.Service (SaveConfig).
type ConfigWriter interface {
	SaveConfig(content string) error
}

type Service struct {
	db     *gorm.DB
	writer ConfigWriter
	keygen KeyGen
}

func NewService(db *gorm.DB, writer ConfigWriter, keygen KeyGen) *Service {
	return &Service{db: db, writer: writer, keygen: keygen}
}

func (s *Service) ListInbounds() ([]models.Inbound, error) {
	var ins []models.Inbound
	err := s.db.Preload("Users").Order("id").Find(&ins).Error
	return ins, err
}

func (s *Service) CreateInbound(tag string, port uint16, handshake string) (models.Inbound, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.Inbound{}, ErrInvalidTag
	}
	var count int64
	s.db.Model(&models.Inbound{}).Where("tag = ?", tag).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrTagExists
	}
	s.db.Model(&models.Inbound{}).Where("port = ?", port).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrPortInUse
	}
	priv, pub, err := s.keygen.RealityKeypair()
	if err != nil {
		return models.Inbound{}, err
	}
	in := models.Inbound{
		Tag: tag, Port: port, Flow: "xtls-rprx-vision",
		RealityPrivateKey: priv, RealityPublicKey: pub, RealityShortID: s.keygen.ShortID(),
		Handshake: handshake, HandshakePort: 443, ServerName: handshake,
	}
	if err := s.db.Create(&in).Error; err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) DeleteInbound(id uint) error {
	res := s.db.Select("Users").Delete(&models.Inbound{ID: id})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return s.Regenerate()
}

func (s *Service) ListUsers(inboundID uint) ([]models.User, error) {
	var us []models.User
	err := s.db.Where("inbound_id = ?", inboundID).Order("id").Find(&us).Error
	return us, err
}

func (s *Service) CreateUser(inboundID uint, name string) (models.User, error) {
	var in models.Inbound
	if err := s.db.First(&in, inboundID).Error; err != nil {
		return models.User{}, ErrNotFound
	}
	u := models.User{InboundID: inboundID, Name: name, UUID: s.keygen.UUID()}
	if err := s.db.Create(&u).Error; err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) DeleteUser(id uint) error {
	res := s.db.Delete(&models.User{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return s.Regenerate()
}

// Regenerate rebuilds config.json from the current inbounds/users.
func (s *Service) Regenerate() error {
	ins, err := s.ListInbounds()
	if err != nil {
		return err
	}
	content, err := Generate(ins)
	if err != nil {
		return err
	}
	return s.writer.SaveConfig(content)
}

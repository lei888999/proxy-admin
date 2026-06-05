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

type ConfigWriter interface {
	SaveConfig(content string) error
}

type Service struct {
	db     *gorm.DB
	writer ConfigWriter
}

func NewService(db *gorm.DB, writer ConfigWriter) *Service {
	return &Service{db: db, writer: writer}
}

type InboundView struct {
	ID         uint           `json:"id"`
	Type       string         `json:"type"`
	Tag        string         `json:"tag"`
	Port       uint16         `json:"port"`
	Network    string         `json:"network"`
	PublicInfo map[string]any `json:"publicInfo"`
	Users      []models.User  `json:"users"`
}

func (s *Service) listInbounds() ([]models.Inbound, error) {
	var ins []models.Inbound
	err := s.db.Preload("Users").Order("id").Find(&ins).Error
	return ins, err
}

func (s *Service) ListInboundViews() ([]InboundView, error) {
	ins, err := s.listInbounds()
	if err != nil {
		return nil, err
	}
	views := make([]InboundView, 0, len(ins))
	for _, in := range ins {
		v := InboundView{ID: in.ID, Type: in.Type, Tag: in.Tag, Port: in.Port, Network: in.Network, Users: in.Users}
		if d, ok := Get(in.Type); ok {
			if pi, err := d.PublicInfo(in.Settings); err == nil {
				v.PublicInfo = pi
			}
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *Service) Types() []TypeInfo { return Types() }

func (s *Service) CreateInbound(typ, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	d, ok := Get(typ)
	if !ok {
		return models.Inbound{}, ErrUnknownType
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.Inbound{}, ErrInvalidTag
	}
	var count int64
	s.db.Model(&models.Inbound{}).Where("tag = ?", tag).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrTagExists
	}
	s.db.Model(&models.Inbound{}).Where("port = ? AND network = ?", port, d.Network()).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrPortInUse
	}
	settings, err := d.BuildSettings(params)
	if err != nil {
		return models.Inbound{}, err
	}
	in := models.Inbound{Tag: tag, Type: typ, Network: d.Network(), Port: port, Settings: settings}
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
	d, ok := Get(in.Type)
	if !ok {
		return models.User{}, ErrUnknownType
	}
	u := models.User{InboundID: inboundID, Name: name, Credential: d.NewCredential()}
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

func (s *Service) Regenerate() error {
	ins, err := s.listInbounds()
	if err != nil {
		return err
	}
	content, err := Generate(ins)
	if err != nil {
		return err
	}
	return s.writer.SaveConfig(content)
}

package inbound

import (
	"strings"

	"singbox-admin/internal/models"
)

type OutboundView struct {
	ID       uint   `json:"id"`
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     uint16 `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

var outboundTypes = map[string]bool{"http": true, "socks5": true}

func (s *Service) listOutbounds() ([]models.Outbound, error) {
	var obs []models.Outbound
	err := s.db.Order("id").Find(&obs).Error
	return obs, err
}

func (s *Service) ListOutboundViews() ([]OutboundView, error) {
	obs, err := s.listOutbounds()
	if err != nil {
		return nil, err
	}
	views := make([]OutboundView, 0, len(obs))
	for _, o := range obs {
		views = append(views, OutboundView{
			ID: o.ID, Tag: o.Tag, Type: o.Type, Server: o.Server,
			Port: o.Port, Username: o.Username, Password: o.Password,
		})
	}
	return views, nil
}

func validateOutboundFields(typ, tag, server string, port uint16) (string, string, string, error) {
	typ = strings.TrimSpace(typ)
	if !outboundTypes[typ] {
		return "", "", "", ErrInvalidType
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", "", "", ErrInvalidTag
	}
	server = strings.TrimSpace(server)
	if server == "" || port == 0 {
		return "", "", "", ErrInvalidOutbound
	}
	return typ, tag, server, nil
}

func (s *Service) CreateOutbound(typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	typ, tag, server, err := validateOutboundFields(typ, tag, server, port)
	if err != nil {
		return models.Outbound{}, err
	}
	var count int64
	s.db.Model(&models.Outbound{}).Where("tag = ?", tag).Count(&count)
	if count > 0 {
		return models.Outbound{}, ErrTagExists
	}
	o := models.Outbound{Tag: tag, Type: typ, Server: server, Port: port, Username: username, Password: password}
	if err := s.db.Create(&o).Error; err != nil {
		return models.Outbound{}, err
	}
	return o, s.Regenerate()
}

func (s *Service) UpdateOutbound(id uint, typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	var o models.Outbound
	if err := s.db.First(&o, id).Error; err != nil {
		return models.Outbound{}, ErrNotFound
	}
	typ, tag, server, err := validateOutboundFields(typ, tag, server, port)
	if err != nil {
		return models.Outbound{}, err
	}
	var count int64
	s.db.Model(&models.Outbound{}).Where("tag = ? AND id <> ?", tag, id).Count(&count)
	if count > 0 {
		return models.Outbound{}, ErrTagExists
	}
	o.Type, o.Tag, o.Server, o.Port, o.Username, o.Password = typ, tag, server, port, username, password
	if err := s.db.Save(&o).Error; err != nil {
		return models.Outbound{}, err
	}
	return o, s.Regenerate()
}

func (s *Service) DeleteOutbound(id uint) error {
	var o models.Outbound
	if err := s.db.First(&o, id).Error; err != nil {
		return ErrNotFound
	}
	// Referencing users fall back to direct.
	if err := s.db.Model(&models.User{}).Where("outbound_id = ?", id).Update("outbound_id", nil).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&o).Error; err != nil {
		return err
	}
	return s.Regenerate()
}

// validateOutbound checks that an optional assigned outbound exists.
func (s *Service) validateOutbound(id *uint) error {
	if id == nil {
		return nil
	}
	var count int64
	s.db.Model(&models.Outbound{}).Where("id = ?", *id).Count(&count)
	if count == 0 {
		return ErrInvalidOutbound
	}
	return nil
}

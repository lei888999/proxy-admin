package inbound

import (
	"strings"

	"gorm.io/gorm"

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
	var o models.Outbound
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Outbound{}).Where("tag = ?", tag).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrTagExists
		}
		o = models.Outbound{Tag: tag, Type: typ, Server: server, Port: port, Username: username, Password: password}
		return tx.Create(&o).Error
	})
	if err != nil {
		return models.Outbound{}, err
	}
	return o, s.Regenerate()
}

func (s *Service) UpdateOutbound(id uint, typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	typ, tag, server, err := validateOutboundFields(typ, tag, server, port)
	if err != nil {
		return models.Outbound{}, err
	}
	var o models.Outbound
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&o, id).Error; err != nil {
			return ErrNotFound
		}
		var count int64
		if err := tx.Model(&models.Outbound{}).Where("tag = ? AND id <> ?", tag, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrTagExists
		}
		o.Type, o.Tag, o.Server, o.Port, o.Username, o.Password = typ, tag, server, port, username, password
		return tx.Save(&o).Error
	})
	if err != nil {
		return models.Outbound{}, err
	}
	return o, s.Regenerate()
}

func (s *Service) DeleteOutbound(id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var o models.Outbound
		if err := tx.First(&o, id).Error; err != nil {
			return ErrNotFound
		}
		var users int64
		if err := tx.Model(&models.User{}).Where("outbound_id = ?", id).Count(&users).Error; err != nil {
			return err
		}
		// Falling back to direct would silently expose the client's address after
		// an admin deletes a live egress. Require reassignment first.
		if users > 0 {
			return ErrOutboundInUse
		}
		return tx.Delete(&o).Error
	})
	if err != nil {
		return err
	}
	return s.Regenerate()
}

// validateOutbound checks that an optional assigned outbound exists, within the
// caller's transaction so the check cannot be invalidated before the write.
func validateOutbound(tx *gorm.DB, id *uint) error {
	if id == nil {
		return nil
	}
	var count int64
	if err := tx.Model(&models.Outbound{}).Where("id = ?", *id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrInvalidOutbound
	}
	return nil
}

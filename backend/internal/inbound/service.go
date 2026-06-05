package inbound

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

var (
	ErrTagExists   = errors.New("tag exists")
	ErrNotFound    = errors.New("not found")
	ErrInvalidTag  = errors.New("invalid tag")
	ErrInvalidName = errors.New("invalid name")
	ErrPortInUse   = errors.New("port in use")
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

func genUUID() string { return NewKeyGen().UUID() }

func genPassword() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func genToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type InboundView struct {
	ID         uint           `json:"id"`
	Type       string         `json:"type"`
	Tag        string         `json:"tag"`
	Port       uint16         `json:"port"`
	Network    string         `json:"network"`
	PublicInfo map[string]any `json:"publicInfo"`
}

func (s *Service) listInbounds() ([]models.Inbound, error) {
	var ins []models.Inbound
	err := s.db.Preload("Users").Order("id").Find(&ins).Error
	return ins, err
}

func (s *Service) ListInboundViews() ([]InboundView, error) {
	var ins []models.Inbound
	if err := s.db.Order("id").Find(&ins).Error; err != nil {
		return nil, err
	}
	views := make([]InboundView, 0, len(ins))
	for _, in := range ins {
		v := InboundView{ID: in.ID, Type: in.Type, Tag: in.Tag, Port: in.Port, Network: in.Network}
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

func (s *Service) UpdateInbound(id uint, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	var in models.Inbound
	if err := s.db.First(&in, id).Error; err != nil {
		return models.Inbound{}, ErrNotFound
	}
	d, ok := Get(in.Type)
	if !ok {
		return models.Inbound{}, ErrUnknownType
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.Inbound{}, ErrInvalidTag
	}
	var count int64
	s.db.Model(&models.Inbound{}).Where("tag = ? AND id <> ?", tag, id).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrTagExists
	}
	s.db.Model(&models.Inbound{}).Where("port = ? AND network = ? AND id <> ?", port, in.Network, id).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrPortInUse
	}
	settings, err := d.UpdateSettings(in.Settings, params)
	if err != nil {
		return models.Inbound{}, err
	}
	in.Tag, in.Port, in.Settings = tag, port, settings
	if err := s.db.Save(&in).Error; err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) ResetInboundKeys(id uint) (models.Inbound, error) {
	var in models.Inbound
	if err := s.db.First(&in, id).Error; err != nil {
		return models.Inbound{}, ErrNotFound
	}
	d, ok := Get(in.Type)
	if !ok {
		return models.Inbound{}, ErrUnknownType
	}
	settings, err := d.ResetSecrets(in.Settings)
	if err != nil {
		return models.Inbound{}, err
	}
	in.Settings = settings
	if err := s.db.Save(&in).Error; err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) DeleteInbound(id uint) error {
	var in models.Inbound
	if err := s.db.First(&in, id).Error; err != nil {
		return ErrNotFound
	}
	if err := s.db.Model(&in).Association("Users").Clear(); err != nil {
		return err
	}
	if err := s.db.Delete(&in).Error; err != nil {
		return err
	}
	return s.Regenerate()
}

type UserView struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	UUID        string   `json:"uuid"`
	Password    string   `json:"password"`
	SubToken    string   `json:"subToken"`
	UpBytes     int64    `json:"upBytes"`
	DownBytes   int64    `json:"downBytes"`
	InboundIDs  []uint   `json:"inboundIds"`
	InboundTags []string `json:"inboundTags"`
}

func (s *Service) ListUserViews() ([]UserView, error) {
	var us []models.User
	if err := s.db.Preload("Inbounds").Order("id").Find(&us).Error; err != nil {
		return nil, err
	}
	views := make([]UserView, 0, len(us))
	for _, u := range us {
		v := UserView{ID: u.ID, Name: u.Name, UUID: u.UUID, Password: u.Password, SubToken: u.SubToken,
			UpBytes: u.UpBytes, DownBytes: u.DownBytes, InboundIDs: []uint{}, InboundTags: []string{}}
		for _, in := range u.Inbounds {
			v.InboundIDs = append(v.InboundIDs, in.ID)
			v.InboundTags = append(v.InboundTags, in.Tag)
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *Service) setUserInbounds(u *models.User, inboundIDs []uint) error {
	var ins []models.Inbound
	if len(inboundIDs) > 0 {
		if err := s.db.Find(&ins, inboundIDs).Error; err != nil {
			return err
		}
	}
	return s.db.Model(u).Association("Inbounds").Replace(ins)
}

func (s *Service) CreateUser(name string, inboundIDs []uint) (models.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	u := models.User{Name: name, UUID: genUUID(), Password: genPassword(), SubToken: genToken()}
	if err := s.db.Create(&u).Error; err != nil {
		return models.User{}, err
	}
	if err := s.setUserInbounds(&u, inboundIDs); err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) UpdateUser(id uint, name string, inboundIDs []uint) (models.User, error) {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return models.User{}, ErrNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	u.Name = name
	if err := s.db.Save(&u).Error; err != nil {
		return models.User{}, err
	}
	if err := s.setUserInbounds(&u, inboundIDs); err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) ResetUserCreds(id uint) (models.User, error) {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return models.User{}, ErrNotFound
	}
	u.UUID, u.Password, u.SubToken = genUUID(), genPassword(), genToken()
	if err := s.db.Save(&u).Error; err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

// BackfillUserTokens gives a SubToken to any pre-existing user that lacks one.
func (s *Service) BackfillUserTokens() error {
	var us []models.User
	if err := s.db.Where("sub_token = '' OR sub_token IS NULL").Find(&us).Error; err != nil {
		return err
	}
	for i := range us {
		us[i].SubToken = genToken()
		if err := s.db.Model(&us[i]).Update("sub_token", us[i].SubToken).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DeleteUser(id uint) error {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return ErrNotFound
	}
	if err := s.db.Model(&u).Association("Inbounds").Clear(); err != nil {
		return err
	}
	if err := s.db.Delete(&u).Error; err != nil {
		return err
	}
	return s.Regenerate()
}

// AddTraffic accumulates a delta onto a user's cumulative counters.
func (s *Service) AddTraffic(userID uint, up, down int64) error {
	return s.db.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumns(map[string]any{
			"up_bytes":   gorm.Expr("up_bytes + ?", up),
			"down_bytes": gorm.Expr("down_bytes + ?", down),
		}).Error
}

// ResetUserTraffic zeroes a user's cumulative counters.
func (s *Service) ResetUserTraffic(id uint) error {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return ErrNotFound
	}
	return s.db.Model(&u).UpdateColumns(map[string]any{"up_bytes": 0, "down_bytes": 0}).Error
}

const (
	metaClashAddr   = "clash_api_addr"
	metaClashSecret = "clash_api_secret"

	defaultClashAddr = "127.0.0.1:9090"
)

// APIConfig returns the experimental API endpoints, generating + persisting
// them in the meta table on first use so config and pollers stay in sync.
func (s *Service) APIConfig() (ExperimentalConfig, error) {
	get := func(key, def string) (string, error) {
		var m models.Meta
		err := s.db.First(&m, "key = ?", key).Error
		if err == nil {
			return m.Value, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		val := def
		if key == metaClashSecret {
			val = genToken()
		}
		if err := s.db.Create(&models.Meta{Key: key, Value: val}).Error; err != nil {
			return "", err
		}
		return val, nil
	}
	clashAddr, err := get(metaClashAddr, defaultClashAddr)
	if err != nil {
		return ExperimentalConfig{}, err
	}
	secret, err := get(metaClashSecret, "")
	if err != nil {
		return ExperimentalConfig{}, err
	}
	return ExperimentalConfig{ClashAddr: clashAddr, ClashSecret: secret}, nil
}

func (s *Service) Regenerate() error {
	ins, err := s.listInbounds()
	if err != nil {
		return err
	}
	exp, err := s.APIConfig()
	if err != nil {
		return err
	}
	content, err := Generate(ins, exp)
	if err != nil {
		return err
	}
	return s.writer.SaveConfig(content)
}

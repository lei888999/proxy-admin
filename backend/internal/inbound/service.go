package inbound

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

var (
	ErrTagExists       = errors.New("tag exists")
	ErrNotFound        = errors.New("not found")
	ErrInvalidTag      = errors.New("invalid tag")
	ErrInvalidName     = errors.New("invalid name")
	ErrPortInUse       = errors.New("port in use")
	ErrPortReserved    = errors.New("port reserved by the panel")
	ErrInvalidType     = errors.New("invalid type")
	ErrInvalidOutbound = errors.New("invalid outbound")
)

// ApplyError marks a failure that happened AFTER the database change committed:
// the row is saved, but regenerating or applying the sing-box config failed.
// Callers must report these as success-with-warning — retrying the business
// operation would only collide with the row that is already there.
type ApplyError struct{ Err error }

func (e *ApplyError) Error() string { return e.Err.Error() }
func (e *ApplyError) Unwrap() error { return e.Err }

type ConfigWriter interface {
	SaveConfig(content string) error
	// ApplyConfig persists the config and, if sing-box is running, validates and
	// restarts it so changes take effect without a manual "apply".
	ApplyConfig(content string) error
}

// portKey identifies a listen port within one network ("tcp"/"udp").
type portKey struct {
	port    uint16
	network string
}

type Service struct {
	db     *gorm.DB
	writer ConfigWriter

	// mu serializes config regeneration. Every mutation ends in a read-all ->
	// Generate -> ApplyConfig sequence; without this, two concurrent requests
	// interleave their writes and stop/start cycles and leave the config file,
	// the pid file and the live process disagreeing.
	mu sync.Mutex

	// reserved holds ports the panel itself occupies, so an inbound cannot be
	// created on one. sing-box would merely fail to bind, and the only clue
	// would be in its own log.
	resMu    sync.RWMutex
	reserved map[portKey]string
}

func NewService(db *gorm.DB, writer ConfigWriter) *Service {
	return &Service{db: db, writer: writer, reserved: map[portKey]string{}}
}

// ReservePort marks port/network as taken by the panel, with a human label used
// in the rejection message.
func (s *Service) ReservePort(port uint16, network, label string) {
	if port == 0 {
		return
	}
	s.resMu.Lock()
	defer s.resMu.Unlock()
	s.reserved[portKey{port, network}] = label
}

func (s *Service) checkReserved(port uint16, network string) error {
	s.resMu.RLock()
	defer s.resMu.RUnlock()
	if label, ok := s.reserved[portKey{port, network}]; ok {
		return fmt.Errorf("%w: %s", ErrPortReserved, label)
	}
	return nil
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
	// SettingsError surfaces a settings blob that no longer decodes, instead of
	// silently rendering an inbound with no detail at all.
	SettingsError string `json:"settingsError,omitempty"`
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
		d, ok := Get(in.Type)
		if !ok {
			v.SettingsError = "unknown inbound type: " + in.Type
		} else if pi, err := d.PublicInfo(in.Settings); err != nil {
			v.SettingsError = err.Error()
		} else {
			v.PublicInfo = pi
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *Service) Types() []TypeInfo { return Types() }

// ensureInboundFree rejects a duplicate tag or an already-used port/network pair
// within the given transaction, so the check and the insert cannot be separated
// by a concurrent writer.
func ensureInboundFree(tx *gorm.DB, tag string, port uint16, network string, excludeID uint) error {
	var n int64
	q := tx.Model(&models.Inbound{}).Where("tag = ?", tag)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ErrTagExists
	}
	q = tx.Model(&models.Inbound{}).Where("port = ? AND network = ?", port, network)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ErrPortInUse
	}
	return nil
}

func (s *Service) CreateInbound(typ, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	d, ok := Get(typ)
	if !ok {
		return models.Inbound{}, ErrUnknownType
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.Inbound{}, ErrInvalidTag
	}
	if err := s.checkReserved(port, d.Network()); err != nil {
		return models.Inbound{}, err
	}
	var in models.Inbound
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := ensureInboundFree(tx, tag, port, d.Network(), 0); err != nil {
			return err
		}
		settings, err := d.BuildSettings(params)
		if err != nil {
			return err
		}
		in = models.Inbound{Tag: tag, Type: typ, Network: d.Network(), Port: port, Settings: settings}
		return tx.Create(&in).Error
	})
	if err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) UpdateInbound(id uint, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.Inbound{}, ErrInvalidTag
	}
	var in models.Inbound
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&in, id).Error; err != nil {
			return ErrNotFound
		}
		d, ok := Get(in.Type)
		if !ok {
			return ErrUnknownType
		}
		if err := s.checkReserved(port, in.Network); err != nil {
			return err
		}
		if err := ensureInboundFree(tx, tag, port, in.Network, id); err != nil {
			return err
		}
		settings, err := d.UpdateSettings(in.Settings, params)
		if err != nil {
			return err
		}
		in.Tag, in.Port, in.Settings = tag, port, settings
		return tx.Save(&in).Error
	})
	if err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) ResetInboundKeys(id uint) (models.Inbound, error) {
	var in models.Inbound
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&in, id).Error; err != nil {
			return ErrNotFound
		}
		d, ok := Get(in.Type)
		if !ok {
			return ErrUnknownType
		}
		settings, err := d.ResetSecrets(in.Settings)
		if err != nil {
			return err
		}
		in.Settings = settings
		return tx.Save(&in).Error
	})
	if err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) DeleteInbound(id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var in models.Inbound
		if err := tx.First(&in, id).Error; err != nil {
			return ErrNotFound
		}
		if err := tx.Model(&in).Association("Users").Clear(); err != nil {
			return err
		}
		return tx.Delete(&in).Error
	})
	if err != nil {
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
	OutboundID  *uint    `json:"outboundId"`
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
			UpBytes: u.UpBytes, DownBytes: u.DownBytes, OutboundID: u.OutboundID, InboundIDs: []uint{}, InboundTags: []string{}}
		for _, in := range u.Inbounds {
			v.InboundIDs = append(v.InboundIDs, in.ID)
			v.InboundTags = append(v.InboundTags, in.Tag)
		}
		views = append(views, v)
	}
	return views, nil
}

func setUserInbounds(tx *gorm.DB, u *models.User, inboundIDs []uint) error {
	var ins []models.Inbound
	if len(inboundIDs) > 0 {
		if err := tx.Find(&ins, inboundIDs).Error; err != nil {
			return err
		}
	}
	return tx.Model(u).Association("Inbounds").Replace(ins)
}

func (s *Service) CreateUser(name string, inboundIDs []uint, outboundID *uint) (models.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	var u models.User
	// One transaction so a user is never left half-created: the old code created
	// the row, failed to attach inbounds, and reported an error for a user that
	// already existed with no inbounds at all.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := validateOutbound(tx, outboundID); err != nil {
			return err
		}
		u = models.User{Name: name, UUID: genUUID(), Password: genPassword(), SubToken: genToken(), OutboundID: outboundID}
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		return setUserInbounds(tx, &u, inboundIDs)
	})
	if err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) UpdateUser(id uint, name string, inboundIDs []uint, outboundID *uint) (models.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	var u models.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&u, id).Error; err != nil {
			return ErrNotFound
		}
		if err := validateOutbound(tx, outboundID); err != nil {
			return err
		}
		if err := tx.Model(&u).Select("name", "outbound_id").
			Updates(map[string]any{"name": name, "outbound_id": outboundID}).Error; err != nil {
			return err
		}
		return setUserInbounds(tx, &u, inboundIDs)
	})
	if err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) ResetUserCreds(id uint) (models.User, error) {
	var u models.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&u, id).Error; err != nil {
			return ErrNotFound
		}
		u.UUID, u.Password, u.SubToken = genUUID(), genPassword(), genToken()
		return tx.Save(&u).Error
	})
	if err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

// BackfillUserTokens gives a SubToken to any pre-existing user that lacks one.
func (s *Service) BackfillUserTokens() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var us []models.User
		if err := tx.Where("sub_token = '' OR sub_token IS NULL").Find(&us).Error; err != nil {
			return err
		}
		for i := range us {
			if err := tx.Model(&us[i]).Update("sub_token", genToken()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) DeleteUser(id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var u models.User
		if err := tx.First(&u, id).Error; err != nil {
			return ErrNotFound
		}
		if err := tx.Model(&u).Association("Inbounds").Clear(); err != nil {
			return err
		}
		return tx.Delete(&u).Error
	})
	if err != nil {
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

// Regenerate rebuilds the sing-box config from the database and applies it.
// Every failure here is an *ApplyError: the caller's database change is already
// committed, so this is a "saved but not applied" condition, not a failed write.
func (s *Service) Regenerate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.regenerate(); err != nil {
		return &ApplyError{Err: err}
	}
	return nil
}

func (s *Service) regenerate() error {
	ins, err := s.listInbounds()
	if err != nil {
		return err
	}
	obs, err := s.listOutbounds()
	if err != nil {
		return err
	}
	exp, err := s.APIConfig()
	if err != nil {
		return err
	}
	content, err := Generate(ins, obs, exp)
	if err != nil {
		return err
	}
	// ApplyConfig writes the config and auto-restarts a running sing-box (after
	// validation) so edits take effect without a manual "应用并重启".
	return s.writer.ApplyConfig(content)
}

package singbox

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ConfigStore struct{ path string }

func NewConfigStore(path string) *ConfigStore { return &ConfigStore{path: path} }

func (c *ConfigStore) Path() string { return c.path }
func (c *ConfigStore) Exists() bool { _, err := os.Stat(c.path); return err == nil }

func (c *ConfigStore) Get() (string, error) {
	b, err := os.ReadFile(c.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *ConfigStore) Save(content string) error {
	if !json.Valid([]byte(content)) {
		return ErrInvalidJSON
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

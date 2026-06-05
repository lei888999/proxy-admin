package singbox

import (
	"path/filepath"
	"testing"
)

func TestConfigStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	cs := NewConfigStore(filepath.Join(dir, "config.json"))
	if cs.Exists() {
		t.Fatal("should not exist initially")
	}
	if got, err := cs.Get(); err != nil || got != "" {
		t.Fatalf("empty Get = %q, %v", got, err)
	}
	if err := cs.Save(`{"log":{"level":"info"}}`); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !cs.Exists() {
		t.Fatal("should exist after save")
	}
	got, err := cs.Get()
	if err != nil || got != `{"log":{"level":"info"}}` {
		t.Fatalf("Get = %q, %v", got, err)
	}
}

func TestConfigStoreRejectsInvalidJSON(t *testing.T) {
	cs := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	if err := cs.Save("{not json"); err != ErrInvalidJSON {
		t.Fatalf("err = %v, want ErrInvalidJSON", err)
	}
	if cs.Exists() {
		t.Fatal("invalid save must not create the file")
	}
}

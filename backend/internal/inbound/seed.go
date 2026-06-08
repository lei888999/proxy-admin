package inbound

import "singbox-admin/internal/models"

// SeedDefaults creates one vless-reality (tcp 4443) and one hysteria2 (udp 8443)
// inbound when there are none yet. Idempotent.
func SeedDefaults(s *Service) error {
	var n int64
	if err := s.db.Model(&models.Inbound{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := s.CreateInbound("vless-reality", "vless-reality", 4443, nil); err != nil {
		return err
	}
	if _, err := s.CreateInbound("hysteria2", "hysteria2", 8443, nil); err != nil {
		return err
	}
	return nil
}

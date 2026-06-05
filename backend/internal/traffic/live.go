package traffic

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Live is a one-tick throughput snapshot in bytes/second.
type Live struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// ReadLive reads a single tick from the Clash API /traffic stream. On any
// failure it returns a zero snapshot (the widget degrades gracefully).
func ReadLive(addr, secret string) Live {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/traffic", nil)
	if err != nil {
		return Live{}
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Live{}
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	if sc.Scan() {
		var l Live
		if json.Unmarshal(sc.Bytes(), &l) == nil {
			return l
		}
	}
	return Live{}
}

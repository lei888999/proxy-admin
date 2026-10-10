package singbox

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxLogBytes = 1 << 20
const maxLogConnections = 300

type LoggedOutbound struct {
	Type   string `json:"type"`
	Tag    string `json:"tag"`
	Target string `json:"target"`
}

type ConnectionLog struct {
	ID        string           `json:"id"`
	Time      string           `json:"time"`
	User      string           `json:"user"`
	Inbound   string           `json:"inbound"`
	Source    string           `json:"source"`
	Target    string           `json:"target"`
	Outbounds []LoggedOutbound `json:"outbounds"`
	Error     string           `json:"error"`
}

type ConnectionLogs struct {
	Entries   []ConnectionLog `json:"entries"`
	Available bool            `json:"available"`
	Truncated bool            `json:"truncated"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// ConnectionLogs reads a bounded tail of the existing managed-process log.
// Unlike polling active connections, this retains short/failed connections too.
// It never infers a route from the configured user assignment.
func (s *Service) ConnectionLogs() (ConnectionLogs, error) {
	result := ConnectionLogs{Entries: []ConnectionLog{}, UpdatedAt: time.Now()}
	f, err := os.Open(filepath.Join(s.dir, "sing-box.log"))
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	start := info.Size() - maxLogBytes
	if start < 0 {
		start = 0
	}
	data, err := io.ReadAll(io.NewSectionReader(f, start, maxLogBytes))
	if err != nil {
		return result, err
	}
	result.Available = true
	result.Truncated = start > 0
	if start > 0 {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		} else {
			data = nil
		}
	}
	// Ignore a concurrently-written final partial line; the next refresh reads it.
	if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
		data = data[:i+1]
	} else {
		data = nil
	}
	result.Entries = parseConnectionLogs(data)
	if len(result.Entries) > maxLogConnections {
		result.Entries = result.Entries[:maxLogConnections]
		result.Truncated = true
	}
	return result, nil
}

var logANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var logContext = regexp.MustCompile(`\[(\d+) [^\]]+\] (.*)$`)
var logComponent = regexp.MustCompile(`^(inbound|outbound)/([^\[]+)\[([^\]]+)\]: (.*)$`)
var logUser = regexp.MustCompile(`^\[([^\]]+)\] (.*)$`)

func parseConnectionLogs(data []byte) []ConnectionLog {
	entries := []ConnectionLog{}
	byID := map[string]int{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := logANSI.ReplaceAllString(raw, "")
		if strings.Contains(line, "sing-box started (") {
			byID = map[string]int{}
			continue
		}
		match := logContext.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		id := line[match[2]:match[3]]
		message := line[match[4]:match[5]]
		component := logComponent.FindStringSubmatch(message)
		isError := strings.Contains(line[:match[0]], "ERROR") || strings.Contains(line[:match[0]], "WARN")
		if component == nil && !isError {
			continue
		}
		index, exists := byID[id]
		if !exists {
			index = len(entries)
			byID[id] = index
			entries = append(entries, ConnectionLog{ID: id, Time: strings.TrimSpace(line[:match[0]]), Outbounds: []LoggedOutbound{}})
		}
		entry := &entries[index]
		if isError {
			entry.Error = message
			continue
		}
		direction, typ, tag, body := component[1], component[2], component[3], component[4]
		if direction == "inbound" {
			entry.Inbound = tag
			if user := logUser.FindStringSubmatch(body); user != nil {
				entry.User = user[1]
				body = user[2]
			}
			for _, prefix := range []string{"inbound connection from ", "inbound packet connection from "} {
				if strings.HasPrefix(body, prefix) {
					entry.Source = strings.TrimPrefix(body, prefix)
				}
			}
			for _, prefix := range []string{"inbound connection to ", "inbound packet connection to "} {
				if strings.HasPrefix(body, prefix) {
					entry.Target = strings.TrimPrefix(body, prefix)
				}
			}
		} else {
			for _, prefix := range []string{"outbound connection to ", "outbound packet connection to "} {
				if strings.HasPrefix(body, prefix) {
					outbound := LoggedOutbound{Type: typ, Tag: tag, Target: strings.TrimPrefix(body, prefix)}
					if len(entry.Outbounds) == 0 || entry.Outbounds[len(entry.Outbounds)-1] != outbound {
						entry.Outbounds = append(entry.Outbounds, outbound)
					}
				}
			}
		}
	}
	// Most recently opened connections first; log time may be elapsed process time.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

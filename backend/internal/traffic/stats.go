package traffic

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Stat is one V2Ray stats counter.
type Stat struct {
	Name  string
	Value int64
}

// StatsClient queries cumulative-since-last-reset counters. Abstracted so the
// poller can be tested against a fake.
type StatsClient interface {
	QueryStats(ctx context.Context) ([]Stat, error)
	Close() error
}

// --- minimal protobuf wire encode/decode for the two tiny messages ---

func encodeQueryStatsRequest(pattern string, reset bool) []byte {
	out := []byte{0x0a, byte(len(pattern))}
	out = append(out, []byte(pattern)...)
	if reset {
		out = append(out, 0x10, 0x01)
	}
	return out
}

func decodeVarint(b []byte, i int) (uint64, int) {
	var x uint64
	var s uint
	for ; i < len(b); i++ {
		c := b[i]
		x |= uint64(c&0x7f) << s
		if c < 0x80 {
			return x, i + 1
		}
		s += 7
	}
	return x, i
}

// decodeQueryStatsResponse parses repeated Stat stat = 1 (each an embedded msg).
func decodeQueryStatsResponse(b []byte) ([]Stat, error) {
	stats := []Stat{}
	i := 0
	for i < len(b) {
		tag := b[i]
		i++
		if tag != 0x0a { // field 1, length-delimited
			return nil, fmt.Errorf("unexpected tag 0x%x", tag)
		}
		l, ni := decodeVarint(b, i)
		i = ni
		inner := b[i : i+int(l)]
		i += int(l)
		stats = append(stats, decodeStat(inner))
	}
	return stats, nil
}

func decodeStat(b []byte) Stat {
	var s Stat
	i := 0
	for i < len(b) {
		tag := b[i]
		i++
		switch tag {
		case 0x0a: // name, length-delimited
			l, ni := decodeVarint(b, i)
			i = ni
			s.Name = string(b[i : i+int(l)])
			i += int(l)
		case 0x10: // value, varint
			v, ni := decodeVarint(b, i)
			i = ni
			s.Value = int64(v)
		default:
			return s
		}
	}
	return s
}

// statsCodec marshals our two hand-rolled message types for grpc.ForceCodec.
type statsCodec struct{}

func (statsCodec) Name() string { return "v2ray-stats" }

func (statsCodec) Marshal(v any) ([]byte, error) {
	r, ok := v.(*queryStatsRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type %T", v)
	}
	return encodeQueryStatsRequest(r.pattern, r.reset), nil
}

func (statsCodec) Unmarshal(data []byte, v any) error {
	resp, ok := v.(*queryStatsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type %T", v)
	}
	stats, err := decodeQueryStatsResponse(data)
	if err != nil {
		return err
	}
	resp.stats = stats
	return nil
}

type queryStatsRequest struct {
	pattern string
	reset   bool
}
type queryStatsResponse struct {
	stats []Stat
}

// grpcStatsClient is the real client talking to sing-box's v2ray_api.
type grpcStatsClient struct{ cc *grpc.ClientConn }

// NewStatsClient dials the v2ray_api listen address (insecure, localhost).
func NewStatsClient(addr string) (StatsClient, error) {
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &grpcStatsClient{cc: cc}, nil
}

const statsMethod = "/v2ray.core.app.stats.command.StatsService/QueryStats"

func (c *grpcStatsClient) QueryStats(ctx context.Context) ([]Stat, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req := &queryStatsRequest{pattern: "user>>>", reset: true}
	resp := &queryStatsResponse{}
	if err := c.cc.Invoke(ctx, statsMethod, req, resp, grpc.ForceCodec(statsCodec{})); err != nil {
		return nil, err
	}
	return resp.stats, nil
}

func (c *grpcStatsClient) Close() error { return c.cc.Close() }

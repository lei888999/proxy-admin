package traffic

import "testing"

func TestQueryStatsRequestEncode(t *testing.T) {
	b := encodeQueryStatsRequest("user>>>", true)
	want := append([]byte{0x0a, 0x07}, []byte("user>>>")...)
	want = append(want, 0x10, 0x01)
	if string(b) != string(want) {
		t.Fatalf("encode=% x, want % x", b, want)
	}
}

func TestQueryStatsResponseDecode(t *testing.T) {
	name := "user>>>u7>>>traffic>>>uplink"
	inner := append([]byte{0x0a, byte(len(name))}, []byte(name)...)
	inner = append(inner, 0x10, 0x80, 0x08) // value 1024 as varint
	msg := append([]byte{0x0a, byte(len(inner))}, inner...)

	stats, err := decodeQueryStatsResponse(msg)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(stats) != 1 || stats[0].Name != name || stats[0].Value != 1024 {
		t.Fatalf("stats=%+v", stats)
	}
}

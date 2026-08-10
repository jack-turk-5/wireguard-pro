package nft

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/nftables/expr"
)

func rawObjFrom(t *testing.T, jsonStr string) rawObj {
	t.Helper()
	var m rawObj
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("unmarshal %s: %v", jsonStr, err)
	}
	return m
}

func TestSingleKey(t *testing.T) {
	if _, _, err := singleKey(rawObjFrom(t, `{"accept": null}`)); err != nil {
		t.Errorf("one key: unexpected error: %v", err)
	}
	if _, _, err := singleKey(rawObjFrom(t, `{}`)); err == nil {
		t.Error("zero keys: expected error, got nil")
	}
	if _, _, err := singleKey(rawObjFrom(t, `{"a": 1, "b": 2}`)); err == nil {
		t.Error("two keys: expected error, got nil")
	}
}

func TestBuildCtMatch(t *testing.T) {
	m := jsonMatch{Op: "in", Right: json.RawMessage(`["established","related"]`)}
	leftRaw := json.RawMessage(`{"key":"state"}`)

	got, err := buildCtMatch(m, leftRaw)
	if err != nil {
		t.Fatalf("buildCtMatch: %v", err)
	}

	want := []expr.Any{
		&expr.Ct{Register: 1, Key: expr.CtKeySTATE},
		&expr.Bitwise{
			SourceRegister: 1, DestRegister: 1, Len: 4,
			Mask: []byte{0x06, 0x00, 0x00, 0x00}, // established(2) | related(4), native-endian
			Xor:  []byte{0x00, 0x00, 0x00, 0x00},
		},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: []byte{0, 0, 0, 0}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildCtMatch mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestBuildCtMatchErrors(t *testing.T) {
	cases := []struct {
		name    string
		m       jsonMatch
		leftRaw string
	}{
		{"unsupported key", jsonMatch{Op: "in", Right: json.RawMessage(`["established"]`)}, `{"key":"mark"}`},
		{"unsupported op", jsonMatch{Op: "==", Right: json.RawMessage(`["established"]`)}, `{"key":"state"}`},
		{"unknown state name", jsonMatch{Op: "in", Right: json.RawMessage(`["bogus"]`)}, `{"key":"state"}`},
		{"right not a list", jsonMatch{Op: "in", Right: json.RawMessage(`"established"`)}, `{"key":"state"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := buildCtMatch(c.m, json.RawMessage(c.leftRaw)); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestIfnameCmpExprs(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		got := ifnameCmpExprs("wg0")
		want := []expr.Any{
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: append([]byte("wg0"), make([]byte, 13)...)},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("exact match mismatch:\ngot:  %#v\nwant: %#v", got, want)
		}
	})

	t.Run("wildcard prefix", func(t *testing.T) {
		got := ifnameCmpExprs("tap*")

		wantMask := make([]byte, 16)
		wantMask[0], wantMask[1], wantMask[2] = 0xff, 0xff, 0xff
		wantData := make([]byte, 16)
		copy(wantData, "tap")

		want := []expr.Any{
			&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 16, Mask: wantMask, Xor: make([]byte, 16)},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: wantData},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("wildcard mismatch:\ngot:  %#v\nwant: %#v", got, want)
		}
	})
}

func TestBuildMetaMatch(t *testing.T) {
	got, err := buildMetaMatch(jsonMatch{Op: "==", Right: json.RawMessage(`"wg0"`)}, json.RawMessage(`{"key":"iifname"}`))
	if err != nil {
		t.Fatalf("buildMetaMatch: %v", err)
	}
	want := []expr.Any{
		&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: append([]byte("wg0"), make([]byte, 13)...)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestBuildMetaMatchErrors(t *testing.T) {
	cases := []struct {
		name    string
		m       jsonMatch
		leftRaw string
	}{
		{"unsupported key", jsonMatch{Op: "==", Right: json.RawMessage(`"eth0"`)}, `{"key":"l4proto"}`},
		{"unsupported op", jsonMatch{Op: "!=", Right: json.RawMessage(`"wg0"`)}, `{"key":"iifname"}`},
		{"right not a string", jsonMatch{Op: "==", Right: json.RawMessage(`5`)}, `{"key":"iifname"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := buildMetaMatch(c.m, json.RawMessage(c.leftRaw)); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestPrefixRange(t *testing.T) {
	cases := []struct {
		name        string
		prefix      jsonPrefix
		wantStart   []byte
		wantExclEnd []byte
	}{
		{
			name:        "ipv4 /24",
			prefix:      jsonPrefix{Addr: "172.26.0.0", Len: 24},
			wantStart:   []byte{172, 26, 0, 0},
			wantExclEnd: []byte{172, 26, 1, 0},
		},
		{
			name:        "ipv4 /24 second octet boundary",
			prefix:      jsonPrefix{Addr: "172.26.15.0", Len: 24},
			wantStart:   []byte{172, 26, 15, 0},
			wantExclEnd: []byte{172, 26, 16, 0},
		},
		{
			name:        "ipv4 carry across all bytes",
			prefix:      jsonPrefix{Addr: "255.255.255.0", Len: 24},
			wantStart:   []byte{255, 255, 255, 0},
			wantExclEnd: []byte{0, 0, 0, 0}, // wraps: this is the documented half-open [start, end) boundary
		},
		{
			name:        "ipv4 /32 single host",
			prefix:      jsonPrefix{Addr: "10.0.0.5", Len: 32},
			wantStart:   []byte{10, 0, 0, 5},
			wantExclEnd: []byte{10, 0, 0, 6},
		},
		{
			// /120 leaves the entire last byte (index 15) free, spanning
			// 0x00-0xff; the exclusive upper bound is one past that, which
			// carries into byte 14 rather than incrementing byte 15.
			name:        "ipv6 /120",
			prefix:      jsonPrefix{Addr: "fd00::", Len: 120},
			wantStart:   []byte{0xfd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantExclEnd: []byte{0xfd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, exclEnd, err := prefixRange(c.prefix)
			if err != nil {
				t.Fatalf("prefixRange: %v", err)
			}
			if !reflect.DeepEqual(start, c.wantStart) {
				t.Errorf("start = %v, want %v", start, c.wantStart)
			}
			if !reflect.DeepEqual(exclEnd, c.wantExclEnd) {
				t.Errorf("exclusiveEnd = %v, want %v", exclEnd, c.wantExclEnd)
			}
		})
	}
}

func TestPrefixRangeErrors(t *testing.T) {
	cases := []jsonPrefix{
		{Addr: "not-an-ip", Len: 24},
		{Addr: "10.0.0.0", Len: 33},
		{Addr: "10.0.0.0", Len: -1},
	}
	for _, p := range cases {
		if _, _, err := prefixRange(p); err == nil {
			t.Errorf("prefixRange(%+v): expected error, got nil", p)
		}
	}
}

func TestBuildReject(t *testing.T) {
	cases := []struct {
		name    string
		r       jsonReject
		want    *expr.Reject
		wantErr bool
	}{
		{"icmp port-unreachable", jsonReject{Type: "icmp", Expr: "port-unreachable"}, &expr.Reject{Type: 0, Code: 3}, false},
		{"icmpv6 port-unreachable", jsonReject{Type: "icmpv6", Expr: "port-unreachable"}, &expr.Reject{Type: 0, Code: 4}, false},
		{"tcp reset", jsonReject{Type: "tcp", Expr: "reset"}, &expr.Reject{Type: 1}, false},
		{"bare reject unsupported", jsonReject{}, nil, true},
		{"unknown combo", jsonReject{Type: "icmpv4", Expr: "host-unreachable"}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := buildReject(c.r)
			if c.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildReject: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %#v, want %#v", got, c.want)
			}
		})
	}
}

func TestParseFamily(t *testing.T) {
	valid := []string{"ip", "ip6", "inet", "arp", "bridge", "netdev"}
	for _, f := range valid {
		if _, err := parseFamily(f); err != nil {
			t.Errorf("parseFamily(%q): unexpected error: %v", f, err)
		}
	}
	if _, err := parseFamily("bogus"); err == nil {
		t.Error("parseFamily(bogus): expected error, got nil")
	}
}

func TestChainHookAndPolicy(t *testing.T) {
	for _, h := range []string{"prerouting", "input", "forward", "output", "postrouting"} {
		if _, err := chainHook(h); err != nil {
			t.Errorf("chainHook(%q): unexpected error: %v", h, err)
		}
	}
	if _, err := chainHook("bogus"); err == nil {
		t.Error("chainHook(bogus): expected error, got nil")
	}

	for _, p := range []string{"accept", "drop"} {
		if _, err := chainPolicy(p); err != nil {
			t.Errorf("chainPolicy(%q): unexpected error: %v", p, err)
		}
	}
	if _, err := chainPolicy("bogus"); err == nil {
		t.Error("chainPolicy(bogus): expected error, got nil")
	}
}

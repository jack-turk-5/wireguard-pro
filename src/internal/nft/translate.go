//go:build linux

package nft

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// register is the single packet-data register this translator uses for
// every load+compare sequence. Nothing here needs more than one live value
// at a time, so there's no reason to manage a register allocator.
const register = 1

// parseFamily maps an nft JSON table family name to its google/nftables
// constant.
func parseFamily(s string) (nftables.TableFamily, error) {
	switch s {
	case "ip":
		return nftables.TableFamilyIPv4, nil
	case "ip6":
		return nftables.TableFamilyIPv6, nil
	case "inet":
		return nftables.TableFamilyINet, nil
	case "arp":
		return nftables.TableFamilyARP, nil
	case "bridge":
		return nftables.TableFamilyBridge, nil
	case "netdev":
		return nftables.TableFamilyNetdev, nil
	default:
		return 0, fmt.Errorf("unsupported table family %q", s)
	}
}

var chainHooks = map[string]*nftables.ChainHook{
	"prerouting":  nftables.ChainHookPrerouting,
	"input":       nftables.ChainHookInput,
	"forward":     nftables.ChainHookForward,
	"output":      nftables.ChainHookOutput,
	"postrouting": nftables.ChainHookPostrouting,
}

// chainHook maps an nft JSON chain hook name to its google/nftables
// constant.
func chainHook(name string) (*nftables.ChainHook, error) {
	h, ok := chainHooks[name]
	if !ok {
		return nil, fmt.Errorf("unsupported chain hook %q", name)
	}
	return h, nil
}

// chainPolicy maps an nft JSON chain policy name to its google/nftables
// constant.
func chainPolicy(name string) (*nftables.ChainPolicy, error) {
	var p nftables.ChainPolicy
	switch name {
	case "accept":
		p = nftables.ChainPolicyAccept
	case "drop":
		p = nftables.ChainPolicyDrop
	default:
		return nil, fmt.Errorf("unsupported chain policy %q", name)
	}
	return &p, nil
}

// buildExprs translates one rule's JSON statement list into the equivalent
// expr.Any sequence. family is the enclosing table's family string (from
// jsonRule.Family, e.g. "inet"), needed because payload matches on a
// protocol-specific field (ip/ip6) inside a dual-stack "inet" table need an
// explicit nfproto guard prepended -- nft's own compiler inserts this
// automatically when compiling the native ruleset syntax (verified by
// comparing `nft --debug=netlink` output against this translator's output
// for the same ruleset), but nothing forces a from-scratch JSON translator
// to remember it, and skipping it would let payload offsets be
// misinterpreted for the family the rule wasn't written for.
func buildExprs(conn *nftables.Conn, table *nftables.Table, family string, stmts []rawObj) ([]expr.Any, error) {
	var exprs []expr.Any

	for _, stmt := range stmts {
		key, raw, err := singleKey(stmt)
		if err != nil {
			return nil, fmt.Errorf("statement: %w", err)
		}

		switch key {
		case "match":
			var m jsonMatch
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, fmt.Errorf("match: %w", err)
			}
			built, err := buildMatch(conn, table, family, m)
			if err != nil {
				return nil, fmt.Errorf("match: %w", err)
			}
			exprs = append(exprs, built...)

		case "accept":
			exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

		case "drop":
			exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictDrop})

		case "masquerade":
			exprs = append(exprs, &expr.Masq{})

		case "counter":
			exprs = append(exprs, &expr.Counter{})

		case "reject":
			var r jsonReject
			if len(raw) > 0 && string(raw) != "null" {
				if err := json.Unmarshal(raw, &r); err != nil {
					return nil, fmt.Errorf("reject: %w", err)
				}
			}
			built, err := buildReject(r)
			if err != nil {
				return nil, fmt.Errorf("reject: %w", err)
			}
			exprs = append(exprs, built)

		default:
			return nil, fmt.Errorf("unsupported statement %q", key)
		}
	}

	return exprs, nil
}

// buildMatch dispatches a match statement to its dedicated builder based on
// the left operand's kind (ct, meta, or payload).
func buildMatch(conn *nftables.Conn, table *nftables.Table, family string, m jsonMatch) ([]expr.Any, error) {
	leftKey, leftRaw, err := singleKey(m.Left)
	if err != nil {
		return nil, fmt.Errorf("left operand: %w", err)
	}

	switch leftKey {
	case "ct":
		return buildCtMatch(m, leftRaw)
	case "meta":
		return buildMetaMatch(m, leftRaw)
	case "payload":
		return buildPayloadMatch(conn, table, family, m, leftRaw)
	default:
		return nil, fmt.Errorf("unsupported match left operand %q", leftKey)
	}
}

// ctStateBits maps nft's ct state names to the kernel bitmask values
// google/nftables exposes as named constants (expr.CtStateBit*), which in
// turn match /usr/include linux/netfilter/nf_conntrack_tuple_common.h.
var ctStateBits = map[string]uint32{
	"invalid":     expr.CtStateBitINVALID,
	"established": expr.CtStateBitESTABLISHED,
	"related":     expr.CtStateBitRELATED,
	"new":         expr.CtStateBitNEW,
	"untracked":   expr.CtStateBitUNTRACKED,
}

// buildCtMatch translates a `ct state {...}` match. Only key=state, op=in is
// supported, matching the only ct usage this project's ruleset needs.
func buildCtMatch(m jsonMatch, leftRaw json.RawMessage) ([]expr.Any, error) {
	var left jsonCt
	if err := json.Unmarshal(leftRaw, &left); err != nil {
		return nil, fmt.Errorf("ct: %w", err)
	}
	if left.Key != "state" || m.Op != "in" {
		return nil, fmt.Errorf("unsupported ct match: key=%q op=%q (only key=state op=in is supported)", left.Key, m.Op)
	}

	var names []string
	if err := json.Unmarshal(m.Right, &names); err != nil {
		return nil, fmt.Errorf("ct state: right operand must be a list of state names: %w", err)
	}

	var mask uint32
	for _, name := range names {
		bit, ok := ctStateBits[name]
		if !ok {
			return nil, fmt.Errorf("unsupported ct state name %q", name)
		}
		mask |= bit
	}

	// Matches google/nftables' own "ct state established,related accept"
	// test case exactly: load ct state, mask off the bits we care about,
	// then check the result is nonzero (i.e. any of them are set) --
	// nft has no direct "bitmask has any of these bits" comparison op, so
	// it's built from bitwise-AND + not-equal-zero.
	return []expr.Any{
		&expr.Ct{Register: register, Key: expr.CtKeySTATE},
		&expr.Bitwise{
			SourceRegister: register,
			DestRegister:   register,
			Len:            4,
			Mask:           binaryutil.NativeEndian.PutUint32(mask),
			Xor:            binaryutil.NativeEndian.PutUint32(0),
		},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: register, Data: []byte{0, 0, 0, 0}},
	}, nil
}

var metaKeys = map[string]expr.MetaKey{
	"iifname": expr.MetaKeyIIFNAME,
	"oifname": expr.MetaKeyOIFNAME,
}

// buildMetaMatch translates a `meta {iifname,oifname} == "..."` match. Only
// == is supported, matching the only meta usage this project's ruleset
// needs.
func buildMetaMatch(m jsonMatch, leftRaw json.RawMessage) ([]expr.Any, error) {
	var left jsonMeta
	if err := json.Unmarshal(leftRaw, &left); err != nil {
		return nil, fmt.Errorf("meta: %w", err)
	}
	metaKey, ok := metaKeys[left.Key]
	if !ok {
		return nil, fmt.Errorf("unsupported meta key %q", left.Key)
	}
	if m.Op != "==" {
		return nil, fmt.Errorf("unsupported meta op %q (only == is supported)", m.Op)
	}

	var name string
	if err := json.Unmarshal(m.Right, &name); err != nil {
		return nil, fmt.Errorf("meta %s: right operand must be a string: %w", left.Key, err)
	}

	exprs := []expr.Any{&expr.Meta{Key: metaKey, Register: register}}
	return append(exprs, ifnameCmpExprs(name)...), nil
}

// ifnameIFNAMSIZ is IFNAMSIZ, the fixed kernel buffer width for an
// interface name that expr.Meta{Key: MetaKeyIIFNAME/OIFNAME} always loads,
// regardless of the destination register number used.
const ifnameIFNAMSIZ = 16

// ifnameCmpExprs builds the comparison expr(s) for an interface-name match,
// following an exact-match vs prefix-wildcard name.
//
// This was verified the hard way: a naive short Cmp (comparing only the
// prefix's bytes, as nft's own `--debug=netlink` pretty-printer appears to
// show for a compiled "tap*" match) looked byte-identical to what nft's
// compiler produces, but actually applying it and sending real traffic
// through a forward chain proved it never matches -- 0 packets/0 bytes on an
// explicit counter, with policy accept, so it wasn't a policy/return-path
// artifact. Exact-match comparisons using the same short-Cmp style (the
// full 16-byte case below) tested correctly in the same rig, so the bug is
// specific to using a truncated Cmp against a wide (16-byte / multi-word)
// field.
//
// The fix mirrors the exact same pattern this project's ruleset already
// uses for ct state matching (see buildCtMatch): load the full field, mask
// off the bytes that shouldn't matter via Bitwise, then Cmp the full
// width -- rather than trying to get a short Cmp to imply truncation on a
// field wider than one register. Verified against real traffic in a
// three-namespace forward-chain rig (a counter-only rule with policy
// accept, so match state can't be confused with policy/return-path
// effects): a rule built this way for oifname "tap*" correctly counted
// packets egressing a real "tap9" interface, where the naive short-Cmp
// version counted zero.
func ifnameCmpExprs(name string) []expr.Any {
	prefix, isWildcard := strings.CutSuffix(name, "*")
	if !isWildcard {
		b := make([]byte, ifnameIFNAMSIZ)
		copy(b, name)
		return []expr.Any{&expr.Cmp{Op: expr.CmpOpEq, Register: register, Data: b}}
	}

	mask := make([]byte, ifnameIFNAMSIZ)
	for i := range prefix {
		mask[i] = 0xff
	}
	want := make([]byte, ifnameIFNAMSIZ)
	copy(want, prefix)

	return []expr.Any{
		&expr.Bitwise{
			SourceRegister: register,
			DestRegister:   register,
			Len:            ifnameIFNAMSIZ,
			Mask:           mask,
			Xor:            make([]byte, ifnameIFNAMSIZ),
		},
		&expr.Cmp{Op: expr.CmpOpEq, Register: register, Data: want},
	}
}

// nfproto maps an nft JSON payload protocol name to its NFPROTO_* constant.
func nfproto(protocol string) (byte, error) {
	switch protocol {
	case "ip":
		return unix.NFPROTO_IPV4, nil
	case "ip6":
		return unix.NFPROTO_IPV6, nil
	default:
		return 0, fmt.Errorf("unsupported payload protocol %q", protocol)
	}
}

// payloadOffsetLen returns the network-header byte offset and length for a
// payload field, per the fixed IPv4/IPv6 header layouts (RFC 791 / RFC
// 8200): IPv4 addresses sit at bytes 12 (saddr) and 16 (daddr); IPv6
// addresses sit at bytes 8 (saddr) and 24 (daddr), each 4 or 16 bytes long
// respectively. Offset 16/len 4 for ip daddr is confirmed against
// `nft --debug=netlink`'s "payload load 4b @ network header + 16" output.
func payloadOffsetLen(protocol, field string) (offset, length uint32, err error) {
	switch {
	case protocol == "ip" && field == "saddr":
		return 12, 4, nil
	case protocol == "ip" && field == "daddr":
		return 16, 4, nil
	case protocol == "ip6" && field == "saddr":
		return 8, 16, nil
	case protocol == "ip6" && field == "daddr":
		return 24, 16, nil
	default:
		return 0, 0, fmt.Errorf("unsupported payload field %q for protocol %q", field, protocol)
	}
}

// buildPayloadMatch translates an `ip`/`ip6` `saddr`/`daddr` match against
// either a literal address (exact Cmp) or a prefix set (anonymous interval
// set lookup, see buildPrefixSetLookup). Only == is supported.
func buildPayloadMatch(conn *nftables.Conn, table *nftables.Table, family string, m jsonMatch, leftRaw json.RawMessage) ([]expr.Any, error) {
	var left jsonPayload
	if err := json.Unmarshal(leftRaw, &left); err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	if m.Op != "==" {
		return nil, fmt.Errorf("unsupported payload op %q (only == is supported)", m.Op)
	}

	offset, length, err := payloadOffsetLen(left.Protocol, left.Field)
	if err != nil {
		return nil, err
	}

	var exprs []expr.Any

	// See buildExprs' doc comment: an "inet" table is dual-stack, so a
	// protocol-specific payload field needs an explicit guard restricting
	// the rule to that protocol family, or these fixed header offsets get
	// applied to packets they were never written for.
	if family == "inet" {
		proto, err := nfproto(left.Protocol)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs,
			&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: register},
			&expr.Cmp{Op: expr.CmpOpEq, Register: register, Data: []byte{proto}},
		)
	}

	exprs = append(exprs, &expr.Payload{
		DestRegister: register,
		Base:         expr.PayloadBaseNetworkHeader,
		Offset:       offset,
		Len:          length,
	})

	// Right is either a single address (exact match) or {"set": [...]}
	// (anonymous set membership, used for the prefix-list reject rule).
	// Each set element is itself a "one of" object -- {"prefix": {...}} is
	// the only shape this translator understands; anything else (a bare
	// literal, a "range", a concatenation, ...) errors out rather than
	// being silently skipped.
	var setSpec struct {
		Set []rawObj `json:"set"`
	}
	if err := json.Unmarshal(m.Right, &setSpec); err == nil && setSpec.Set != nil {
		prefixes := make([]jsonPrefix, 0, len(setSpec.Set))
		for _, elem := range setSpec.Set {
			elemKey, elemRaw, err := singleKey(elem)
			if err != nil {
				return nil, fmt.Errorf("set element: %w", err)
			}
			if elemKey != "prefix" {
				return nil, fmt.Errorf("unsupported set element kind %q (only \"prefix\" is supported)", elemKey)
			}
			var p jsonPrefix
			if err := json.Unmarshal(elemRaw, &p); err != nil {
				return nil, fmt.Errorf("set element prefix: %w", err)
			}
			prefixes = append(prefixes, p)
		}

		lookup, err := buildPrefixSetLookup(conn, table, left.Protocol, prefixes)
		if err != nil {
			return nil, err
		}
		return append(exprs, lookup), nil
	}

	var addr string
	if err := json.Unmarshal(m.Right, &addr); err != nil {
		return nil, fmt.Errorf("payload %s.%s: right operand must be an address string or a prefix set: %w", left.Protocol, left.Field, err)
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return nil, fmt.Errorf("payload %s.%s: invalid address %q", left.Protocol, left.Field, addr)
	}
	data := ip.To4()
	if left.Protocol == "ip6" {
		data = ip.To16()
	}
	if data == nil {
		return nil, fmt.Errorf("payload %s.%s: address %q doesn't match protocol", left.Protocol, left.Field, addr)
	}

	return append(exprs, &expr.Cmp{Op: expr.CmpOpEq, Register: register, Data: data}), nil
}

// buildPrefixSetLookup creates an anonymous, constant, interval set holding
// one range per CIDR prefix, and returns the Lookup expr referencing it.
//
// Anonymous sets must be created in the same batch as the rule that
// references them -- satisfied here since conn.AddSet + conn.AddRule both
// happen before the single conn.Flush() in nft.go's caller. Verified by
// reproducing this exact requirement: an anonymous (or even named) interval
// set added in its own batch with no referencing rule is rejected by the
// kernel at commit time with EINVAL.
//
// Each range is two elements -- a start (Key) and an exclusive upper bound
// (Key, IntervalEnd: true) -- not the seemingly-equivalent single-element
// SetElement{Key: start, KeyEnd: end} shorthand. That shorthand is what
// google/nftables' own real-kernel-tested example uses, but that example is
// for a *concatenated* set type (multiple fields as one key); tested against
// a real kernel here, it fails the NEWSETELEM step with EINVAL for a plain
// (non-concatenated) interval set like this one. The two-element form,
// confirmed against the same kernel, works.
func buildPrefixSetLookup(conn *nftables.Conn, table *nftables.Table, protocol string, prefixes []jsonPrefix) (expr.Any, error) {
	keyType := nftables.TypeIPAddr
	if protocol == "ip6" {
		keyType = nftables.TypeIP6Addr
	}

	set := &nftables.Set{
		Table:     table,
		Anonymous: true,
		Constant:  true,
		Interval:  true,
		KeyType:   keyType,
	}

	elements := make([]nftables.SetElement, 0, len(prefixes)*2)
	for _, p := range prefixes {
		start, exclusiveEnd, err := prefixRange(p)
		if err != nil {
			return nil, err
		}
		elements = append(elements,
			nftables.SetElement{Key: start},
			nftables.SetElement{Key: exclusiveEnd, IntervalEnd: true},
		)
	}

	if err := conn.AddSet(set, elements); err != nil {
		return nil, fmt.Errorf("add anonymous set: %w", err)
	}

	return &expr.Lookup{SourceRegister: register, SetID: set.ID, SetName: set.Name}, nil
}

// prefixRange computes [network address, exclusive upper bound] for a CIDR
// prefix, for use as a pair of nftables interval set elements (see
// buildPrefixSetLookup). The upper bound is network|^mask (the broadcast
// address) plus one, since nft interval sets are half-open: [start, end).
func prefixRange(p jsonPrefix) (start, exclusiveEnd []byte, err error) {
	ip := net.ParseIP(p.Addr)
	if ip == nil {
		return nil, nil, fmt.Errorf("invalid prefix address %q", p.Addr)
	}
	v4 := ip.To4()
	bits := 32
	addr := []byte(v4)
	if v4 == nil {
		bits = 128
		addr = []byte(ip.To16())
		if addr == nil {
			return nil, nil, fmt.Errorf("invalid prefix address %q", p.Addr)
		}
	}
	if p.Len < 0 || p.Len > bits {
		return nil, nil, fmt.Errorf("invalid prefix length /%d for %q", p.Len, p.Addr)
	}

	mask := net.CIDRMask(p.Len, bits)
	network := make([]byte, len(addr))
	upper := make([]byte, len(addr))
	for i := range addr {
		network[i] = addr[i] & mask[i]
		upper[i] = addr[i] | ^mask[i]
	}
	for i := len(upper) - 1; i >= 0; i-- {
		upper[i]++
		if upper[i] != 0 {
			break
		}
	}

	return network, upper, nil
}

// icmpPortUnreachableCode/icmpv6PortUnreachableCode are the raw ICMP/ICMPv6
// destination-unreachable codes for "port unreachable" (RFC 792 / RFC 4443)
// -- there's no golang.org/x/sys/unix constant for either (that package
// covers syscall-level constants, not ICMP wire codes). Verified against
// real rules via `nft --debug=netlink`: both use the same family-generic
// `reject type 0 (NFT_REJECT_ICMP_UNREACH)`, with the kernel picking
// ICMP vs ICMPv6 wire encoding from the packet's actual protocol at
// evaluation time -- only the code differs (3 for IPv4, 4 for IPv6).
const (
	icmpPortUnreachableCode   = 3
	icmpv6PortUnreachableCode = 4
)

// buildReject translates a `reject` statement. Only the combinations this
// project's ruleset uses (icmp/icmpv6 port-unreachable, tcp reset) are
// supported.
func buildReject(r jsonReject) (expr.Any, error) {
	switch {
	case r.Type == "icmp" && r.Expr == "port-unreachable":
		return &expr.Reject{Type: unix.NFT_REJECT_ICMP_UNREACH, Code: icmpPortUnreachableCode}, nil
	case r.Type == "icmpv6" && r.Expr == "port-unreachable":
		return &expr.Reject{Type: unix.NFT_REJECT_ICMP_UNREACH, Code: icmpv6PortUnreachableCode}, nil
	case r.Type == "tcp" && r.Expr == "reset":
		return &expr.Reject{Type: unix.NFT_REJECT_TCP_RST}, nil
	default:
		// Includes the bare `reject` (no type/expr) case: nft's default
		// there depends on address family in a way not verified here, so
		// it's left unsupported rather than guessed at.
		return nil, fmt.Errorf("unsupported reject type=%q expr=%q", r.Type, r.Expr)
	}
}

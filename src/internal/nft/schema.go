package nft

import (
	"encoding/json"
	"fmt"
)

// document is the top-level shape of an nft JSON ruleset, as produced by
// `nft -j list ruleset` and consumed by `nft -j -f`. See
// libnftables-json(5). Each entry in Nftables is a "one of" object --
// exactly one of "table"/"chain"/"rule"/"metainfo"/etc is present -- which
// is why it's decoded as a raw map rather than a struct: unlike a typed
// struct with optional pointer fields, this lets us tell "key present with
// a JSON null value" (e.g. {"accept": null}) apart from "key absent",
// which a pointer field can't distinguish (both unmarshal to nil).
type document struct {
	Nftables []rawObj `json:"nftables"`
}

type rawObj map[string]json.RawMessage

// singleKey returns the lone key/value of a "one of" JSON object.
func singleKey(m rawObj) (string, json.RawMessage, error) {
	if len(m) != 1 {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		return "", nil, fmt.Errorf("expected exactly one key, got %d %v", len(m), keys)
	}
	for k, v := range m {
		return k, v, nil
	}
	panic("unreachable")
}

type jsonTable struct {
	Family string `json:"family"`
	Name   string `json:"name"`
}

type jsonChain struct {
	Family string `json:"family"`
	Table  string `json:"table"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Hook   string `json:"hook"`
	Prio   int32  `json:"prio"`
	Policy string `json:"policy"`
}

type jsonRule struct {
	Family string   `json:"family"`
	Table  string   `json:"table"`
	Chain  string   `json:"chain"`
	Expr   []rawObj `json:"expr"`
}

type jsonMatch struct {
	Op    string          `json:"op"`
	Left  rawObj          `json:"left"`
	Right json.RawMessage `json:"right"`
}

type jsonReject struct {
	Type string `json:"type"`
	Expr string `json:"expr"`
}

type jsonMeta struct {
	Key string `json:"key"`
}

type jsonCt struct {
	Key string `json:"key"`
}

type jsonPayload struct {
	Protocol string `json:"protocol"`
	Field    string `json:"field"`
}

type jsonPrefix struct {
	Addr string `json:"addr"`
	Len  int    `json:"len"`
}

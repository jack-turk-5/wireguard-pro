//go:build linux

// Package nft applies an nftables ruleset described in libnftables' JSON
// schema (see libnftables-json(5); `nft -j list ruleset` produces it,
// `nft -j -f` consumes it) directly via netlink, using google/nftables --
// no shelling out to nft(8).
//
// The ruleset stays a plain, operator-editable file mounted into the
// container (its path is configured, not baked into the binary); only the
// serialization changed from nft's native curly-brace syntax to this JSON
// form, because the native syntax has no Go-native parser -- reimplementing
// it would mean rebuilding a meaningful chunk of nft's own grammar, whereas
// the JSON form is genuinely JSON and decodes with encoding/json.
//
// This translates the specific, bounded vocabulary of JSON constructs this
// project's ruleset actually uses (table/chain declarations, ct state
// matching, meta iifname/oifname matching, payload ip/ip6 daddr matching
// against a literal address or a prefix set, and the accept/reject/
// masquerade/drop statements) -- not the full nft JSON schema. An
// unrecognized construct fails the apply with a clear error naming what
// wasn't understood, rather than silently misapplying the ruleset.
package nft

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/nftables"
)

// tableKey/chainKey disambiguate declarations across table families -- see
// buildRuleset.
func tableKey(family, name string) string        { return family + "/" + name }
func chainKey(family, table, name string) string { return family + "/" + table + "/" + name }

// Apply reads the JSON ruleset at path and applies it to the kernel via
// netlink, replacing whatever ruleset is currently loaded (mirroring the
// `flush ruleset` that headed the original nftables.conf, so re-applying on
// every restart is idempotent rather than accumulating stale rules from a
// previous version of the file).
func Apply(path string) error {
	conn, err := nftables.New()
	if err != nil {
		return fmt.Errorf("nft: open netlink connection: %w", err)
	}
	return applyFile(conn, path)
}

// applyFile is Apply's logic over an already-opened connection, split out
// so tests can inject a connection scoped to a disposable test network
// namespace (see newSystemConn) instead of the current process's real one.
func applyFile(conn *nftables.Conn, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("nft: read %s: %w", path, err)
	}

	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("nft: parse %s: %w", path, err)
	}

	conn.FlushRuleset()

	if err := buildRuleset(conn, doc); err != nil {
		return fmt.Errorf("nft: %s: %w", path, err)
	}

	if err := conn.Flush(); err != nil {
		return fmt.Errorf("nft: apply %s: %w", path, err)
	}
	return nil
}

// buildRuleset walks doc's top-level entries in file order, staging each
// table/chain/rule declaration onto conn (nftables.Conn batches everything
// until Flush). Tables and chains must be declared before anything that
// references them, matching how `nft -j list ruleset` always orders output.
func buildRuleset(conn *nftables.Conn, doc document) error {
	tables := map[string]*nftables.Table{}
	chains := map[string]*nftables.Chain{}

	for i, obj := range doc.Nftables {
		key, raw, err := singleKey(obj)
		if err != nil {
			return fmt.Errorf("entry %d: %w", i, err)
		}

		switch key {
		case "metainfo":
			// Informational only (nft version that produced a `-j list
			// ruleset` dump); nothing to apply.
			continue

		case "table":
			var t jsonTable
			if err := json.Unmarshal(raw, &t); err != nil {
				return fmt.Errorf("table: %w", err)
			}
			family, err := parseFamily(t.Family)
			if err != nil {
				return fmt.Errorf("table %s: %w", t.Name, err)
			}
			table := conn.AddTable(&nftables.Table{Family: family, Name: t.Name})
			tables[tableKey(t.Family, t.Name)] = table

		case "chain":
			var c jsonChain
			if err := json.Unmarshal(raw, &c); err != nil {
				return fmt.Errorf("chain: %w", err)
			}
			table, ok := tables[tableKey(c.Family, c.Table)]
			if !ok {
				return fmt.Errorf("chain %s: table %s/%s not declared before this chain", c.Name, c.Family, c.Table)
			}

			chain := &nftables.Chain{Table: table, Name: c.Name}
			if c.Type != "" {
				hook, err := chainHook(c.Hook)
				if err != nil {
					return fmt.Errorf("chain %s: %w", c.Name, err)
				}
				policy, err := chainPolicy(c.Policy)
				if err != nil {
					return fmt.Errorf("chain %s: %w", c.Name, err)
				}
				chain.Type = nftables.ChainType(c.Type)
				chain.Hooknum = hook
				chain.Priority = nftables.ChainPriorityRef(nftables.ChainPriority(c.Prio))
				chain.Policy = policy
			}

			conn.AddChain(chain)
			chains[chainKey(c.Family, c.Table, c.Name)] = chain

		case "rule":
			var r jsonRule
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("rule: %w", err)
			}
			table, ok := tables[tableKey(r.Family, r.Table)]
			if !ok {
				return fmt.Errorf("rule in %s/%s: table not declared before this rule", r.Family, r.Table)
			}
			chain, ok := chains[chainKey(r.Family, r.Table, r.Chain)]
			if !ok {
				return fmt.Errorf("rule in %s/%s/%s: chain not declared before this rule", r.Family, r.Table, r.Chain)
			}

			exprs, err := buildExprs(conn, table, r.Family, r.Expr)
			if err != nil {
				return fmt.Errorf("rule in %s/%s/%s: %w", r.Family, r.Table, r.Chain, err)
			}
			conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: exprs})

		default:
			return fmt.Errorf("entry %d: unsupported top-level object %q", i, key)
		}
	}

	return nil
}

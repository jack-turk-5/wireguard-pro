package nft

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

// TestApply_ProductionRuleset applies this project's actual mounted
// ruleset file (container/nftables.json) against a real (disposable) kernel
// network namespace. This is the regression test for both bugs found while
// building this translator by hand: a naive short Cmp not actually
// implementing interface-name wildcard matching (fixed in ifnameCmpExprs),
// and the anonymous prefix-set's element encoding being rejected by the
// kernel with EINVAL (fixed in buildPrefixSetLookup/prefixRange). Both were
// only caught by applying for real, not by unit-testing the expr.Any
// construction in isolation -- see those functions' doc comments for how
// each was actually verified (the wildcard fix needed real traffic through
// a forward chain across real namespaces, which is out of scope for this
// automated suite; the set-encoding fix is fully covered here since it's a
// pure apply-and-inspect check).
func TestApply_ProductionRuleset(t *testing.T) {
	conn := newSystemConn(t)

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "container", "nftables.json")

	if err := applyFile(conn, path); err != nil {
		t.Fatalf("apply %s: %v", path, err)
	}

	tables, err := conn.ListTables()
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	var names []string
	for _, tbl := range tables {
		names = append(names, tbl.Name)
	}
	wantTables := map[string]bool{"filter": false, "nat": false}
	for _, n := range names {
		if _, ok := wantTables[n]; ok {
			wantTables[n] = true
		}
	}
	for name, found := range wantTables {
		if !found {
			t.Errorf("expected table %q to exist after apply, tables present: %v", name, names)
		}
	}
}

// TestAnonymousSet_RequiresReferencingRuleInSameBatch encodes a real kernel
// behavior discovered the hard way: an anonymous set committed without any
// rule referencing it in the same batch is rejected with EINVAL, even
// though the exact same set (same flags, same elements) succeeds when a
// referencing rule is added before Flush. This isn't specific to our
// translator -- it's a property of anonymous nftables sets in general --
// but it's exactly the kind of thing a from-scratch JSON translator can get
// wrong by committing a set before its rule for any reason, so it's worth
// pinning down as a standing regression test.
func TestAnonymousSet_RequiresReferencingRuleInSameBatch(t *testing.T) {
	t.Run("set alone fails", func(t *testing.T) {
		conn := newSystemConn(t)
		table := conn.AddTable(&nftables.Table{Family: nftables.TableFamilyIPv4, Name: "filter"})

		set := &nftables.Set{Table: table, Anonymous: true, Constant: true, KeyType: nftables.TypeInetService}
		if err := conn.AddSet(set, []nftables.SetElement{{Key: []byte{0, 80}}}); err != nil {
			t.Fatalf("AddSet: %v", err)
		}

		if err := conn.Flush(); err == nil {
			t.Fatal("expected Flush to fail for an anonymous set with no referencing rule in the same batch, got nil error")
		}
	})

	t.Run("set plus referencing rule succeeds", func(t *testing.T) {
		conn := newSystemConn(t)
		table := conn.AddTable(&nftables.Table{Family: nftables.TableFamilyIPv4, Name: "filter"})
		chain := conn.AddChain(&nftables.Chain{
			Table: table, Name: "forward", Type: nftables.ChainTypeFilter,
			Hooknum: nftables.ChainHookForward, Priority: nftables.ChainPriorityFilter,
		})

		set := &nftables.Set{Table: table, Anonymous: true, Constant: true, KeyType: nftables.TypeInetService}
		if err := conn.AddSet(set, []nftables.SetElement{{Key: []byte{0, 80}}}); err != nil {
			t.Fatalf("AddSet: %v", err)
		}

		conn.AddRule(&nftables.Rule{
			Table: table,
			Chain: chain,
			Exprs: []expr.Any{
				&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2},
				&expr.Lookup{SourceRegister: 1, SetID: set.ID, SetName: set.Name},
				&expr.Verdict{Kind: expr.VerdictDrop},
			},
		})

		if err := conn.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
	})
}

// TestBuildPrefixSetLookup_IntervalEncoding is the regression test for the
// element-encoding bug: the single-element SetElement{Key, KeyEnd} shorthand
// (which is what google/nftables' own real-kernel-tested example uses, but
// only for a *concatenated* set type) is rejected by this kernel with EINVAL
// on NFT_MSG_NEWSETELEM for a plain (non-concatenated) interval set. The
// fix -- two elements per range, a start and an IntervalEnd-flagged
// exclusive upper bound -- is exercised here through the same code path
// production uses (buildPrefixSetLookup), applied for real and read back to
// confirm the element count and shape.
func TestBuildPrefixSetLookup_IntervalEncoding(t *testing.T) {
	conn := newSystemConn(t)
	table := conn.AddTable(&nftables.Table{Family: nftables.TableFamilyIPv4, Name: "filter"})
	chain := conn.AddChain(&nftables.Chain{
		Table: table, Name: "forward", Type: nftables.ChainTypeFilter,
		Hooknum: nftables.ChainHookForward, Priority: nftables.ChainPriorityFilter,
	})

	prefixes := []jsonPrefix{
		{Addr: "172.26.0.0", Len: 24},
		{Addr: "172.26.15.0", Len: 24},
	}
	lookup, err := buildPrefixSetLookup(conn, table, "ip", prefixes)
	if err != nil {
		t.Fatalf("buildPrefixSetLookup: %v", err)
	}

	conn.AddRule(&nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4},
			lookup,
			&expr.Verdict{Kind: expr.VerdictDrop},
		},
	})

	if err := conn.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	sets, err := conn.GetSets(table)
	if err != nil {
		t.Fatalf("GetSets: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("len(sets) = %d, want 1", len(sets))
	}

	elements, err := conn.GetSetElements(sets[0])
	if err != nil {
		t.Fatalf("GetSetElements: %v", err)
	}
	// Two disjoint /24s, half-open [start, end) representation: 4 raw
	// elements (start + exclusive-end marker per range) -- not 2, which is
	// what the rejected single-element-per-range encoding would have
	// produced.
	if len(elements) != 4 {
		t.Errorf("len(elements) = %d, want 4 (2 ranges x [start, end) markers)", len(elements))
	}
}

package akg

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTagIndexCollisionCompactsCleanly is the SDK-level regression for the
// tag-index key collision (EPIC-18-003). Two nodes share the id "preflop__vpip"
// across the types "counter" and "tendency" and both carry the tag "preflop".
// Under the major-1 tag key (t:{tag}:{id}) both collapsed to one key and
// compaction failed with "duplicate data key"; the major-2 type-qualified key
// (t:{tag}:{type}:{id}) keeps them distinct, so commit + compact succeed and both
// nodes remain independently resolvable.
func TestTagIndexCollisionCompactsCleanly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collision.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if _, err := st.PutNode("counter", "preflop__vpip", NodeFields{Title: "VPIP counter"}, []string{"preflop"}); err != nil {
		t.Fatalf("PutNode counter: %v", err)
	}
	if _, err := st.PutNode("tendency", "preflop__vpip", NodeFields{Title: "VPIP tendency"}, []string{"preflop"}); err != nil {
		t.Fatalf("PutNode tendency: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact (regression — was 'duplicate data key'): %v", err)
	}

	counter, err := st.GetNode("counter", "preflop__vpip")
	if err != nil || counter == nil {
		t.Fatalf("GetNode counter: node=%v err=%v", counter, err)
	}
	tendency, err := st.GetNode("tendency", "preflop__vpip")
	if err != nil || tendency == nil {
		t.Fatalf("GetNode tendency: node=%v err=%v", tendency, err)
	}
	if counter.Title != "VPIP counter" || tendency.Title != "VPIP tendency" {
		t.Fatalf("titles not distinct: counter=%q tendency=%q", counter.Title, tendency.Title)
	}

	byTag, err := st.ListNodesByTag("preflop")
	if err != nil {
		t.Fatalf("ListNodesByTag: %v", err)
	}
	if len(byTag) != 2 {
		t.Fatalf("ListNodesByTag(preflop): got %d nodes, want 2", len(byTag))
	}
}

// TestReadCompatMajor1UpgradesOnCompaction proves the major-2 read-compat
// contract end to end: a major-1 file (3-part tag key t:{tag}:{id}) opens and
// reads clean under the major-2 reader, and compaction rewrites it as a major-2
// file whose tag index uses the type-qualified 4-part key (t:{tag}:{type}:{id}).
func TestReadCompatMajor1UpgradesOnCompaction(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "conformance", "m2-compacted.akg")
	orig, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if orig[4] != 1 {
		t.Fatalf("fixture precondition: want major 1, got %d", orig[4])
	}

	path := filepath.Join(t.TempDir(), "compacted.akg")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open major-1 fixture (read-compat): %v", err)
	}
	// The fixture's single node n:note:live carries the tag "current" (v1 key
	// t:current:live). Read-compat must surface it before compaction.
	before, err := st.ListNodesByTag("current")
	if err != nil || len(before) != 1 || before[0].ID != "live" {
		t.Fatalf("read-compat tag listing before compact: nodes=%#v err=%v", before, err)
	}
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact major-1 file: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	upgraded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read compacted file: %v", err)
	}
	if upgraded[4] != currentMajor {
		t.Fatalf("compaction did not upgrade major: got %d, want %d", upgraded[4], currentMajor)
	}

	c, err := decodeContainer(upgraded)
	if err != nil {
		t.Fatalf("decodeContainer: %v", err)
	}
	entries, err := decodeDataEntries(c.Data)
	if err != nil {
		t.Fatalf("decodeDataEntries: %v", err)
	}
	var tagKeys int
	for _, e := range entries {
		if len(e.Key) > 2 && string(e.Key[:2]) == "t:" {
			tagKeys++
			tag, nodeType, id, perr := parseTagKey(e.Key)
			if perr != nil {
				t.Fatalf("parseTagKey(%q): %v", e.Key, perr)
			}
			if nodeType == "" {
				t.Fatalf("tag key not type-qualified after upgrade: %q", e.Key)
			}
			if tag != "current" || nodeType != "note" || id != "live" {
				t.Fatalf("unexpected upgraded tag key: tag=%q type=%q id=%q", tag, nodeType, id)
			}
		}
	}
	if tagKeys != 1 {
		t.Fatalf("expected exactly 1 tag key after upgrade, got %d", tagKeys)
	}
}

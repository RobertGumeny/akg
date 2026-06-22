package store

import (
	"bytes"
	"errors"
	"sort"

	"github.com/RobertGumeny/akg/internal/format"
	"github.com/RobertGumeny/akg/internal/keys"
	"github.com/RobertGumeny/akg/internal/record"
	"github.com/RobertGumeny/akg/internal/state"
)

var (
	ErrInvalidState             = errors.New("invalid state")
	ErrDuplicateMaterializedKey = errors.New("duplicate materialized key")
)

// MaterializeDataEntries derives the complete live AKG Data key set from the
// authoritative in-memory state at the current binary major. The returned
// entries are sorted by raw bytewise key order and contain no mutable
// derived-index state from outside s. Writers always materialize at the current
// major, so a compacted file's tag index is always type-qualified (major 2).
func MaterializeDataEntries(s *state.State) ([]format.DataEntry, error) {
	return materializeDataEntries(s, format.CurrentMajor)
}

// materializeDataEntries derives the live Data key set as it must appear in a
// file of the given binary major. major selects the tag-key shape: major 1
// emits legacy t:{tag}:{id} keys, major 2 the type-qualified t:{tag}:{type}:{id}.
// Read-side validation passes the file's own major so a major-1 file's 3-part
// tag keys validate against a major-1 materialization.
func materializeDataEntries(s *state.State, major uint8) ([]format.DataEntry, error) {
	if s == nil {
		return nil, ErrInvalidState
	}
	var entries []format.DataEntry
	seen := map[string]struct{}{}
	add := func(key, value []byte) error {
		k := string(key)
		if _, ok := seen[k]; ok {
			return ErrDuplicateMaterializedKey
		}
		seen[k] = struct{}{}
		entries = append(entries, format.DataEntry{
			Key:   append([]byte(nil), key...),
			Value: append([]byte(nil), value...),
		})
		return nil
	}

	for _, node := range s.Nodes() {
		key, err := keys.BuildNodeKey(node.Node.Type, node.ID)
		if err != nil {
			return nil, err
		}
		value, err := record.EncodeNodePayload(node.Node)
		if err != nil {
			return nil, err
		}
		if err := add(key, value); err != nil {
			return nil, err
		}
		for _, tag := range node.Node.Tags {
			tagKey, err := buildTagKeyForMajor(major, tag, node.Node.Type, node.ID)
			if err != nil {
				return nil, err
			}
			if err := add(tagKey, nil); err != nil {
				return nil, err
			}
		}
		temporalKey, err := keys.BuildTemporalNodeKey(node.Node.UpdatedAt, node.Node.Type, node.ID)
		if err != nil {
			return nil, err
		}
		if err := add(temporalKey, nil); err != nil {
			return nil, err
		}
	}

	for _, edge := range s.Edges() {
		key, err := keys.BuildEdgeKey(edge.FromType, edge.FromNode, edge.Relation, edge.ToType, edge.ToNode)
		if err != nil {
			return nil, err
		}
		value, err := record.EncodeEdgePayload(edge)
		if err != nil {
			return nil, err
		}
		if err := add(key, value); err != nil {
			return nil, err
		}
		inboundKey, err := keys.BuildEdgeIndexKey(edge.ToType, edge.ToNode, edge.Relation, edge.FromType, edge.FromNode)
		if err != nil {
			return nil, err
		}
		if err := add(inboundKey, nil); err != nil {
			return nil, err
		}
		temporalKey, err := keys.BuildTemporalEdgeKey(edge.UpdatedAt, edge.FromType, edge.FromNode, edge.Relation, edge.ToType, edge.ToNode)
		if err != nil {
			return nil, err
		}
		if err := add(temporalKey, nil); err != nil {
			return nil, err
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].Key, entries[j].Key) < 0
	})
	return entries, nil
}

// buildTagKeyForMajor builds the tag-index key in the shape required by the
// given binary major: legacy t:{tag}:{id} for major 1, type-qualified
// t:{tag}:{type}:{id} for major 2 and later.
func buildTagKeyForMajor(major uint8, tag, nodeType string, id record.NodeID) ([]byte, error) {
	if major < 2 {
		return keys.BuildTagKeyV1(tag, id)
	}
	return keys.BuildTagKey(tag, nodeType, id)
}

package store

import (
	"bytes"
	"errors"
	"sort"

	"github.com/RobertGumeny/akg-format/internal/format"
	"github.com/RobertGumeny/akg-format/internal/keys"
	"github.com/RobertGumeny/akg-format/internal/record"
	"github.com/RobertGumeny/akg-format/internal/state"
)

var (
	ErrInvalidState             = errors.New("invalid state")
	ErrDuplicateMaterializedKey = errors.New("duplicate materialized key")
)

// MaterializeDataEntries derives the complete live AKG Data key set from the
// authoritative in-memory state. The returned entries are sorted by raw bytewise
// key order and contain no mutable derived-index state from outside s.
func MaterializeDataEntries(s *state.State) ([]format.DataEntry, error) {
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
			tagKey, err := keys.BuildTagKey(tag, node.ID)
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

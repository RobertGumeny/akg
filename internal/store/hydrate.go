package store

import (
	"bytes"
	"errors"
	"strings"

	"github.com/RobertGumeny/akg-format/internal/format"
	"github.com/RobertGumeny/akg-format/internal/keys"
	"github.com/RobertGumeny/akg-format/internal/record"
	"github.com/RobertGumeny/akg-format/internal/state"
)

var (
	ErrInvalidDataEntries   = errors.New("invalid data entries")
	ErrIdentityMismatch     = errors.New("key/payload identity mismatch")
	ErrDerivedIndexMismatch = errors.New("derived index mismatch")
	ErrNonEmptyDerivedValue = errors.New("non-empty derived index value")
)

// HydrateDataEntries reconstructs authoritative live state from decoded AKG Data
// entries. Primary node/edge records are authoritative; derived index entries
// are validated against the regenerated key set and are not stored as state.
func HydrateDataEntries(entries []format.DataEntry) (*state.State, error) {
	if err := validateEntryOrder(entries); err != nil {
		return nil, err
	}

	s := state.New()
	for _, entry := range entries {
		key := entry.Key
		switch {
		case strings.HasPrefix(string(key), "n:"):
			parsed, err := keys.ParseNodeKey(key)
			if err != nil {
				return nil, err
			}
			node, err := record.DecodeNodePayload(entry.Value)
			if err != nil {
				return nil, err
			}
			if node.Type != parsed.Type {
				return nil, ErrIdentityMismatch
			}
			if err := s.LoadNodeRecord(state.NodeRecord{ID: parsed.ID, Node: node}); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "e:"):
			parsed, err := keys.ParseEdgeKey(key)
			if err != nil {
				return nil, err
			}
			edge, err := record.DecodeEdgePayload(entry.Value)
			if err != nil {
				return nil, err
			}
			if edge.FromNode != parsed.FromNode || edge.Relation != parsed.Relation || edge.ToNode != parsed.ToNode {
				return nil, ErrIdentityMismatch
			}
			if err := s.LoadEdge(edge); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "ei:"):
			if len(entry.Value) != 0 {
				return nil, ErrNonEmptyDerivedValue
			}
			if _, err := keys.ParseEdgeIndexKey(key); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "t:"):
			if len(entry.Value) != 0 {
				return nil, ErrNonEmptyDerivedValue
			}
			if _, err := keys.ParseTagKey(key); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "ts:"):
			if len(entry.Value) != 0 {
				return nil, ErrNonEmptyDerivedValue
			}
			if _, err := keys.ParseTemporalKey(key); err != nil {
				return nil, err
			}
		default:
			return nil, ErrInvalidDataEntries
		}
	}

	if err := validateDerivedKeys(s, entries); err != nil {
		return nil, err
	}
	return s, nil
}

func validateEntryOrder(entries []format.DataEntry) error {
	for i := 1; i < len(entries); i++ {
		cmp := bytes.Compare(entries[i-1].Key, entries[i].Key)
		if cmp == 0 {
			return format.ErrDuplicateDataKey
		}
		if cmp > 0 {
			return format.ErrInvalidDataSection
		}
	}
	return nil
}

func validateDerivedKeys(s *state.State, entries []format.DataEntry) error {
	expected, err := MaterializeDataEntries(s)
	if err != nil {
		return err
	}
	if len(expected) != len(entries) {
		return ErrDerivedIndexMismatch
	}
	for i := range expected {
		if !bytes.Equal(expected[i].Key, entries[i].Key) {
			return ErrDerivedIndexMismatch
		}
	}
	return nil
}

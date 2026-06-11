package keys

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/RobertGumeny/akg/internal/record"
)

const (
	prefixNode       = "n"
	prefixEdge       = "e"
	prefixEdgeIndex  = "ei"
	prefixTag        = "t"
	prefixTemporal   = "ts"
	temporalNodeKind = "n"
	temporalEdgeKind = "e"
	// maxComponentBytes caps every key component — node-id, type, relation, and
	// tag — at 64 UTF-8 bytes (spec 04:31/34/54/77, echoed 01:18/62/116). Bytes,
	// not codepoints: unambiguous and identical across language implementations.
	maxComponentBytes = 64
)

var (
	ErrMalformedKey     = errors.New("malformed key")
	ErrInvalidComponent = errors.New("invalid key component")
)

// NodeKey is the parsed identity carried by n:{type}:{id} keys.
type NodeKey struct {
	Type string
	ID   record.NodeID
}

// EdgeKey is the parsed identity carried by e:{fromType}:{fromID}:{relation}:{toType}:{toID} keys.
type EdgeKey struct {
	FromType string
	FromNode record.NodeID
	Relation record.Relation
	ToType   string
	ToNode   record.NodeID
}

// EdgeIndexKey is the parsed identity carried by ei:{toType}:{toID}:{relation}:{fromType}:{fromID} keys.
type EdgeIndexKey struct {
	ToType   string
	ToNode   record.NodeID
	Relation record.Relation
	FromType string
	FromNode record.NodeID
}

// TagKey is the parsed identity carried by t:{tag}:{node_id} keys.
type TagKey struct {
	Tag    string
	NodeID record.NodeID
}

// TemporalKey is the parsed identity carried by self-describing ts: keys.
type TemporalKey struct {
	Timestamp record.TimestampMicros
	Kind      string
	Node      NodeKey
	Edge      EdgeKey
}

func BuildNodeKey(nodeType string, id record.NodeID) ([]byte, error) {
	if err := validateComponent(nodeType); err != nil {
		return nil, err
	}
	if err := validateNodeID(id); err != nil {
		return nil, err
	}
	return []byte(prefixNode + ":" + nodeType + ":" + string(id)), nil
}

func ParseNodeKey(key []byte) (NodeKey, error) {
	parts := splitKey(key, 3)
	if parts == nil || parts[0] != prefixNode {
		return NodeKey{}, ErrMalformedKey
	}
	if err := validateComponent(parts[1]); err != nil {
		return NodeKey{}, ErrMalformedKey
	}
	id := record.NodeID(parts[2])
	if err := validateNodeID(id); err != nil {
		return NodeKey{}, ErrMalformedKey
	}
	return NodeKey{Type: parts[1], ID: id}, nil
}

func BuildEdgeKey(fromType string, from record.NodeID, relation record.Relation, toType string, to record.NodeID) ([]byte, error) {
	if err := validateComponent(fromType); err != nil {
		return nil, err
	}
	if err := validateNodeID(from); err != nil {
		return nil, err
	}
	if err := validateComponent(string(relation)); err != nil {
		return nil, err
	}
	if err := validateComponent(toType); err != nil {
		return nil, err
	}
	if err := validateNodeID(to); err != nil {
		return nil, err
	}
	return []byte(prefixEdge + ":" + fromType + ":" + string(from) + ":" + string(relation) + ":" + toType + ":" + string(to)), nil
}

func ParseEdgeKey(key []byte) (EdgeKey, error) {
	parts := splitKey(key, 6)
	if parts == nil || parts[0] != prefixEdge {
		return EdgeKey{}, ErrMalformedKey
	}
	fromType, from, relation, toType, to := parts[1], record.NodeID(parts[2]), record.Relation(parts[3]), parts[4], record.NodeID(parts[5])
	if validateComponent(fromType) != nil || validateNodeID(from) != nil || validateComponent(string(relation)) != nil || validateComponent(toType) != nil || validateNodeID(to) != nil {
		return EdgeKey{}, ErrMalformedKey
	}
	return EdgeKey{FromType: fromType, FromNode: from, Relation: relation, ToType: toType, ToNode: to}, nil
}

func BuildEdgeIndexKey(toType string, to record.NodeID, relation record.Relation, fromType string, from record.NodeID) ([]byte, error) {
	if err := validateComponent(toType); err != nil {
		return nil, err
	}
	if err := validateNodeID(to); err != nil {
		return nil, err
	}
	if err := validateComponent(string(relation)); err != nil {
		return nil, err
	}
	if err := validateComponent(fromType); err != nil {
		return nil, err
	}
	if err := validateNodeID(from); err != nil {
		return nil, err
	}
	return []byte(prefixEdgeIndex + ":" + toType + ":" + string(to) + ":" + string(relation) + ":" + fromType + ":" + string(from)), nil
}

func ParseEdgeIndexKey(key []byte) (EdgeIndexKey, error) {
	parts := splitKey(key, 6)
	if parts == nil || parts[0] != prefixEdgeIndex {
		return EdgeIndexKey{}, ErrMalformedKey
	}
	toType, to, relation, fromType, from := parts[1], record.NodeID(parts[2]), record.Relation(parts[3]), parts[4], record.NodeID(parts[5])
	if validateComponent(toType) != nil || validateNodeID(to) != nil || validateComponent(string(relation)) != nil || validateComponent(fromType) != nil || validateNodeID(from) != nil {
		return EdgeIndexKey{}, ErrMalformedKey
	}
	return EdgeIndexKey{ToType: toType, ToNode: to, Relation: relation, FromType: fromType, FromNode: from}, nil
}

func BuildTagKey(tag string, nodeID record.NodeID) ([]byte, error) {
	if err := validateTag(tag); err != nil {
		return nil, err
	}
	if err := validateNodeID(nodeID); err != nil {
		return nil, err
	}
	return []byte(prefixTag + ":" + tag + ":" + string(nodeID)), nil
}

func ParseTagKey(key []byte) (TagKey, error) {
	parts := splitKey(key, 3)
	if parts == nil || parts[0] != prefixTag {
		return TagKey{}, ErrMalformedKey
	}
	if validateTag(parts[1]) != nil {
		return TagKey{}, ErrMalformedKey
	}
	id := record.NodeID(parts[2])
	if validateNodeID(id) != nil {
		return TagKey{}, ErrMalformedKey
	}
	return TagKey{Tag: parts[1], NodeID: id}, nil
}

func BuildTemporalNodeKey(timestamp record.TimestampMicros, nodeType string, id record.NodeID) ([]byte, error) {
	nodeKey, err := BuildNodeKey(nodeType, id)
	if err != nil {
		return nil, err
	}
	return []byte(prefixTemporal + ":" + strconv.FormatUint(uint64(timestamp), 10) + ":" + string(nodeKey)), nil
}

func BuildTemporalEdgeKey(timestamp record.TimestampMicros, fromType string, from record.NodeID, relation record.Relation, toType string, to record.NodeID) ([]byte, error) {
	edgeKey, err := BuildEdgeKey(fromType, from, relation, toType, to)
	if err != nil {
		return nil, err
	}
	return []byte(prefixTemporal + ":" + strconv.FormatUint(uint64(timestamp), 10) + ":" + string(edgeKey)), nil
}

func ParseTemporalKey(key []byte) (TemporalKey, error) {
	parts := strings.Split(string(key), ":")
	if len(parts) < 4 || parts[0] != prefixTemporal {
		return TemporalKey{}, ErrMalformedKey
	}
	timestamp, err := parseCanonicalTimestamp(parts[1])
	if err != nil {
		return TemporalKey{}, ErrMalformedKey
	}
	suffix := []byte(strings.Join(parts[2:], ":"))
	switch parts[2] {
	case temporalNodeKind:
		node, err := ParseNodeKey(suffix)
		if err != nil {
			return TemporalKey{}, err
		}
		return TemporalKey{Timestamp: timestamp, Kind: temporalNodeKind, Node: node}, nil
	case temporalEdgeKind:
		edge, err := ParseEdgeKey(suffix)
		if err != nil {
			return TemporalKey{}, err
		}
		return TemporalKey{Timestamp: timestamp, Kind: temporalEdgeKind, Edge: edge}, nil
	default:
		return TemporalKey{}, ErrMalformedKey
	}
}

func splitKey(key []byte, want int) []string {
	parts := strings.Split(string(key), ":")
	if len(parts) != want {
		return nil
	}
	for _, part := range parts {
		if part == "" {
			return nil
		}
	}
	return parts
}

func validateNodeID(id record.NodeID) error {
	return validateComponent(string(id))
}

// validateComponent enforces only the format-level key-safety rules that apply
// to every component (type, relation, tag, node-id): non-empty, valid UTF-8, no
// colon delimiter, and at most 64 UTF-8 bytes (spec 01:18/62/116, 04:31/34/54/77).
// Casing and word-separation (lowercase, snake_case) are an SDK-level convention,
// not a format rule (04:80) — writers must not reject or silently correct them.
func validateComponent(value string) error {
	if value == "" || !utf8.ValidString(value) || len(value) > maxComponentBytes || strings.ContainsRune(value, ':') {
		return ErrInvalidComponent
	}
	return nil
}

func validateTag(tag string) error {
	return validateComponent(tag)
}

func parseCanonicalTimestamp(value string) (record.TimestampMicros, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, ErrInvalidComponent
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, ErrInvalidComponent
		}
	}
	u, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, ErrInvalidComponent
	}
	return record.TimestampMicros(u), nil
}

// BytewiseLess reports AKG's canonical raw byte lexicographic ordering.
func BytewiseLess(a, b []byte) bool {
	return bytes.Compare(a, b) < 0
}

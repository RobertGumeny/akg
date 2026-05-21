package keys

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/earendil-works/akg/internal/record"
)

const (
	prefixNode       = "n"
	prefixEdge       = "e"
	prefixEdgeIndex  = "ei"
	prefixTag        = "t"
	prefixTemporal   = "ts"
	temporalNodeKind = "n"
	temporalEdgeKind = "e"
	maxNodeIDLen     = 64
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

// EdgeKey is the parsed identity carried by e:{from}:{relation}:{to} keys.
type EdgeKey struct {
	FromNode record.NodeID
	Relation record.Relation
	ToNode   record.NodeID
}

// EdgeIndexKey is the parsed identity carried by ei:{to}:{relation}:{from} keys.
type EdgeIndexKey struct {
	ToNode   record.NodeID
	Relation record.Relation
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

func BuildEdgeKey(from record.NodeID, relation record.Relation, to record.NodeID) ([]byte, error) {
	if err := validateNodeID(from); err != nil {
		return nil, err
	}
	if err := validateComponent(string(relation)); err != nil {
		return nil, err
	}
	if err := validateNodeID(to); err != nil {
		return nil, err
	}
	return []byte(prefixEdge + ":" + string(from) + ":" + string(relation) + ":" + string(to)), nil
}

func ParseEdgeKey(key []byte) (EdgeKey, error) {
	parts := splitKey(key, 4)
	if parts == nil || parts[0] != prefixEdge {
		return EdgeKey{}, ErrMalformedKey
	}
	from, relation, to := record.NodeID(parts[1]), record.Relation(parts[2]), record.NodeID(parts[3])
	if validateNodeID(from) != nil || validateComponent(string(relation)) != nil || validateNodeID(to) != nil {
		return EdgeKey{}, ErrMalformedKey
	}
	return EdgeKey{FromNode: from, Relation: relation, ToNode: to}, nil
}

func BuildEdgeIndexKey(to record.NodeID, relation record.Relation, from record.NodeID) ([]byte, error) {
	if err := validateNodeID(to); err != nil {
		return nil, err
	}
	if err := validateComponent(string(relation)); err != nil {
		return nil, err
	}
	if err := validateNodeID(from); err != nil {
		return nil, err
	}
	return []byte(prefixEdgeIndex + ":" + string(to) + ":" + string(relation) + ":" + string(from)), nil
}

func ParseEdgeIndexKey(key []byte) (EdgeIndexKey, error) {
	parts := splitKey(key, 4)
	if parts == nil || parts[0] != prefixEdgeIndex {
		return EdgeIndexKey{}, ErrMalformedKey
	}
	to, relation, from := record.NodeID(parts[1]), record.Relation(parts[2]), record.NodeID(parts[3])
	if validateNodeID(to) != nil || validateComponent(string(relation)) != nil || validateNodeID(from) != nil {
		return EdgeIndexKey{}, ErrMalformedKey
	}
	return EdgeIndexKey{ToNode: to, Relation: relation, FromNode: from}, nil
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

func BuildTemporalEdgeKey(timestamp record.TimestampMicros, from record.NodeID, relation record.Relation, to record.NodeID) ([]byte, error) {
	edgeKey, err := BuildEdgeKey(from, relation, to)
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
	value := string(id)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxNodeIDLen || strings.ContainsRune(value, ':') {
		return ErrInvalidComponent
	}
	return nil
}

func validateComponent(value string) error {
	if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, ':') {
		return ErrInvalidComponent
	}
	return nil
}

func validateTag(tag string) error {
	if validateComponent(tag) != nil {
		return ErrInvalidComponent
	}
	prevUnderscore := false
	for i, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			prevUnderscore = false
		case r == '_':
			if i == 0 || prevUnderscore {
				return ErrInvalidComponent
			}
			prevUnderscore = true
		default:
			return ErrInvalidComponent
		}
	}
	if prevUnderscore {
		return ErrInvalidComponent
	}
	return nil
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

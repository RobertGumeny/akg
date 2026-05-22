package akg

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxNodeIDLen = 64

var (
	errMalformedKey     = errors.New("malformed key")
	errInvalidComponent = errors.New("invalid key component")
)

type parsedNodeKey struct {
	Type string
	ID   nodeID
}

type parsedEdgeKey struct {
	FromType string
	FromNode nodeID
	Relation relation
	ToType   string
	ToNode   nodeID
}

type parsedEdgeIndexKey struct {
	ToType   string
	ToNode   nodeID
	Relation relation
	FromType string
	FromNode nodeID
}

func buildNodeKey(nodeType string, id nodeID) ([]byte, error) {
	if err := validateComponent(nodeType); err != nil {
		return nil, err
	}
	if err := validateNodeID(id); err != nil {
		return nil, err
	}
	return []byte("n:" + nodeType + ":" + string(id)), nil
}

func parseNodeKey(key []byte) (parsedNodeKey, error) {
	parts := splitKey(key, 3)
	if parts == nil || parts[0] != "n" {
		return parsedNodeKey{}, errMalformedKey
	}
	if err := validateComponent(parts[1]); err != nil {
		return parsedNodeKey{}, errMalformedKey
	}
	id := nodeID(parts[2])
	if err := validateNodeID(id); err != nil {
		return parsedNodeKey{}, errMalformedKey
	}
	return parsedNodeKey{Type: parts[1], ID: id}, nil
}

func buildEdgeKey(fromType string, from nodeID, rel relation, toType string, to nodeID) ([]byte, error) {
	if err := validateComponent(fromType); err != nil {
		return nil, err
	}
	if err := validateNodeID(from); err != nil {
		return nil, err
	}
	if err := validateComponent(string(rel)); err != nil {
		return nil, err
	}
	if err := validateComponent(toType); err != nil {
		return nil, err
	}
	if err := validateNodeID(to); err != nil {
		return nil, err
	}
	return []byte("e:" + fromType + ":" + string(from) + ":" + string(rel) + ":" + toType + ":" + string(to)), nil
}

func parseEdgeKey(key []byte) (parsedEdgeKey, error) {
	parts := splitKey(key, 6)
	if parts == nil || parts[0] != "e" {
		return parsedEdgeKey{}, errMalformedKey
	}
	fromType, from, rel, toType, to := parts[1], nodeID(parts[2]), relation(parts[3]), parts[4], nodeID(parts[5])
	if validateComponent(fromType) != nil || validateNodeID(from) != nil || validateComponent(string(rel)) != nil || validateComponent(toType) != nil || validateNodeID(to) != nil {
		return parsedEdgeKey{}, errMalformedKey
	}
	return parsedEdgeKey{FromType: fromType, FromNode: from, Relation: rel, ToType: toType, ToNode: to}, nil
}

func buildEdgeIndexKey(toType string, to nodeID, rel relation, fromType string, from nodeID) ([]byte, error) {
	if err := validateComponent(toType); err != nil {
		return nil, err
	}
	if err := validateNodeID(to); err != nil {
		return nil, err
	}
	if err := validateComponent(string(rel)); err != nil {
		return nil, err
	}
	if err := validateComponent(fromType); err != nil {
		return nil, err
	}
	if err := validateNodeID(from); err != nil {
		return nil, err
	}
	return []byte("ei:" + toType + ":" + string(to) + ":" + string(rel) + ":" + fromType + ":" + string(from)), nil
}

func parseEdgeIndexKey(key []byte) (parsedEdgeIndexKey, error) {
	parts := splitKey(key, 6)
	if parts == nil || parts[0] != "ei" {
		return parsedEdgeIndexKey{}, errMalformedKey
	}
	toType, to, rel, fromType, from := parts[1], nodeID(parts[2]), relation(parts[3]), parts[4], nodeID(parts[5])
	if validateComponent(toType) != nil || validateNodeID(to) != nil || validateComponent(string(rel)) != nil || validateComponent(fromType) != nil || validateNodeID(from) != nil {
		return parsedEdgeIndexKey{}, errMalformedKey
	}
	return parsedEdgeIndexKey{ToType: toType, ToNode: to, Relation: rel, FromType: fromType, FromNode: from}, nil
}

func buildTagKey(tag string, id nodeID) ([]byte, error) {
	if err := validateTag(tag); err != nil {
		return nil, err
	}
	if err := validateNodeID(id); err != nil {
		return nil, err
	}
	return []byte("t:" + tag + ":" + string(id)), nil
}

func parseTagKey(key []byte) (string, nodeID, error) {
	parts := splitKey(key, 3)
	if parts == nil || parts[0] != "t" {
		return "", "", errMalformedKey
	}
	if validateTag(parts[1]) != nil {
		return "", "", errMalformedKey
	}
	id := nodeID(parts[2])
	if validateNodeID(id) != nil {
		return "", "", errMalformedKey
	}
	return parts[1], id, nil
}

func buildTemporalNodeKey(ts timestampMicros, nodeType string, id nodeID) ([]byte, error) {
	nodeKey, err := buildNodeKey(nodeType, id)
	if err != nil {
		return nil, err
	}
	return []byte("ts:" + strconv.FormatUint(uint64(ts), 10) + ":" + string(nodeKey)), nil
}

func buildTemporalEdgeKey(ts timestampMicros, fromType string, from nodeID, rel relation, toType string, to nodeID) ([]byte, error) {
	edgeKey, err := buildEdgeKey(fromType, from, rel, toType, to)
	if err != nil {
		return nil, err
	}
	return []byte("ts:" + strconv.FormatUint(uint64(ts), 10) + ":" + string(edgeKey)), nil
}

func parseTemporalKey(key []byte) error {
	parts := strings.Split(string(key), ":")
	if len(parts) < 4 || parts[0] != "ts" {
		return errMalformedKey
	}
	if _, err := parseCanonicalTimestamp(parts[1]); err != nil {
		return errMalformedKey
	}
	suffix := []byte(strings.Join(parts[2:], ":"))
	switch parts[2] {
	case "n":
		_, err := parseNodeKey(suffix)
		return err
	case "e":
		_, err := parseEdgeKey(suffix)
		return err
	default:
		return errMalformedKey
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

func validateNodeID(id nodeID) error {
	value := string(id)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxNodeIDLen || strings.ContainsRune(value, ':') {
		return errInvalidComponent
	}
	return nil
}

func validateComponent(value string) error {
	if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, ':') {
		return errInvalidComponent
	}
	return nil
}

func validateTag(tag string) error {
	if validateComponent(tag) != nil {
		return errInvalidComponent
	}
	prevUnderscore := false
	for i, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			prevUnderscore = false
		case r == '_':
			if i == 0 || prevUnderscore {
				return errInvalidComponent
			}
			prevUnderscore = true
		default:
			return errInvalidComponent
		}
	}
	if prevUnderscore {
		return errInvalidComponent
	}
	return nil
}

func parseCanonicalTimestamp(value string) (timestampMicros, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, errInvalidComponent
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, errInvalidComponent
		}
	}
	u, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errInvalidComponent
	}
	return timestampMicros(u), nil
}

func bytewiseLess(a, b []byte) bool { return bytes.Compare(a, b) < 0 }

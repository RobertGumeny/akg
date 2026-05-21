package record

import (
	"encoding/binary"
	"math"
	"sort"
)

// EncodeNodePayload encodes a node as a deterministic MessagePack map for WAL/data payloads.
func EncodeNodePayload(n Node) ([]byte, error) {
	if err := n.ValidateForWrite(); err != nil {
		return nil, err
	}
	m := map[string]any{"type": n.Type, "title": n.Title, "created_at": uint64(n.CreatedAt), "updated_at": uint64(n.UpdatedAt)}
	if n.Body != "" {
		m["body"] = n.Body
	}
	if n.Meta != nil && len(n.Meta) > 0 {
		m["meta"] = n.Meta
	}
	if n.Tags != nil && len(n.Tags) > 0 {
		a := make([]any, len(n.Tags))
		for i, v := range n.Tags {
			a[i] = v
		}
		m["tags"] = a
	}
	if n.Version != 0 && n.Version != 1 {
		m["version"] = uint64(n.Version)
	}
	return encodeMsgpack(m)
}

// DecodeNodePayload decodes a MessagePack node map and applies AKG read defaults.
func DecodeNodePayload(b []byte) (Node, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return Node{}, ErrInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return Node{}, ErrInvalidPayload
	}
	typ, ok := m["type"].(string)
	if !ok || typ == "" {
		return Node{}, ErrMissingRequiredField
	}
	title, ok := m["title"].(string)
	if !ok || title == "" {
		return Node{}, ErrMissingRequiredField
	}
	node := Node{Type: typ, Title: title}
	if s, ok := m["body"].(string); ok {
		node.Body = s
	}
	if u, ok := asUint(m["created_at"]); ok {
		node.CreatedAt = TimestampMicros(u)
	}
	if u, ok := asUint(m["updated_at"]); ok {
		node.UpdatedAt = TimestampMicros(u)
	}
	if u, ok := asUint(m["version"]); ok {
		node.Version = Version(u)
	}
	if mm, ok := m["meta"].(map[string]any); ok {
		node.Meta = mm
	}
	if arr, ok := m["tags"].([]any); ok {
		node.Tags = make([]string, 0, len(arr))
		for _, x := range arr {
			s, ok := x.(string)
			if !ok {
				return Node{}, ErrInvalidPayload
			}
			node.Tags = append(node.Tags, s)
		}
	}
	node.ApplyReadDefaults()
	return node, nil
}

func EncodeEdgePayload(e Edge) ([]byte, error) {
	if err := e.ValidateForWrite(); err != nil {
		return nil, err
	}
	m := map[string]any{"from_node": string(e.FromNode), "to_node": string(e.ToNode), "relation": string(e.Relation), "created_at": uint64(e.CreatedAt), "updated_at": uint64(e.UpdatedAt)}
	if e.Strength != 0 {
		m["strength"] = e.Strength
	}
	if e.Confidence != nil {
		m["confidence"] = *e.Confidence
	}
	if e.Meta != nil && len(e.Meta) > 0 {
		m["meta"] = e.Meta
	}
	if e.Version != 0 && e.Version != 1 {
		m["version"] = uint64(e.Version)
	}
	return encodeMsgpack(m)
}

func DecodeEdgePayload(b []byte) (Edge, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return Edge{}, ErrInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return Edge{}, ErrInvalidPayload
	}
	from, ok := m["from_node"].(string)
	if !ok || from == "" {
		return Edge{}, ErrMissingRequiredField
	}
	to, ok := m["to_node"].(string)
	if !ok || to == "" {
		return Edge{}, ErrMissingRequiredField
	}
	rel, ok := m["relation"].(string)
	if !ok || rel == "" {
		return Edge{}, ErrMissingRequiredField
	}
	edge := Edge{FromNode: NodeID(from), ToNode: NodeID(to), Relation: Relation(rel)}
	if f, ok := asFloat(m["strength"]); ok {
		edge.Strength = f
	}
	if m["confidence"] != nil {
		if f, ok := asFloat(m["confidence"]); ok {
			edge.Confidence = &f
		}
	}
	if u, ok := asUint(m["created_at"]); ok {
		edge.CreatedAt = TimestampMicros(u)
	}
	if u, ok := asUint(m["updated_at"]); ok {
		edge.UpdatedAt = TimestampMicros(u)
	}
	if u, ok := asUint(m["version"]); ok {
		edge.Version = Version(u)
	}
	if mm, ok := m["meta"].(map[string]any); ok {
		edge.Meta = mm
	}
	edge.ApplyReadDefaults()
	return edge, nil
}

func EncodeNodeDeletePayload(d NodeDelete) ([]byte, error) {
	if err := d.ValidateForWrite(); err != nil {
		return nil, err
	}
	return encodeMsgpack(d.Map())
}
func EncodeEdgeDeletePayload(d EdgeDelete) ([]byte, error) {
	if err := d.ValidateForWrite(); err != nil {
		return nil, err
	}
	return encodeMsgpack(d.Map())
}
func DecodeNodeDeletePayload(b []byte) (NodeDelete, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return NodeDelete{}, ErrInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return NodeDelete{}, ErrInvalidPayload
	}
	return NodeDeleteFromMap(m)
}
func DecodeEdgeDeletePayload(b []byte) (EdgeDelete, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return EdgeDelete{}, ErrInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return EdgeDelete{}, ErrInvalidPayload
	}
	return EdgeDeleteFromMap(m)
}

func encodeMsgpack(v any) ([]byte, error) {
	var out []byte
	err := appendMsgpack(&out, v)
	return out, err
}

func appendMsgpack(out *[]byte, v any) error {
	switch x := v.(type) {
	case nil:
		*out = append(*out, 0xc0)
	case bool:
		if x {
			*out = append(*out, 0xc3)
		} else {
			*out = append(*out, 0xc2)
		}
	case string:
		appendStr(out, x)
	case uint64:
		*out = append(*out, 0xcf)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], x)
		*out = append(*out, b[:]...)
	case uint32:
		return appendMsgpack(out, uint64(x))
	case int:
		return appendMsgpack(out, uint64(x))
	case float64:
		*out = append(*out, 0xcb)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], math.Float64bits(x))
		*out = append(*out, b[:]...)
	case []any:
		appendArrayHeader(out, len(x))
		for _, e := range x {
			if err := appendMsgpack(out, e); err != nil {
				return err
			}
		}
	case map[string]any:
		appendMapHeader(out, len(x))
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			appendStr(out, k)
			if err := appendMsgpack(out, x[k]); err != nil {
				return err
			}
		}
	default:
		return ErrInvalidPayload
	}
	return nil
}

func appendStr(out *[]byte, s string) {
	l := len(s)
	if l < 32 {
		*out = append(*out, byte(0xa0|l))
	} else {
		*out = append(*out, 0xdb)
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(l))
		*out = append(*out, b[:]...)
	}
	*out = append(*out, s...)
}
func appendMapHeader(out *[]byte, l int) {
	if l < 16 {
		*out = append(*out, byte(0x80|l))
	} else {
		*out = append(*out, 0xde, byte(l>>8), byte(l))
	}
}
func appendArrayHeader(out *[]byte, l int) {
	if l < 16 {
		*out = append(*out, byte(0x90|l))
	} else {
		*out = append(*out, 0xdc, byte(l>>8), byte(l))
	}
}

func decodeMsgpack(b []byte) (any, int, error) {
	if len(b) == 0 {
		return nil, 0, ErrInvalidPayload
	}
	c := b[0]
	if c <= 0x7f {
		return uint64(c), 1, nil
	}
	if c >= 0xa0 && c <= 0xbf {
		l := int(c & 0x1f)
		if len(b) < 1+l {
			return nil, 0, ErrInvalidPayload
		}
		return string(b[1 : 1+l]), 1 + l, nil
	}
	if c >= 0x90 && c <= 0x9f {
		return decodeArray(b, 1, int(c&0x0f))
	}
	if c >= 0x80 && c <= 0x8f {
		return decodeMap(b, 1, int(c&0x0f))
	}
	switch c {
	case 0xc0:
		return nil, 1, nil
	case 0xc2:
		return false, 1, nil
	case 0xc3:
		return true, 1, nil
	case 0xcc:
		if len(b) < 2 {
			return nil, 0, ErrInvalidPayload
		}
		return uint64(b[1]), 2, nil
	case 0xcd:
		if len(b) < 3 {
			return nil, 0, ErrInvalidPayload
		}
		return uint64(binary.BigEndian.Uint16(b[1:3])), 3, nil
	case 0xce:
		if len(b) < 5 {
			return nil, 0, ErrInvalidPayload
		}
		return uint64(binary.BigEndian.Uint32(b[1:5])), 5, nil
	case 0xcf:
		if len(b) < 9 {
			return nil, 0, ErrInvalidPayload
		}
		return binary.BigEndian.Uint64(b[1:9]), 9, nil
	case 0xcb:
		if len(b) < 9 {
			return nil, 0, ErrInvalidPayload
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b[1:9])), 9, nil
	case 0xd9:
		if len(b) < 2 {
			return nil, 0, ErrInvalidPayload
		}
		l := int(b[1])
		if len(b) < 2+l {
			return nil, 0, ErrInvalidPayload
		}
		return string(b[2 : 2+l]), 2 + l, nil
	case 0xdb:
		if len(b) < 5 {
			return nil, 0, ErrInvalidPayload
		}
		l := int(binary.BigEndian.Uint32(b[1:5]))
		if len(b) < 5+l {
			return nil, 0, ErrInvalidPayload
		}
		return string(b[5 : 5+l]), 5 + l, nil
	case 0xdc:
		if len(b) < 3 {
			return nil, 0, ErrInvalidPayload
		}
		return decodeArray(b, 3, int(binary.BigEndian.Uint16(b[1:3])))
	case 0xde:
		if len(b) < 3 {
			return nil, 0, ErrInvalidPayload
		}
		return decodeMap(b, 3, int(binary.BigEndian.Uint16(b[1:3])))
	default:
		return nil, 0, ErrInvalidPayload
	}
}

func decodeArray(b []byte, off, l int) (any, int, error) {
	arr := make([]any, l)
	pos := off
	for i := 0; i < l; i++ {
		v, n, err := decodeMsgpack(b[pos:])
		if err != nil {
			return nil, 0, err
		}
		arr[i] = v
		pos += n
	}
	return arr, pos, nil
}
func decodeMap(b []byte, off, l int) (any, int, error) {
	m := make(map[string]any, l)
	pos := off
	for i := 0; i < l; i++ {
		k, n, err := decodeMsgpack(b[pos:])
		if err != nil {
			return nil, 0, err
		}
		pos += n
		ks, ok := k.(string)
		if !ok {
			return nil, 0, ErrInvalidPayload
		}
		v, n, err := decodeMsgpack(b[pos:])
		if err != nil {
			return nil, 0, err
		}
		pos += n
		m[ks] = v
	}
	return m, pos, nil
}
func asUint(v any) (uint64, bool) { u, ok := v.(uint64); return u, ok }
func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
}

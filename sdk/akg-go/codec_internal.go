package akg

import (
	"encoding/binary"
	"math"
	"sort"
	"unicode/utf8"
)

func encodeNodePayload(n coreNode) ([]byte, error) {
	m, err := nodePayloadMap(n)
	if err != nil {
		return nil, err
	}
	return encodeMsgpack(m)
}

func encodeNodePutPayload(p nodePut) ([]byte, error) {
	if p.ID == "" {
		return nil, ErrMissingRequiredField
	}
	m, err := nodePayloadMap(p.Node)
	if err != nil {
		return nil, err
	}
	m["id"] = string(p.ID)
	return encodeMsgpack(m)
}

func nodePayloadMap(n coreNode) (map[string]any, error) {
	if err := n.validateForWrite(); err != nil {
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
	return m, nil
}

func nodeFromMap(m map[string]any) (coreNode, error) {
	typ, ok := m["type"].(string)
	if !ok || typ == "" {
		return coreNode{}, ErrMissingRequiredField
	}
	title, ok := m["title"].(string)
	if !ok || title == "" {
		return coreNode{}, ErrMissingRequiredField
	}
	node := coreNode{Type: typ, Title: title}
	if s, ok := m["body"].(string); ok {
		node.Body = s
	}
	if u, ok := asUint(m["created_at"]); ok {
		node.CreatedAt = timestampMicros(u)
	}
	if u, ok := asUint(m["updated_at"]); ok {
		node.UpdatedAt = timestampMicros(u)
	}
	if u, ok := asUint(m["version"]); ok {
		node.Version = version(u)
	}
	if mm, ok := m["meta"].(map[string]any); ok {
		node.Meta = mm
	}
	if arr, ok := m["tags"].([]any); ok {
		node.Tags = make([]string, 0, len(arr))
		for _, x := range arr {
			s, ok := x.(string)
			if !ok {
				return coreNode{}, errInvalidPayload
			}
			node.Tags = append(node.Tags, s)
		}
	}
	node.applyReadDefaults()
	return node, nil
}

func decodeNodePayload(b []byte) (coreNode, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return coreNode{}, errInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return coreNode{}, errInvalidPayload
	}
	return nodeFromMap(m)
}

func decodeNodePutPayload(b []byte) (nodePut, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return nodePut{}, errInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nodePut{}, errInvalidPayload
	}
	id, ok := m["id"].(string)
	if !ok || id == "" {
		return nodePut{}, ErrMissingRequiredField
	}
	node, err := nodeFromMap(m)
	if err != nil {
		return nodePut{}, err
	}
	return nodePut{ID: nodeID(id), Node: node}, nil
}

func encodeNodeDeletePayload(d nodeDelete) ([]byte, error) {
	if d.Type == "" || d.ID == "" {
		return nil, ErrMissingRequiredField
	}
	return encodeMsgpack(map[string]any{"type": d.Type, "id": string(d.ID)})
}

func decodeNodeDeletePayload(b []byte) (nodeDelete, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return nodeDelete{}, errInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nodeDelete{}, errInvalidPayload
	}
	typeValue, ok := m["type"].(string)
	if !ok || typeValue == "" {
		return nodeDelete{}, ErrMissingRequiredField
	}
	idValue, ok := m["id"].(string)
	if !ok || idValue == "" {
		return nodeDelete{}, ErrMissingRequiredField
	}
	return nodeDelete{Type: typeValue, ID: nodeID(idValue)}, nil
}

func encodeEdgePayload(e coreEdge) ([]byte, error) {
	m, err := edgePayloadMap(e)
	if err != nil {
		return nil, err
	}
	return encodeMsgpack(m)
}

func encodeEdgePutPayload(p edgePut) ([]byte, error) {
	return encodeEdgePayload(p.Edge)
}

func edgePayloadMap(e coreEdge) (map[string]any, error) {
	if err := e.validateForWrite(); err != nil {
		return nil, err
	}
	m := map[string]any{
		"from_node_type": e.FromType,
		"from_node":      string(e.FromNode),
		"to_node_type":   e.ToType,
		"to_node":        string(e.ToNode),
		"relation":       string(e.Relation),
		"strength":       e.Strength,
		"created_at":     uint64(e.CreatedAt),
		"updated_at":     uint64(e.UpdatedAt),
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
	return m, nil
}

func decodeEdgePayload(b []byte) (coreEdge, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return coreEdge{}, errInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return coreEdge{}, errInvalidPayload
	}
	fromTypeValue, ok := m["from_node_type"].(string)
	if !ok || fromTypeValue == "" {
		return coreEdge{}, ErrMissingRequiredField
	}
	fromValue, ok := m["from_node"].(string)
	if !ok || fromValue == "" {
		return coreEdge{}, ErrMissingRequiredField
	}
	toTypeValue, ok := m["to_node_type"].(string)
	if !ok || toTypeValue == "" {
		return coreEdge{}, ErrMissingRequiredField
	}
	toValue, ok := m["to_node"].(string)
	if !ok || toValue == "" {
		return coreEdge{}, ErrMissingRequiredField
	}
	relationValue, ok := m["relation"].(string)
	if !ok || relationValue == "" {
		return coreEdge{}, ErrMissingRequiredField
	}
	edge := coreEdge{FromType: fromTypeValue, FromNode: nodeID(fromValue), ToType: toTypeValue, ToNode: nodeID(toValue), Relation: relation(relationValue), Strength: 0.5}
	if f, ok := asFloat64(m["strength"]); ok {
		edge.Strength = f
	}
	if _, ok := m["confidence"]; ok {
		if m["confidence"] == nil {
			edge.Confidence = nil
		} else {
			f, ok := asFloat64(m["confidence"])
			if !ok {
				return coreEdge{}, errInvalidPayload
			}
			edge.Confidence = &f
		}
	}
	if u, ok := asUint(m["created_at"]); ok {
		edge.CreatedAt = timestampMicros(u)
	}
	if u, ok := asUint(m["updated_at"]); ok {
		edge.UpdatedAt = timestampMicros(u)
	}
	if u, ok := asUint(m["version"]); ok {
		edge.Version = version(u)
	}
	if mm, ok := m["meta"].(map[string]any); ok {
		edge.Meta = mm
	}
	edge.applyReadDefaults()
	return edge, nil
}

func decodeEdgePutPayload(b []byte) (edgePut, error) {
	edge, err := decodeEdgePayload(b)
	if err != nil {
		return edgePut{}, err
	}
	return edgePut{Edge: edge}, nil
}

func encodeEdgeDeletePayload(d edgeDelete) ([]byte, error) {
	if d.FromType == "" || d.FromNode == "" || d.Relation == "" || d.ToType == "" || d.ToNode == "" {
		return nil, ErrMissingRequiredField
	}
	return encodeMsgpack(map[string]any{
		"from_node_type": d.FromType,
		"from_node":      string(d.FromNode),
		"relation":       string(d.Relation),
		"to_node_type":   d.ToType,
		"to_node":        string(d.ToNode),
	})
}

func decodeEdgeDeletePayload(b []byte) (edgeDelete, error) {
	v, n, err := decodeMsgpack(b)
	if err != nil || n != len(b) {
		return edgeDelete{}, errInvalidPayload
	}
	m, ok := v.(map[string]any)
	if !ok {
		return edgeDelete{}, errInvalidPayload
	}
	fromTypeValue, ok := m["from_node_type"].(string)
	if !ok || fromTypeValue == "" {
		return edgeDelete{}, ErrMissingRequiredField
	}
	fromValue, ok := m["from_node"].(string)
	if !ok || fromValue == "" {
		return edgeDelete{}, ErrMissingRequiredField
	}
	relationValue, ok := m["relation"].(string)
	if !ok || relationValue == "" {
		return edgeDelete{}, ErrMissingRequiredField
	}
	toTypeValue, ok := m["to_node_type"].(string)
	if !ok || toTypeValue == "" {
		return edgeDelete{}, ErrMissingRequiredField
	}
	toValue, ok := m["to_node"].(string)
	if !ok || toValue == "" {
		return edgeDelete{}, ErrMissingRequiredField
	}
	return edgeDelete{FromType: fromTypeValue, FromNode: nodeID(fromValue), Relation: relation(relationValue), ToType: toTypeValue, ToNode: nodeID(toValue)}, nil
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
		if !utf8.ValidString(x) {
			return errInvalidPayload
		}
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
			if !utf8.ValidString(k) {
				return errInvalidPayload
			}
			appendStr(out, k)
			if err := appendMsgpack(out, x[k]); err != nil {
				return err
			}
		}
		return nil
	default:
		return errInvalidPayload
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
		return nil, 0, errInvalidPayload
	}
	c := b[0]
	if c <= 0x7f {
		return uint64(c), 1, nil
	}
	if c >= 0xa0 && c <= 0xbf {
		l := int(c & 0x1f)
		if len(b) < 1+l {
			return nil, 0, errInvalidPayload
		}
		s := string(b[1 : 1+l])
		if !utf8.ValidString(s) {
			return nil, 0, errInvalidPayload
		}
		return s, 1 + l, nil
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
			return nil, 0, errInvalidPayload
		}
		return uint64(b[1]), 2, nil
	case 0xcd:
		if len(b) < 3 {
			return nil, 0, errInvalidPayload
		}
		return uint64(binary.BigEndian.Uint16(b[1:3])), 3, nil
	case 0xce:
		if len(b) < 5 {
			return nil, 0, errInvalidPayload
		}
		return uint64(binary.BigEndian.Uint32(b[1:5])), 5, nil
	case 0xcf:
		if len(b) < 9 {
			return nil, 0, errInvalidPayload
		}
		return binary.BigEndian.Uint64(b[1:9]), 9, nil
	case 0xcb:
		if len(b) < 9 {
			return nil, 0, errInvalidPayload
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b[1:9])), 9, nil
	case 0xdb:
		if len(b) < 5 {
			return nil, 0, errInvalidPayload
		}
		l := int(binary.BigEndian.Uint32(b[1:5]))
		if len(b) < 5+l {
			return nil, 0, errInvalidPayload
		}
		s := string(b[5 : 5+l])
		if !utf8.ValidString(s) {
			return nil, 0, errInvalidPayload
		}
		return s, 5 + l, nil
	case 0xdc:
		if len(b) < 3 {
			return nil, 0, errInvalidPayload
		}
		return decodeArray(b, 3, int(binary.BigEndian.Uint16(b[1:3])))
	case 0xde:
		if len(b) < 3 {
			return nil, 0, errInvalidPayload
		}
		return decodeMap(b, 3, int(binary.BigEndian.Uint16(b[1:3])))
	default:
		return nil, 0, errInvalidPayload
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
			return nil, 0, errInvalidPayload
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

func asFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case uint64:
		return float64(x), true
	}
	return 0, false
}

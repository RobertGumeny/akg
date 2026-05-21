package wal

// SequenceNumber is a monotonically increasing WAL sequence value.
type SequenceNumber uint64

// Operation identifies an AKG WAL record operation.
type Operation uint8

const (
	OpPutNode    Operation = 0x01
	OpDeleteNode Operation = 0x02
	OpPutEdge    Operation = 0x03
	OpDeleteEdge Operation = 0x04
	OpCommit     Operation = 0x05
)

// Valid reports whether op is defined by AKG v1.
func (op Operation) Valid() bool {
	switch op {
	case OpPutNode, OpDeleteNode, OpPutEdge, OpDeleteEdge, OpCommit:
		return true
	default:
		return false
	}
}

// Record is the foundational WAL record shape. Encoding and checksum behavior
// are implemented by later Milestone 1 tasks.
type Record struct {
	Sequence  SequenceNumber
	Operation Operation
	Payload   []byte
}

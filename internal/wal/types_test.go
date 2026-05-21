package wal

import "testing"

func TestOperationConstants(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
		code uint8
	}{
		{"PUT_NODE", OpPutNode, 0x01},
		{"DELETE_NODE", OpDeleteNode, 0x02},
		{"PUT_EDGE", OpPutEdge, 0x03},
		{"DELETE_EDGE", OpDeleteEdge, 0x04},
		{"COMMIT", OpCommit, 0x05},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if uint8(tt.op) != tt.code {
				t.Fatalf("code = %#02x, want %#02x", uint8(tt.op), tt.code)
			}
			if !tt.op.Valid() {
				t.Fatalf("%s should be valid", tt.name)
			}
		})
	}
	if Operation(0xff).Valid() {
		t.Fatal("unknown operation reported valid")
	}
}

package p2p

import (
	"encoding/json"
	"testing"

	"github.com/paylink/paylink-chain/internal/types"
)

func TestEncodeDecodeEnvelope_Block(t *testing.T) {
	block := &types.Block{
		Header: types.BlockHeader{
			Height:    42,
			Timestamp: 1000,
		},
		Hash: types.Hash{0x01, 0x02, 0x03},
	}

	data, err := EncodeEnvelope(MsgTypeBlock, BlockMessage{Block: block})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if env.Type != MsgTypeBlock {
		t.Fatalf("type = %q, want %q", env.Type, MsgTypeBlock)
	}

	var bm BlockMessage
	if err := json.Unmarshal(env.Data, &bm); err != nil {
		t.Fatalf("unmarshal block: %v", err)
	}
	if bm.Block == nil {
		t.Fatal("block is nil")
	}
	if bm.Block.Header.Height != 42 {
		t.Errorf("height = %d, want 42", bm.Block.Header.Height)
	}
}

func TestEncodeDecodeEnvelope_Tx(t *testing.T) {
	tx := &types.Transaction{
		Type:  types.TxTransfer,
		Nonce: 7,
		Hash:  types.Hash{0xAA},
	}

	data, err := EncodeEnvelope(MsgTypeTx, TxMessage{Transaction: tx})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if env.Type != MsgTypeTx {
		t.Fatalf("type = %q, want %q", env.Type, MsgTypeTx)
	}

	var tm TxMessage
	if err := json.Unmarshal(env.Data, &tm); err != nil {
		t.Fatalf("unmarshal tx: %v", err)
	}
	if tm.Transaction == nil {
		t.Fatal("tx is nil")
	}
	if tm.Transaction.Nonce != 7 {
		t.Errorf("nonce = %d, want 7", tm.Transaction.Nonce)
	}
}

func TestEncodeDecodeEnvelope_SyncRequest(t *testing.T) {
	req := SyncRequest{FromHeight: 100, ToHeight: 200}
	data, err := EncodeEnvelope(MsgTypeSyncReq, req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if env.Type != MsgTypeSyncReq {
		t.Fatalf("type = %q, want %q", env.Type, MsgTypeSyncReq)
	}

	var decoded SyncRequest
	if err := json.Unmarshal(env.Data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.FromHeight != 100 || decoded.ToHeight != 200 {
		t.Errorf("request = %+v, want {100, 200}", decoded)
	}
}

func TestEncodeDecodeEnvelope_SyncResponse(t *testing.T) {
	blocks := []*types.Block{
		{Header: types.BlockHeader{Height: 1}, Hash: types.Hash{0x01}},
		{Header: types.BlockHeader{Height: 2}, Hash: types.Hash{0x02}},
	}
	resp := SyncResponse{Blocks: blocks}

	data, err := EncodeEnvelope(MsgTypeSyncResp, resp)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var decoded SyncResponse
	if err := json.Unmarshal(env.Data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(decoded.Blocks))
	}
	if decoded.Blocks[0].Header.Height != 1 || decoded.Blocks[1].Header.Height != 2 {
		t.Error("block heights don't match")
	}
}

func TestDecodeEnvelope_Invalid(t *testing.T) {
	_, err := DecodeEnvelope([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeEnvelope_Empty(t *testing.T) {
	_, err := DecodeEnvelope(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestEncodeEnvelope_RoundTrip(t *testing.T) {
	// Ensure that encoding and decoding preserves all message types
	msgTypes := []string{MsgTypeBlock, MsgTypeTx, MsgTypeSyncReq, MsgTypeSyncResp}
	for _, mt := range msgTypes {
		data, err := EncodeEnvelope(mt, struct{ Foo string }{Foo: "bar"})
		if err != nil {
			t.Fatalf("encode %s: %v", mt, err)
		}
		env, err := DecodeEnvelope(data)
		if err != nil {
			t.Fatalf("decode %s: %v", mt, err)
		}
		if env.Type != mt {
			t.Errorf("type = %q, want %q", env.Type, mt)
		}
	}
}

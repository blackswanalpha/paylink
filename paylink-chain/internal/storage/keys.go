package storage

import "github.com/paylink/paylink-chain/internal/types"

// Key prefixes for DB namespacing.
var (
	PrefixBlock     = []byte("b/")  // b/{height} -> Block
	PrefixBlockHash = []byte("bh/") // bh/{hash} -> height
	PrefixTx        = []byte("tx/") // tx/{hash} -> Transaction
	PrefixReceipt   = []byte("r/")  // r/{txHash} -> TxReceipt
	PrefixState     = []byte("s/")  // s/ -> serialized state
	PrefixChainMeta = []byte("m/")  // m/{key} -> chain metadata
)

// Key construction helpers

func BlockKey(height uint64) []byte {
	return append(PrefixBlock, uint64ToBytes(height)...)
}

func BlockHashKey(hash types.Hash) []byte {
	return append(PrefixBlockHash, hash[:]...)
}

func TxKey(hash types.Hash) []byte {
	return append(PrefixTx, hash[:]...)
}

func ReceiptKey(txHash types.Hash) []byte {
	return append(PrefixReceipt, txHash[:]...)
}

func ChainMetaKey(key string) []byte {
	return append(PrefixChainMeta, []byte(key)...)
}

func uint64ToBytes(v uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}

package eventbus

import "strings"

// Domains is the fixed set of domain topics (see workload/catalog.md). One topic per domain; the
// full logical event name lives in the envelope, not the topic.
var Domains = []string{
	"paylink", "payment", "chain", "merchant", "compliance",
	"identity", "notification", "escrow", "settlement", "fee",
}

// TopicFor maps a logical event name to its domain topic — the first dot-segment. So
// "paylink.verified" → "paylink" and "chain.paylink.verified" → "chain". A name with no dot maps to
// itself.
func TopicFor(name string) string {
	if i := strings.IndexByte(name, '.'); i >= 0 {
		return name[:i]
	}
	return name
}

package eventbus

import "strings"

// Domains is the fixed set of domain topics (see workload/catalog.md). One topic per domain; the
// full logical event name lives in the envelope, not the topic. Kept byte-identical (same order)
// with eventbus-python/topics.py DOMAINS and the docker-compose redpanda-init topic list.
var Domains = []string{
	"paylink", "payment", "chain", "merchant", "compliance",
	"identity", "notification", "escrow", "settlement", "fee",
	"pricing", "fx", "invoice", "refund", "dispute",
}

// topicAliases routes logical-name domains that share another domain's physical topic (see
// catalog.md). settlement-service (work23) publishes both "settlement.*" and "payout.*" on the
// settlement topic, so "payout" routes to "settlement" rather than a topic of its own.
var topicAliases = map[string]string{"payout": "settlement"}

// TopicFor maps a logical event name to its domain topic — the first dot-segment (with aliases). So
// "paylink.verified" → "paylink", "chain.paylink.verified" → "chain", and "payout.scheduled" →
// "settlement". A name with no dot maps to itself.
func TopicFor(name string) string {
	segment := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		segment = name[:i]
	}
	if alias, ok := topicAliases[segment]; ok {
		return alias
	}
	return segment
}

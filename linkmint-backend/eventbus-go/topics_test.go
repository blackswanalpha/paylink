package eventbus

import "testing"

func TestTopicFor(t *testing.T) {
	cases := map[string]string{
		"paylink.verified":            "paylink",
		"chain.paylink.verified":      "chain",
		"payment.proof_received":      "payment",
		"merchant.bank_account.added": "merchant",
		"identity.user.registered":    "identity",
		"singleton":                   "singleton",
	}
	for name, want := range cases {
		if got := TopicFor(name); got != want {
			t.Errorf("TopicFor(%q)=%q want %q", name, got, want)
		}
	}
}

func TestSplitBrokers(t *testing.T) {
	got := SplitBrokers(" a:9092, b:9092 ,,c:9092 ")
	want := []string{"a:9092", "b:9092", "c:9092"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestDomainsCoversTopicForOfEachCatalogDomain(t *testing.T) {
	for _, d := range Domains {
		if got := TopicFor(d + ".some.event"); got != d {
			t.Errorf("domain %q: TopicFor=%q", d, got)
		}
	}
}

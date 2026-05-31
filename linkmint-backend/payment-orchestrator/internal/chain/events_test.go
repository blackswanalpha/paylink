package chain

import "testing"

func TestChainStatusForEvent(t *testing.T) {
	cases := []struct {
		name   string
		ev     Event
		want   string
		wantOK bool
	}{
		{"toState verified wins", Event{Kind: KindPayLinkVerified, ToState: "VERIFIED"}, "VERIFIED", true},
		{"toState used over kind", Event{Kind: "paylink.voted", ToState: "CANCELLED"}, "CANCELLED", true},
		{"kind verified", Event{Kind: KindPayLinkVerified}, "VERIFIED", true},
		{"kind cancelled", Event{Kind: KindPayLinkCancelled}, "CANCELLED", true},
		{"kind failed", Event{Kind: KindPayLinkFailed}, "FAILED", true},
		{"kind created", Event{Kind: KindPayLinkCreated}, "CREATED", true},
		{"unrelated kind", Event{Kind: "paylink.voted"}, "", false},
		{"empty", Event{}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ChainStatusForEvent(c.ev)
			if got != c.want || ok != c.wantOK {
				t.Errorf("ChainStatusForEvent(%+v) = (%q,%v), want (%q,%v)", c.ev, got, ok, c.want, c.wantOK)
			}
		})
	}
}

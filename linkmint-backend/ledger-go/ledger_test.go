package ledger

import "testing"

func TestFlip(t *testing.T) {
	if flip(DR) != CR {
		t.Fatal("flip(DR) should be CR")
	}
	if flip(CR) != DR {
		t.Fatal("flip(CR) should be DR")
	}
}

func TestClampLimit(t *testing.T) {
	for in, want := range map[int]int{0: 100, -5: 100, 1: 1, 50: 50, 1000: 1000, 5000: 1000} {
		if got := clampLimit(in); got != want {
			t.Fatalf("clampLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestNullIfEmpty(t *testing.T) {
	if nullIfEmpty("") != nil {
		t.Fatal(`nullIfEmpty("") should be nil`)
	}
	if nullIfEmpty("PLK1") != "PLK1" {
		t.Fatal(`nullIfEmpty("PLK1") should return the value`)
	}
}

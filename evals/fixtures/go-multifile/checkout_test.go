package evalfixture

import "testing"

func TestCheckoutMath(t *testing.T) {
	if got := ApplyDiscount(1000, 20); got != 800 {
		t.Fatalf("ApplyDiscount(1000, 20) = %d, want 800", got)
	}
	if got := AddTax(1000, 8); got != 1080 {
		t.Fatalf("AddTax(1000, 8) = %d, want 1080", got)
	}
}

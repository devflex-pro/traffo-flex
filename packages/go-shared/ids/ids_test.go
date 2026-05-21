package ids

import (
	"strings"
	"testing"
)

func TestNewAddsPrefixAndUniqueValue(t *testing.T) {
	first := New("clk")
	second := New("clk")

	if !strings.HasPrefix(
		first,
		"clk_",
	) {
		t.Fatalf(
			"id %q does not have clk_ prefix",
			first,
		)
	}
	if first == second {
		t.Fatalf(
			"ids should be unique, got %q twice",
			first,
		)
	}
}

package macros

import (
	"reflect"
	"testing"
)

func TestRenderReplacesKnownValuesAndLeavesMissingMacros(t *testing.T) {
	got := Render("https://example.com/?cid={click_id}&sub={sub1}&missing={missing}", map[string]string{
		"click_id": "clk_1",
		"sub1":     "zone_a",
	})
	want := "https://example.com/?cid=clk_1&sub=zone_a&missing={missing}"

	if got != want {
		t.Fatalf(
			"Render returned %q, want %q",
			got,
			want,
		)
	}
}

func TestNamesReturnsSortedUniqueMacroNames(t *testing.T) {
	got := Names("{sub1}-{click_id}-{sub1}")
	want := []string{"click_id", "sub1"}

	if !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf(
			"Names returned %#v, want %#v",
			got,
			want,
		)
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("click_id"); err != nil {
		t.Fatalf(
			"valid macro rejected: %v",
			err,
		)
	}
	if err := ValidateName("ClickID"); err == nil {
		t.Fatal("expected invalid macro name error")
	}
	if err := ValidateName("bad}{x"); err == nil {
		t.Fatal("expected invalid macro name error")
	}
}

func TestValidateTemplate(t *testing.T) {
	allowed := map[string]struct{}{
		"click_id": {},
		"sub1":     {},
	}

	if err := ValidateTemplate(
		"{click_id}-{sub1}",
		allowed,
	); err != nil {
		t.Fatalf(
			"valid template rejected: %v",
			err,
		)
	}
	if err := ValidateTemplate(
		"{click_id}-{unknown}",
		allowed,
	); err == nil {
		t.Fatal("expected unknown macro error")
	}
}

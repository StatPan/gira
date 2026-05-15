package gira

import "testing"

func TestQuoteShellArg(t *testing.T) {
	cases := map[string]string{
		"StatPan/gira":    "StatPan/gira",
		"status:ready":    "status:ready",
		"":                "''",
		"two words":       "'two words'",
		"feature;cleanup": "'feature;cleanup'",
		"$(touch marker)": "'$(touch marker)'",
		"owner's repo":    "'owner'\"'\"'s repo'",
		"line\nbreak":     "'line\nbreak'",
	}
	for input, want := range cases {
		if got := QuoteShellArg(input); got != want {
			t.Fatalf("QuoteShellArg(%q) = %q, want %q", input, got, want)
		}
	}
}

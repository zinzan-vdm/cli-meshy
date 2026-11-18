package ca

import "testing"

func TestNextCountsUpFromTheTrustedAuthority(t *testing.T) {
	cases := []struct {
		names []string
		want  string
	}{
		{[]string{"ca-1"}, "ca-2"},
		{[]string{"ca-2"}, "ca-3"},
		{[]string{"ca-9"}, "ca-10"},
		{[]string{"ca-1", "ca-2"}, "ca-3"},
		{[]string{"ca-1", "core-1"}, "ca-2"},
		{[]string{"ca-1", "ca-x"}, "ca-2"},
		{nil, "ca-1"},
	}

	for _, c := range cases {
		if got := Next(c.names); got != c.want {
			t.Errorf("Next(%v) = %q, want %q", c.names, got, c.want)
		}
	}
}

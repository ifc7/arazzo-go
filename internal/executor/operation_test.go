package executor

import "testing"

func TestParseOperationID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, source, op string
	}{
		{"$sourceDescriptions.userSessionApi.passwordLogin", "userSessionApi", "passwordLogin"},
		{"userSessionApi.passwordLogin", "userSessionApi", "passwordLogin"},
		{"passwordLogin", "", "passwordLogin"},
	}
	for _, tc := range cases {
		source, op := parseOperationID(tc.in)
		if source != tc.source || op != tc.op {
			t.Errorf("%q: got %q %q want %q %q", tc.in, source, op, tc.source, tc.op)
		}
	}
}

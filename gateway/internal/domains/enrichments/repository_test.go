package enrichments

import "testing"

func TestComputeOverallStatus(t *testing.T) {
	tests := []struct {
		name       string
		applicable []string
		want       string
	}{
		{
			name:       "empty (all not_applicable) → complete",
			applicable: []string{},
			want:       StatusComplete,
		},
		{
			name:       "all success → complete",
			applicable: []string{SourceSuccess, SourceSuccess, SourceSuccess},
			want:       StatusComplete,
		},
		{
			name:       "all failed → failed",
			applicable: []string{SourceFailed, SourceFailed},
			want:       StatusFailed,
		},
		{
			name:       "any pending → pending",
			applicable: []string{SourceSuccess, SourcePending, SourceFailed},
			want:       StatusPending,
		},
		{
			name:       "mix success + failed, no pending → partial",
			applicable: []string{SourceSuccess, SourceFailed},
			want:       StatusPartial,
		},
		{
			name:       "single success → complete",
			applicable: []string{SourceSuccess},
			want:       StatusComplete,
		},
		{
			name:       "single failed → failed",
			applicable: []string{SourceFailed},
			want:       StatusFailed,
		},
		{
			name:       "single pending → pending",
			applicable: []string{SourcePending},
			want:       StatusPending,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeOverallStatus(tc.applicable)
			if got != tc.want {
				t.Errorf("computeOverallStatus(%v) = %q, want %q", tc.applicable, got, tc.want)
			}
		})
	}
}

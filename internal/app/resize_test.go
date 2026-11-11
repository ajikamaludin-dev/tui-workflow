package app

import "testing"

func TestIsTooSmall(t *testing.T) {
	tests := []struct {
		w, h     int
		expected bool
	}{
		{79, 24, true},
		{80, 23, true},
		{79, 23, true},
		{80, 24, false},
		{120, 40, false},
	}

	for _, tc := range tests {
		got := IsTooSmall(tc.w, tc.h)
		if got != tc.expected {
			t.Errorf("IsTooSmall(%d, %d) = %v; want %v", tc.w, tc.h, got, tc.expected)
		}
	}
}

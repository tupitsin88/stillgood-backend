package geo

import "testing"

func TestValidLatitude(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		want  bool
	}{
		{name: "lower boundary", value: -90, want: true},
		{name: "upper boundary", value: 90, want: true},
		{name: "below lower boundary", value: -90.1, want: false},
		{name: "above upper boundary", value: 90.1, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidLatitude(tc.value); got != tc.want {
				t.Fatalf("ValidLatitude(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestValidLongitude(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		want  bool
	}{
		{name: "lower boundary", value: -180, want: true},
		{name: "upper boundary", value: 180, want: true},
		{name: "below lower boundary", value: -180.1, want: false},
		{name: "above upper boundary", value: 180.1, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidLongitude(tc.value); got != tc.want {
				t.Fatalf("ValidLongitude(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestValidRadiusMeters(t *testing.T) {
	if !ValidRadiusMeters(1) {
		t.Fatal("ValidRadiusMeters(1) = false, want true")
	}
	if ValidRadiusMeters(0) {
		t.Fatal("ValidRadiusMeters(0) = true, want false")
	}
}

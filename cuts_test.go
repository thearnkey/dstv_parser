package dstv_parser

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicFixtures(t *testing.T) {
	for _, tc := range []struct {
		file, profile string
		length        float64
		contours      int
		ready         bool
	}{
		{"baseflow_product2.nc1", "B", 320, 1, false},
		{"tluijken_channel_cut.nc1", "U", 322.25, 3, false},
		{"tluijken_ibeam_curved.nc1", "I", 954.5, 3, true},
		{"tluijken_tube_mitre.nc1", "M", 1000, 4, false},
		{"tluijken_plate_comma.nc", "B", 233.7, 1, true},
	} {
		t.Run(tc.file, func(t *testing.T) {
			p, err := ParseFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			if p.Header.ProfileCode != tc.profile || math.Abs(p.Cutting().LengthMM-tc.length) > 0.001 || len(p.ExternalContours) != tc.contours || p.Cutting().Ready != tc.ready {
				t.Fatalf("unexpected result: header=%+v, cutting=%+v, contours=%d", p.Header, p.Cutting(), len(p.ExternalContours))
			}
		})
	}
}

func TestCutsRequireReviewWithoutSawLength(t *testing.T) {
	base := strings.Replace(example, "2000.00 2030.00", "2000.00", 1)
	base = strings.Replace(base, "  0.00\r\n  0.00\r\n  0.00\r\n  0.00\r\n  beam", "  15.00\r\n  0.00\r\n  0.00\r\n  0.00\r\n  beam", 1)
	p, err := Parse(strings.NewReader(base))
	if err != nil {
		t.Fatal(err)
	}
	if p.Cutting().Ready || !strings.Contains(strings.Join(p.Cutting().Reasons, " "), "skew cut") {
		t.Fatalf("missing skew warning: %+v", p.Cutting())
	}
}

func TestSCAndAKSpan(t *testing.T) {
	input := strings.Replace(example, "2000.00 2030.00", "2000.00", 1)
	input = strings.Replace(input, "EN\r\n", "AK\r\n  v 0.00u 0.00 0.00\r\n  2050.00 0.00 0.00\r\n  2050.00 100.00 0.00\r\n  0.00 100.00 0.00\r\n  0.00 0.00 0.00\r\nSC\r\n  0 0 0 -1 0 0\r\nEN\r\n", 1)
	p, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	a := p.Cutting()
	if a.Ready || a.LengthMM != 2050 || a.LengthSource != "ak_span" || len(p.SawCuts) != 1 {
		t.Fatalf("unexpected cut data: %+v, SC=%+v", a, p.SawCuts)
	}
}

func TestArcXBounds(t *testing.T) {
	start := ContourPoint{X: 0, Y: 0, Radius: 50}
	end := ContourPoint{X: 0, Y: 100}
	lo, hi, err := arcXBounds(start, end)
	if err != nil || math.Abs(lo) > 1e-9 || math.Abs(hi-50) > 1e-9 {
		t.Fatalf("positive arc: %v %v %v", lo, hi, err)
	}
	start.Radius = -50
	lo, hi, err = arcXBounds(start, end)
	if err != nil || math.Abs(lo+50) > 1e-9 || math.Abs(hi) > 1e-9 {
		t.Fatalf("negative arc: %v %v %v", lo, hi, err)
	}
	start.Radius = 40
	if _, _, err := arcXBounds(start, end); err == nil {
		t.Fatal("impossible arc accepted")
	}
}

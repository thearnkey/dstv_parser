package dstv_parser

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Contour is an AK external contour on one face. Coordinates are in mm.
// Points retain their source lines; welding preparation is kept in RawData.
type Contour struct {
	Line   int
	Kind   string // AK (external) or IK (internal).
	Points []ContourPoint
}

type ContourPoint struct {
	Line      int
	Face      string
	X, Y      float64
	Reference string
	Notch     string
	Radius    float64
	RawData   string
}

// SawCut is a DSTV SC plane: a point on the plane and its outward normal.
type SawCut struct {
	Line                      int
	X, Y, Z                   float64
	NormalX, NormalY, NormalZ float64
}

// CuttingAssessment describes the length available for linear nesting. Ready
// means the file supplies enough information to use LengthMM as a blank length;
// it does not check machine limits, kerf or clamping.
type CuttingAssessment struct {
	LengthMM      float64
	LengthSource  string
	ContourSpanMM float64
	Ready         bool
	Reasons       []string
}

var contourPattern = regexp.MustCompile(`(?i)^\s*([vouh])?\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+))\s*([osu])?\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+))\s*([tw])?\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+))(.*)$`)

func (p *Part) parseCuts() {
	for _, block := range p.Blocks {
		switch block.Code {
		case "AK", "IK":
			c := Contour{Line: block.Line, Kind: block.Code}
			face, reference := "", ""
			for _, line := range block.Data {
				m := contourPattern.FindStringSubmatch(line.Text)
				if m == nil {
					p.CutWarnings = append(p.CutWarnings, fmt.Sprintf("line %d: unsupported %s point", line.Line, block.Code))
					continue
				}
				x, errX := number(m[2], line.Line)
				y, errY := number(m[4], line.Line)
				radius, errR := number(m[6], line.Line)
				if errX != nil || errY != nil || errR != nil {
					p.CutWarnings = append(p.CutWarnings, fmt.Sprintf("line %d: invalid %s point", line.Line, block.Code))
					continue
				}
				if m[1] != "" {
					face = strings.ToLower(m[1])
				}
				if m[3] != "" {
					reference = strings.ToLower(m[3])
				}
				c.Points = append(c.Points, ContourPoint{Line: line.Line, Face: face, X: x, Y: y, Reference: reference, Notch: strings.ToLower(m[5]), Radius: radius, RawData: strings.TrimSpace(m[7])})
			}
			ordinary := contourGeometryPoints(c.Points)
			if len(ordinary) < 2 {
				p.CutWarnings = append(p.CutWarnings, fmt.Sprintf("line %d: incomplete %s contour", block.Line, block.Code))
			} else if first, last := ordinary[0], ordinary[len(ordinary)-1]; math.Abs(first.X-last.X) > 0.001 || math.Abs(first.Y-last.Y) > 0.001 {
				p.CutWarnings = append(p.CutWarnings, fmt.Sprintf("line %d: %s contour is not closed", block.Line, block.Code))
			}
			if block.Code == "AK" {
				p.ExternalContours = append(p.ExternalContours, c)
			} else {
				p.InternalContours = append(p.InternalContours, c)
			}
		case "SC":
			for _, line := range block.Data {
				fields := strings.Fields(line.Text)
				if len(fields) != 6 {
					p.CutWarnings = append(p.CutWarnings, fmt.Sprintf("line %d: SC requires six numbers", line.Line))
					continue
				}
				var v [6]float64
				valid := true
				for i, field := range fields {
					var err error
					v[i], err = number(field, line.Line)
					if err != nil {
						valid = false
						break
					}
				}
				if !valid || (v[3] == 0 && v[4] == 0 && v[5] == 0) {
					p.CutWarnings = append(p.CutWarnings, fmt.Sprintf("line %d: invalid SC plane", line.Line))
					continue
				}
				p.SawCuts = append(p.SawCuts, SawCut{Line: line.Line, X: v[0], Y: v[1], Z: v[2], NormalX: v[3], NormalY: v[4], NormalZ: v[5]})
			}
		default:
			if len(block.Code) == 2 && block.Code[0] == 'A' && block.Code[1] >= '0' && block.Code[1] <= '9' {
				p.CutWarnings = append(p.CutWarnings, fmt.Sprintf("line %d: additional-plane contour %s needs review", block.Line, block.Code))
			}
		}
	}
}

// Cutting returns the length and whether it is suitable for automatic nesting.
// With no explicit saw length, a parsed AK contour can raise the required
// length to its X span. Nonzero contour radii or SC planes need human review.
func (p *Part) Cutting() CuttingAssessment {
	a := CuttingAssessment{LengthMM: p.Header.Length, LengthSource: "header", Ready: true}
	if p.Header.HasSawLength {
		a.LengthMM = p.Header.SawLength
		a.LengthSource = "saw_length"
	}
	a.Reasons = append(a.Reasons, p.CutWarnings...)
	if len(p.CutWarnings) > 0 {
		a.Ready = false
	}
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, c := range p.ExternalContours {
		points := contourGeometryPoints(c.Points)
		for i, point := range points {
			minX = math.Min(minX, point.X)
			maxX = math.Max(maxX, point.X)
			if point.Radius != 0 && i+1 < len(points) {
				lo, hi, err := arcXBounds(point, points[i+1])
				if err != nil {
					a.Ready = false
					a.Reasons = append(a.Reasons, fmt.Sprintf("line %d: %v", point.Line, err))
				} else {
					minX = math.Min(minX, lo)
					maxX = math.Max(maxX, hi)
				}
			}
			if hasWeldPreparation(point.RawData) {
				a.Ready = false
				a.Reasons = append(a.Reasons, fmt.Sprintf("line %d: welding preparation needs review", point.Line))
			}
		}
	}
	for _, block := range p.Blocks {
		if block.Code == "KA" {
			a.Ready = false
			a.Reasons = append(a.Reasons, "bent part needs review")
		}
	}
	if !math.IsInf(minX, 0) {
		a.ContourSpanMM = maxX - minX
		if a.ContourSpanMM > a.LengthMM+0.001 {
			a.LengthMM = a.ContourSpanMM
			a.LengthSource = "ak_span"
			if p.Header.HasSawLength {
				a.Ready = false
				a.Reasons = append(a.Reasons, "AK span exceeds explicit saw length")
			}
		}
	}
	if !p.Header.HasSawLength {
		if p.Header.WebStartCut != 0 || p.Header.WebEndCut != 0 || p.Header.FlangeStartCut != 0 || p.Header.FlangeEndCut != 0 {
			a.Ready = false
			a.Reasons = append(a.Reasons, "skew cut without explicit saw length")
		}
		if len(p.SawCuts) > 0 {
			a.Ready = false
			a.Reasons = append(a.Reasons, "SC cut without explicit saw length")
		}
	}
	return a
}

func contourGeometryPoints(points []ContourPoint) []ContourPoint {
	result := make([]ContourPoint, 0, len(points))
	for _, point := range points {
		if point.Notch == "" {
			result = append(result, point)
		}
	}
	return result
}

// arcXBounds returns the longitudinal extrema of the minor circular arc from
// start to end. DSTV stores the signed radius on the start point.
func arcXBounds(start, end ContourPoint) (float64, float64, error) {
	r := math.Abs(start.Radius)
	dx, dy := end.X-start.X, end.Y-start.Y
	chord := math.Hypot(dx, dy)
	if chord < 1e-9 || chord > 2*r+1e-6 {
		return 0, 0, fmt.Errorf("invalid contour arc")
	}
	mx, my := (start.X+end.X)/2, (start.Y+end.Y)/2
	h := math.Sqrt(math.Max(0, r*r-chord*chord/4))
	ux, uy := -dy/chord, dx/chord
	for _, side := range []float64{1, -1} {
		cx, cy := mx+side*h*ux, my+side*h*uy
		from := math.Atan2(start.Y-cy, start.X-cx)
		to := math.Atan2(end.Y-cy, end.X-cx)
		sweep := positiveAngle(to - from)
		if start.Radius < 0 {
			sweep = positiveAngle(from - to)
		}
		if sweep > math.Pi+1e-8 {
			continue
		}
		lo, hi := math.Min(start.X, end.X), math.Max(start.X, end.X)
		for _, angle := range []float64{0, math.Pi} {
			travel := positiveAngle(angle - from)
			if start.Radius < 0 {
				travel = positiveAngle(from - angle)
			}
			if travel <= sweep+1e-8 {
				x := cx + r*math.Cos(angle)
				lo, hi = math.Min(lo, x), math.Max(hi, x)
			}
		}
		return lo, hi, nil
	}
	return 0, 0, fmt.Errorf("invalid contour arc orientation")
}

func positiveAngle(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a
}

func hasWeldPreparation(data string) bool {
	if data == "" {
		return false
	}
	fields := strings.Fields(data)
	if len(fields) != 2 && len(fields) != 4 {
		return true
	}
	for _, field := range fields {
		v, err := number(field, 0)
		if err != nil || v != 0 {
			return true
		}
	}
	return false
}

// Package dstv_parser reads DSTV/NC1 part files. All lengths are in millimetres.
package dstv_parser

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// Part is one DSTV part description. Blocks retain their original data lines so
// unsupported and vendor-specific geometry is never silently discarded.
type Part struct {
	Header   Header
	Blocks   []Block
	Comments []Comment
}

type Header struct {
	Order, Drawing, Phase, Piece, SteelGrade string
	Quantity                                 int
	Profile, ProfileCode                     string
	Length, SawLength                        float64
	HasSawLength                             bool
	Height, FlangeWidth, FlangeThickness     float64
	WebThickness, Radius                     float64
	WeightPerMeter, PaintAreaPerMeter        float64
	WebStartCut, WebEndCut                   float64
	FlangeStartCut, FlangeEndCut             float64
	Info                                     []string
}

// StockLength is the requested length for preliminary linear nesting. The
// caller must separately account for kerf, clamping, contours and machine rules.
func (h Header) StockLength() float64 {
	if h.HasSawLength {
		return h.SawLength
	}
	return h.Length
}

type Block struct {
	Code string
	Line int
	Data []DataLine
}

type DataLine struct {
	Line int
	Text string // Original line, without its line ending.
}

type Comment struct {
	Line int
	Text string // Text after the leading **.
}

// Parse reads one part. It accepts LF and CRLF, comments between records, and
// additional block codes. Unknown blocks remain available in Part.Blocks.
func Parse(r io.Reader) (*Part, error) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 16*1024*1024)
	part := &Part{}
	var headerLines []DataLine
	var current *Block
	state := 0 // 0: before ST, 1: header, 2: blocks, 3: after EN
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSuffix(s.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// Empty header lines can be optional free-text fields.
			if state == 1 && len(headerLines) >= 20 {
				headerLines = append(headerLines, DataLine{lineNo, ""})
			}
			continue
		}
		if strings.HasPrefix(line, "**") {
			part.Comments = append(part.Comments, Comment{lineNo, strings.TrimPrefix(line, "**")})
			continue
		}
		if code, ok := blockCode(line); ok {
			switch code {
			case "ST":
				if state != 0 {
					return nil, fmt.Errorf("line %d: unexpected ST", lineNo)
				}
				state = 1
			case "EN":
				if state == 0 || state == 3 {
					return nil, fmt.Errorf("line %d: unexpected EN", lineNo)
				}
				state = 3
			default:
				if state == 0 || state == 3 {
					return nil, fmt.Errorf("line %d: block %s outside part", lineNo, code)
				}
				state = 2
				part.Blocks = append(part.Blocks, Block{Code: code, Line: lineNo})
				current = &part.Blocks[len(part.Blocks)-1]
			}
			continue
		}
		switch state {
		case 1:
			headerLines = append(headerLines, DataLine{lineNo, trimmed})
		case 2:
			current.Data = append(current.Data, DataLine{lineNo, line})
		default:
			return nil, fmt.Errorf("line %d: data outside part", lineNo)
		}
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read DSTV: %w", err)
	}
	if state != 3 {
		return nil, fmt.Errorf("line %d: missing EN", lineNo)
	}
	h, err := parseHeader(headerLines)
	if err != nil {
		return nil, err
	}
	part.Header = h
	return part, nil
}

func ParseFile(path string) (*Part, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

func blockCode(line string) (string, bool) {
	if len(line) < 2 || strings.TrimSpace(line[2:]) != "" {
		return "", false
	}
	a, b := line[0], line[1]
	if a >= 'A' && a <= 'Z' && b >= 'A' && b <= 'Z' {
		return line[:2], true
	}
	if (a == 'E' || a == 'B' || a == 'S' || a == 'A' || a == 'I' || a == 'P' || a == 'K') && b >= '0' && b <= '9' {
		return line[:2], true
	}
	return "", false
}

func parseHeader(lines []DataLine) (Header, error) {
	var h Header
	if len(lines) < 20 {
		return h, fmt.Errorf("ST header: expected at least 20 fields, got %d", len(lines))
	}
	if len(lines) > 24 {
		return h, fmt.Errorf("ST header: expected at most 24 fields, got %d", len(lines))
	}
	value := func(i int) string { return strings.TrimSpace(lines[i].Text) }
	h.Order, h.Drawing, h.Phase, h.Piece, h.SteelGrade = value(0), value(1), value(2), value(3), value(4)
	var err error
	h.Quantity, err = strconv.Atoi(value(5))
	if err != nil || h.Quantity <= 0 {
		return h, fmt.Errorf("line %d: invalid quantity %q", lines[5].Line, value(5))
	}
	h.Profile, h.ProfileCode = value(6), value(7)
	if h.Profile == "" || h.ProfileCode == "" {
		return h, fmt.Errorf("ST header: profile and profile code are required")
	}
	lengths := strings.Fields(value(8))
	if len(lengths) < 1 || len(lengths) > 2 {
		return h, fmt.Errorf("line %d: expected length and optional saw length", lines[8].Line)
	}
	h.Length, err = number(lengths[0], lines[8].Line)
	if err != nil || h.Length <= 0 {
		return h, fmt.Errorf("line %d: invalid length %q", lines[8].Line, value(8))
	}
	if len(lengths) == 2 {
		h.SawLength, err = number(lengths[1], lines[8].Line)
		if err != nil || h.SawLength <= 0 {
			return h, fmt.Errorf("line %d: invalid saw length %q", lines[8].Line, lengths[1])
		}
		h.HasSawLength = true
	}
	fields := []*float64{&h.Height, &h.FlangeWidth, &h.FlangeThickness, &h.WebThickness, &h.Radius, &h.WeightPerMeter, &h.PaintAreaPerMeter, &h.WebStartCut, &h.WebEndCut, &h.FlangeStartCut, &h.FlangeEndCut}
	for i, target := range fields {
		*target, err = number(value(i+9), lines[i+9].Line)
		if err != nil {
			return h, err
		}
	}
	for _, line := range lines[20:] {
		h.Info = append(h.Info, strings.TrimSpace(line.Text))
	}
	return h, nil
}

func number(value string, line int) (float64, error) {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("line %d: invalid number %q", line, value)
	}
	return n, nil
}

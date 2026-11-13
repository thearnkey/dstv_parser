package dstv_parser

import (
	"strings"
	"testing"
)

const example = "ST\r\n** sample\r\n  JOB-7\r\n  DWG-1\r\n  PHASE-1\r\n  P-12\r\n  S355\r\n  3\r\n  HEB400\r\n  I\r\n  2000.00 2030.00\r\n  400.00\r\n  300.00\r\n  24.00\r\n  13.50\r\n  27.00\r\n  155.00\r\n  1.93\r\n  0.00\r\n  0.00\r\n  0.00\r\n  0.00\r\n  beam\r\nBO\r\n  v 450.00o 280.00 24.00\r\nXX\r\n  vendor data\r\nEN\r\n"

func TestParse(t *testing.T) {
	p, err := Parse(strings.NewReader(example))
	if err != nil {
		t.Fatal(err)
	}
	if p.Header.Piece != "P-12" || p.Header.Quantity != 3 || p.Header.Profile != "HEB400" || p.Header.StockLength() != 2030 {
		t.Fatalf("unexpected header: %+v", p.Header)
	}
	if len(p.Blocks) != 2 || p.Blocks[0].Code != "BO" || p.Blocks[0].Data[0].Text != "  v 450.00o 280.00 24.00" || p.Blocks[1].Code != "XX" {
		t.Fatalf("blocks were not preserved: %+v", p.Blocks)
	}
	if len(p.Comments) != 1 || p.Comments[0].Text != " sample" {
		t.Fatalf("comments: %+v", p.Comments)
	}
}

func TestInvalidHeaderAndTerminator(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"missing EN", strings.TrimSuffix(example, "EN\r\n"), "missing EN"},
		{"bad quantity", strings.Replace(example, "  3\r\n  HEB400", "  zero\r\n  HEB400", 1), "invalid quantity"},
		{"bad number", strings.Replace(example, "  400.00\r\n", "  nope\r\n", 1), "invalid number"},
		{"trailing data", example + "junk\n", "data outside part"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestLengthWithoutSawLength(t *testing.T) {
	input := strings.Replace(example, "2000.00 2030.00", "2000.00", 1)
	p, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if p.Header.HasSawLength || p.Header.StockLength() != 2000 {
		t.Fatalf("unexpected length: %+v", p.Header)
	}
}

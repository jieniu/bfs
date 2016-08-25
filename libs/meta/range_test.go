package meta

import (
	"testing"
)

func TestConvertRange(t *testing.T) {
	var r = Range{}
	r.Start = 99
	r.End = 0
	r.ConvertRange(50)
	if r.Start != 49 || r.End != 49 {
		t.Errorf("convert range error start=%d, end=%d", r.Start, r.End)
	}

	r.Start = 99
	r.End = 200
	r.ConvertRange(50)
	if r.Start != 49 || r.End != 49 {
		t.Errorf("convert range error start=%d, end=%d", r.Start, r.End)

	}

	r.Start = 50
	r.End = 200
	r.ConvertRange(50)
	if r.Start != 0 || r.End != 49 {
		t.Errorf("convert range error start=%d, end=%d", r.Start, r.End)

	}

	r.Start = 51
	r.End = 50
	r.ConvertRange(50)
	if r.Start != 1 || r.End != 49 {
		t.Errorf("convert range error start=%d, end=%d", r.Start, r.End)

	}
}

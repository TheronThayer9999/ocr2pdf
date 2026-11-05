package ocr

import "testing"

const sampleTSV = `level	page_num	block_num	par_num	line_num	word_num	left	top	width	height	conf	text
1	1	0	0	0	0	0	0	987	826	-1	
4	1	1	1	1	0	296	79	397	35	-1	
5	1	1	1	1	1	296	88	61	26	92.3	KỊCH
5	1	1	1	1	2	369	79	57	30	92.9	BẢN
5	1	1	1	1	3	436	88	64	21	12.0	TEST
4	1	1	1	2	0	250	133	489	20	-1	
5	1	1	1	2	1	250	135	71	14	96.5	Checklist
`

func TestTSVParserGroupsWordsIntoLines(t *testing.T) {
	lines, err := NewTSVParser(0).Parse(sampleTSV)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("số dòng = %d, muốn 2", len(lines))
	}

	first := lines[0]
	if first.Text != "KỊCH BẢN TEST" {
		t.Errorf("text dòng 1 = %q", first.Text)
	}
	if first.Top != 79 || first.Height != 35 {
		t.Errorf("hộp dòng 1 = top %d height %d", first.Top, first.Height)
	}
	if len(first.Words) != 3 {
		t.Fatalf("số từ dòng 1 = %d, muốn 3", len(first.Words))
	}
	if w := first.Words[0]; w.Left != 296 || w.Top != 88 || w.Width != 61 || w.Height != 26 || w.Conf != 92.3 {
		t.Errorf("từ đầu = %+v", w)
	}
}

func TestTSVParserRespectsMinConfidence(t *testing.T) {
	lines, err := NewTSVParser(50).Parse(sampleTSV)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("số dòng = %d, muốn 2", len(lines))
	}
	if got := lines[0].Text; got != "KỊCH BẢN" {
		t.Errorf("từ conf thấp phải bị bỏ: %q", got)
	}
}

func TestTSVParserWithoutLineLevel(t *testing.T) {
	// Một số psm không xuất level 4: parser phải tự tạo dòng từ hộp của từ.
	const tsv = `level	page_num	block_num	par_num	line_num	word_num	left	top	width	height	conf	text
5	1	1	1	1	1	10	20	30	12	90.0	Hello
5	1	1	1	1	2	45	18	35	14	90.0	World
`
	lines, err := NewTSVParser(0).Parse(tsv)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("số dòng = %d, muốn 1", len(lines))
	}
	// Hai từ: 20..32 và 18..32 -> hộp dòng là 18..32 (top 18, height 14).
	if lines[0].Top != 18 || lines[0].Height != 14 {
		t.Errorf("hộp dòng = top %d height %d, muốn 18/14", lines[0].Top, lines[0].Height)
	}
	if lines[0].Text != "Hello World" {
		t.Errorf("text = %q", lines[0].Text)
	}
}

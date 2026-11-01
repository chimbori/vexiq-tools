package iqsync

import (
	"encoding/json"
	"testing"
)

func TestNewIQPythonMatchesSampleFields(t *testing.T) {
	out, err := NewIQPython([]byte("print(1)\n"))
	if err != nil {
		t.Fatal(err)
	}
	var f IQPython
	if err := json.Unmarshal(out, &f); err != nil {
		t.Fatal(err)
	}
	if f.Platform != "IQ" || f.HardwareTarget != "brain" || f.FileFormat != "2.0.0" {
		t.Fatalf("unexpected fields: %+v", f)
	}
	if f.TextContent == nil || *f.TextContent != "print(1)\n" {
		t.Fatalf("textContent = %v", f.TextContent)
	}
}

func TestPythonToIQReplacesOnlyText(t *testing.T) {
	doc, err := NewIQPython([]byte("a\n"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := PythonToIQ(doc, []byte("b\r\nc\r"))
	if err != nil {
		t.Fatal(err)
	}
	back, err := IQToPython(out)
	if err != nil || string(back) != "b\nc\n" {
		t.Fatalf("round trip = %q, %v", back, err)
	}
	// Re-encoding the old file with only textContent changed must give the same bytes.
	var want IQPython
	if err := json.Unmarshal(doc, &want); err != nil {
		t.Fatal(err)
	}
	setText(&want, []byte("b\nc\n"))
	expected, err := encode(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(expected) != string(out) {
		t.Fatalf("output differs:\n%s\nwant:\n%s", out, expected)
	}
}

func TestTextEncodingRoundTrip(t *testing.T) {
	cases := []string{
		"",
		"tab\there",
		"quote \" and backslash \\",
		"html <a> & </a>",
		"unicode é 世界",
		"control \x01 char",
	}
	for _, src := range cases {
		out, err := NewIQPython([]byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		back, err := IQToPython(out)
		if err != nil || string(back) != src {
			t.Fatalf("%q: got %q, %v", src, back, err)
		}
	}
}

func TestInvalidInputsRejected(t *testing.T) {
	if _, err := NewIQPython([]byte{0xff, 0xfe}); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	if _, err := IQToPython([]byte(`{"mode": "Text"}`)); err == nil {
		t.Fatal("missing textContent accepted")
	}
	if _, err := IQToPython([]byte(`{"textContent": 5}`)); err == nil {
		t.Fatal("non-string textContent accepted")
	}
}

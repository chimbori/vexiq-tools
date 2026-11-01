package iqsync

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

// IQPython mirrors the fields of a VEXcode IQ .iqpython file, in the order VEXcode writes them.
// Fields that IQ Sync never changes are kept so the file stays a valid VEXcode project.
type IQPython struct {
	Mode             string           `json:"mode"`
	HardwareTarget   string           `json:"hardwareTarget"`
	TextContent      *string          `json:"textContent"`
	TextLanguage     string           `json:"textLanguage"`
	RobotConfig      []any            `json:"robotConfig"`
	Slot             int              `json:"slot"`
	Platform         string           `json:"platform"`
	SdkVersion       string           `json:"sdkVersion"`
	AppVersion       string           `json:"appVersion"`
	FileFormat       string           `json:"fileFormat"`
	TargetBrainGen   string           `json:"targetBrainGen"`
	V5SoundsEnabled  bool             `json:"v5SoundsEnabled"`
	AIVisionSettings AIVisionSettings `json:"aiVisionSettings"`
}

type AIVisionSettings struct {
	Colors               []any `json:"colors"`
	Codes                []any `json:"codes"`
	Tags                 bool  `json:"tags"`
	AIObjects            bool  `json:"AIObjects"`
	AIObjectModel        []any `json:"AIObjectModel"`
	AIModelDropDownValue any   `json:"aiModelDropDownValue"`
}

// newTemplate returns the contents of VEXcode's Sample.iqpython, with an empty textContent.
func newTemplate() IQPython {
	return IQPython{
		Mode:           "Text",
		HardwareTarget: "brain",
		TextLanguage:   "python",
		RobotConfig:    []any{},
		Slot:           0,
		Platform:       "IQ",
		SdkVersion:     "20230818.11.00.00",
		AppVersion:     "4.68.0",
		FileFormat:     "2.0.0",
		TargetBrainGen: "Second",
		AIVisionSettings: AIVisionSettings{
			Colors:               []any{},
			Codes:                []any{},
			Tags:                 true,
			AIObjects:            true,
			AIObjectModel:        []any{},
			AIModelDropDownValue: nil,
		},
	}
}

// NewIQPython returns a new .iqpython file that holds py, built from the template (FR-4.1).
func NewIQPython(py []byte) ([]byte, error) {
	if !utf8.Valid(py) {
		return nil, errors.New("python source is not valid UTF-8")
	}
	f := newTemplate()
	setText(&f, py)
	return encode(f)
}

// PythonToIQ returns doc, an existing .iqpython file, with its textContent replaced by py (FR-3.4).
func PythonToIQ(doc, py []byte) ([]byte, error) {
	if !utf8.Valid(py) {
		return nil, errors.New("python source is not valid UTF-8")
	}
	var f IQPython
	if err := json.Unmarshal(doc, &f); err != nil {
		return nil, err
	}
	setText(&f, py)
	return encode(f)
}

// IQToPython returns the Python source stored in the textContent field of doc, with LF line
// endings (FR-3.9).
func IQToPython(doc []byte) ([]byte, error) {
	var f IQPython
	if err := json.Unmarshal(doc, &f); err != nil {
		return nil, err
	}
	if f.TextContent == nil {
		return nil, errors.New("no textContent field")
	}
	return []byte(normalizeLF(*f.TextContent)), nil
}

func setText(f *IQPython, py []byte) {
	text := normalizeLF(string(py))
	f.TextContent = &text
}

// encode renders f as compact JSON.
func encode(f IQPython) ([]byte, error) {
	return json.Marshal(f)
}

func normalizeLF(s string) string {
	return strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
}

// Package waybar renders PulseAudio source state as Waybar custom-module output.
package waybar

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/edmonl/waybar-pulseaudio-sources/pulse"
)

// Output is one Waybar custom-module JSON update.
type Output struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip,omitempty"`
	Class      string `json:"class,omitempty"`
	Percentage *int   `json:"percentage,omitempty"`
}

type formatData struct {
	Index  int
	Name   string
	Desc   string
	Muted  bool
	Volume int
	State  string
	// Available indicates whether source data is available.
	Available bool
}

// Formatter renders PulseAudio source and status data as Waybar output.
type Formatter struct {
	text    *template.Template
	class   *template.Template
	tooltip *template.Template
}

// NewFormatter creates a Waybar formatter from text, class, and tooltip templates.
func NewFormatter(text, class, tooltip string) (*Formatter, error) {
	textTmpl, err := newTemplate("text", text)
	if err != nil {
		return nil, err
	}
	classTmpl, err := newTemplate("class", class)
	if err != nil {
		return nil, err
	}
	tooltipTmpl, err := newTemplate("tooltip", tooltip)
	if err != nil {
		return nil, err
	}

	return &Formatter{
		text:    textTmpl,
		class:   classTmpl,
		tooltip: tooltipTmpl,
	}, nil
}

// State renders a PulseAudio source as Waybar output.
func (f *Formatter) State(source *pulse.Source) Output {
	data := formatData{
		Index:     int(source.Index),
		Name:      source.Name,
		Desc:      source.Description,
		Muted:     source.Muted,
		Volume:    source.Volume,
		Available: true,
	}

	if source.Muted {
		data.State = "muted"
	}

	out, err := f.format(data)
	if err != nil {
		return formatErrorOutput(err)
	}

	percentage := data.Volume
	out.Percentage = &percentage
	return *out
}

// Unavailable renders a PulseAudio availability failure as Waybar output.
func (f *Formatter) Unavailable(err error) Output {
	out, formatErr := f.format(formatData{
		Index: -1,
		Desc:  err.Error(),
		State: "unavailable",
	})
	if formatErr != nil {
		return formatErrorOutput(formatErr)
	}

	return *out
}

// Error renders a non-availability failure as Waybar output.
func (f *Formatter) Error(err error) Output {
	out, formatErr := f.format(formatData{
		Index: -1,
		Desc:  err.Error(),
		State: "error",
	})
	if formatErr != nil {
		return formatErrorOutput(formatErr)
	}

	return *out
}

func capitalize(value string) string {
	if value == "" {
		return ""
	}

	first, size := utf8.DecodeRuneInString(value)
	return string(unicode.ToUpper(first)) + value[size:]
}

var templateFuncs = template.FuncMap{
	"capitalize": capitalize,
}

func newTemplate(name, tmplContent string) (*template.Template, error) {
	if strings.TrimSpace(tmplContent) == "" {
		return nil, fmt.Errorf("%v template must not be empty", name)
	}

	tmpl, err := template.New(name).Option("missingkey=error").Funcs(templateFuncs).Parse(tmplContent)
	if err != nil {
		return nil, fmt.Errorf("parse %v template: %w", name, err)
	}

	return tmpl, nil
}

// This returns the raw error.
func runTemplate(tmpl *template.Template, data formatData) (string, error) {
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", err
	}

	return output.String(), nil
}

func (f *Formatter) format(data formatData) (*Output, error) {
	text, err := runTemplate(f.text, data)
	if err != nil {
		return nil, formatError("text", err)
	}
	class, err := runTemplate(f.class, data)
	if err != nil {
		return nil, formatError("class", err)
	}
	tooltip, err := runTemplate(f.tooltip, data)
	if err != nil {
		return nil, formatError("tooltip", err)
	}

	output := Output{
		Text:    text,
		Tooltip: tooltip,
		Class:   class,
	}
	return &output, nil
}

// The returned error is internal and not for showing to users.
func formatError(name string, err error) error {
	return fmt.Errorf("%v: %w", name, err)
}

// The error this takes must be the return of formatError.
func formatErrorOutput(err error) Output {
	return Output{
		Text:    "Error",
		Tooltip: fmt.Sprintf("Failed to format %v", err),
		Class:   "error",
	}
}

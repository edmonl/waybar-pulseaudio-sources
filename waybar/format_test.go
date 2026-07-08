package waybar_test

import (
	"strings"
	"testing"

	"github.com/edmonl/waybar-pulseaudio-sources/pulse"
	"github.com/edmonl/waybar-pulseaudio-sources/waybar"
)

const defaultTextTemplate = "{{or (.State | capitalize) (print .Volume `%`)}}"
const defaultClassTemplate = "{{.State}}"
const defaultTooltipTemplate = "{{.Desc}}"

func TestFormatterDefault(t *testing.T) {
	formatter, err := waybar.NewFormatter(defaultTextTemplate, defaultClassTemplate, defaultTooltipTemplate)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.State(&pulse.Source{
		Description: "USB Microphone",
		Volume:      65,
	})

	if got.Text != "65%" {
		t.Fatalf("State().Text = %q, want %q", got.Text, "65%")
	}
	if got.Class != "" {
		t.Fatalf("State().Class = %q, want empty", got.Class)
	}
	if got.Tooltip != "USB Microphone" {
		t.Fatalf("State().Tooltip = %q, want %q", got.Tooltip, "USB Microphone")
	}
	if got.Percentage == nil || *got.Percentage != 65 {
		t.Fatalf("State().Percentage = %v, want %v", got.Percentage, 65)
	}
}

func TestFormatterFields(t *testing.T) {
	formatter, err := waybar.NewFormatter("{{.Index}}|{{.Name}}|{{.Desc}}|{{.Muted}}|{{.Volume}}|{{.State}}|{{.Available}}", "{{.State}}", "{{.Desc}}")
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.State(&pulse.Source{
		Index:       7,
		Name:        "alsa_input.usb",
		Description: "USB Microphone",
		Muted:       true,
		Volume:      42,
	})

	want := "7|alsa_input.usb|USB Microphone|true|42|muted|true"
	if got.Text != want {
		t.Fatalf("State().Text = %q, want %q", got.Text, want)
	}
	if got.Class != "muted" {
		t.Fatalf("State().Class = %q, want %q", got.Class, "muted")
	}
}

func TestFormatterCapitalizeTemplateFunction(t *testing.T) {
	formatter, err := waybar.NewFormatter("{{capitalize .State}}", "{{capitalize .State}}", "{{capitalize .Desc}}")
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.Unavailable(errTest("offline"))

	if got.Text != "Unavailable" {
		t.Fatalf("Unavailable().Text = %q, want %q", got.Text, "Unavailable")
	}
	if got.Class != "Unavailable" {
		t.Fatalf("Unavailable().Class = %q, want %q", got.Class, "Unavailable")
	}
	if got.Tooltip != "Offline" {
		t.Fatalf("Unavailable().Tooltip = %q, want %q", got.Tooltip, "Offline")
	}
}

func TestFormatterCapitalizeTemplateFunctionEmptyString(t *testing.T) {
	formatter, err := waybar.NewFormatter("{{capitalize .State}}{{.Volume}}%", "{{capitalize .State}}", "{{.Desc}}")
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.State(&pulse.Source{
		Description: "USB Microphone",
		Volume:      65,
	})

	if got.Text != "65%" {
		t.Fatalf("State().Text = %q, want %q", got.Text, "65%")
	}
	if got.Class != "" {
		t.Fatalf("State().Class = %q, want empty", got.Class)
	}
}

func TestFormatterRejectsMalformedTemplate(t *testing.T) {
	if _, err := waybar.NewFormatter("{{", defaultClassTemplate, defaultTooltipTemplate); err == nil {
		t.Fatal("NewFormatter() error = nil, want error")
	}
}

func TestFormatterRejectsEmptyTemplates(t *testing.T) {
	for _, format := range []string{"", "   "} {
		t.Run(format, func(t *testing.T) {
			if _, err := waybar.NewFormatter(format, defaultClassTemplate, defaultTooltipTemplate); err == nil {
				t.Fatal("NewFormatter() error = nil, want error")
			}
		})
	}
}

func TestFormatterError(t *testing.T) {
	formatter, err := waybar.NewFormatter("{{.Index}}|{{.Desc}}|{{.Available}}", "{{.State}}", "{{.Desc}}")
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.Error(errTest("boom"))

	if got.Text != "-1|boom|false" {
		t.Fatalf("Error().Text = %q, want %q", got.Text, "-1|boom|false")
	}
	if got.Class != "error" {
		t.Fatalf("Error().Class = %q, want %q", got.Class, "error")
	}
	if got.Tooltip != "boom" {
		t.Fatalf("Error().Tooltip = %q, want %q", got.Tooltip, "boom")
	}
	if got.Percentage != nil {
		t.Fatalf("Error().Percentage = %v, want nil", got.Percentage)
	}
}

func TestFormatterUnavailableIndex(t *testing.T) {
	formatter, err := waybar.NewFormatter("{{.Index}}|{{.Desc}}|{{.Available}}", "{{.State}}", "{{.Desc}}")
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.Unavailable(errTest("offline"))

	if got.Text != "-1|offline|false" {
		t.Fatalf("Unavailable().Text = %q, want %q", got.Text, "-1|offline|false")
	}
	if got.Class != "unavailable" {
		t.Fatalf("Unavailable().Class = %q, want %q", got.Class, "unavailable")
	}
	if got.Tooltip != "offline" {
		t.Fatalf("Unavailable().Tooltip = %q, want %q", got.Tooltip, "offline")
	}
	if got.Percentage != nil {
		t.Fatalf("Unavailable().Percentage = %v, want nil", got.Percentage)
	}
}

func TestFormatterMutedDefault(t *testing.T) {
	formatter, err := waybar.NewFormatter(defaultTextTemplate, defaultClassTemplate, defaultTooltipTemplate)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.State(&pulse.Source{
		Description: "USB Microphone",
		Muted:       true,
		Volume:      42,
	})

	if got.Text != "Muted" {
		t.Fatalf("State().Text = %q, want %q", got.Text, "Muted")
	}
	if got.Class != "muted" {
		t.Fatalf("State().Class = %q, want %q", got.Class, "muted")
	}
	if got.Tooltip != "USB Microphone" {
		t.Fatalf("State().Tooltip = %q, want %q", got.Tooltip, "USB Microphone")
	}
	if got.Percentage == nil || *got.Percentage != 42 {
		t.Fatalf("State().Percentage = %v, want %v", got.Percentage, 42)
	}
}

func TestFormatterForcesErrorOutputOnExecutionError(t *testing.T) {
	formatter, err := waybar.NewFormatter("{{slice .Name 0 .Index}}", defaultClassTemplate, defaultTooltipTemplate)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.State(&pulse.Source{
		Index:       7,
		Name:        "abc",
		Description: "USB Microphone",
		Volume:      42,
	})

	if got.Text != "Error" {
		t.Fatalf("State().Text = %q, want %q", got.Text, "Error")
	}
	if got.Class != "error" {
		t.Fatalf("State().Class = %q, want %q", got.Class, "error")
	}
	if !strings.Contains(got.Tooltip, "Failed to format text") {
		t.Fatalf("State().Tooltip = %q, want execution error detail", got.Tooltip)
	}
	if got.Percentage != nil {
		t.Fatalf("State().Percentage = %v, want nil", got.Percentage)
	}
}

func TestFormatterForcesErrorOutputOnMissingField(t *testing.T) {
	formatter, err := waybar.NewFormatter("{{.Description}}", defaultClassTemplate, defaultTooltipTemplate)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	got := formatter.State(&pulse.Source{
		Description: "USB Microphone",
		Volume:      42,
	})

	if got.Text != "Error" {
		t.Fatalf("State().Text = %q, want %q", got.Text, "Error")
	}
	if got.Class != "error" {
		t.Fatalf("State().Class = %q, want %q", got.Class, "error")
	}
	if !strings.Contains(got.Tooltip, "Description") {
		t.Fatalf("State().Tooltip = %q, want missing field detail", got.Tooltip)
	}
	if got.Percentage != nil {
		t.Fatalf("State().Percentage = %v, want nil", got.Percentage)
	}
}

type errTest string

func (e errTest) Error() string {
	return string(e)
}

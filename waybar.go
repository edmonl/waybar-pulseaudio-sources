package main

import (
	"strconv"
)

const (
	microphoneIcon      = ""
	mutedMicrophoneIcon = ""
)

type waybarOutput struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip,omitempty"`
	Class      string `json:"class,omitempty"`
	Percentage int    `json:"percentage"`
}

func waybarState(source *Source) waybarOutput {
	icon := microphoneIcon
	class := "source"
	if source.Muted {
		icon = mutedMicrophoneIcon
		class = "muted"
	}

	return waybarOutput{
		Text:       strconv.Itoa(source.Volume) + "% " + icon,
		Tooltip:    source.Description,
		Class:      class,
		Percentage: source.Volume,
	}
}

func waybarDefaultSourceNotFound() waybarOutput {
	return waybarOutput{
		Text:    "No source " + mutedMicrophoneIcon,
		Tooltip: "Default source not found",
		Class:   "unavailable",
	}
}

func waybarError(err error) waybarOutput {
	return waybarOutput{
		Text:    "Error " + mutedMicrophoneIcon,
		Tooltip: err.Error(),
		Class:   "error",
	}
}

func waybarUnavailable(err error) waybarOutput {
	return waybarOutput{
		Text:    "Unavailable " + mutedMicrophoneIcon,
		Tooltip: err.Error(),
		Class:   "unavailable",
	}
}

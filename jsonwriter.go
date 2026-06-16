package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type jsonWriter struct {
	writer   *bufio.Writer
	lastLine string
}

func newJSONWriter() *jsonWriter {
	return &jsonWriter{
		writer: bufio.NewWriter(os.Stdout),
	}
}

func (w *jsonWriter) Emit(output any) error {
	line, err := json.Marshal(output)
	if err != nil {
		return err
	}

	return w.emit(string(line))
}

func (w *jsonWriter) EmitIfChanged(output any) error {
	line, err := json.Marshal(output)
	if err != nil {
		return err
	}

	next := string(line)
	if next == w.lastLine {
		return nil
	}

	return w.emit(next)
}

func (w *jsonWriter) emit(next string) error {
	if _, err := fmt.Fprintln(w.writer, next); err != nil {
		return err
	}
	if err := w.writer.Flush(); err != nil {
		return err
	}

	w.lastLine = next
	return nil
}

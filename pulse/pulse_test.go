package pulse

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectNextSource(t *testing.T) {
	dir := t.TempDir()
	testSource := filepath.Join(dir, "select_next_source_test.c")
	testBinary := filepath.Join(dir, "select_next_source_test")

	if err := os.WriteFile(testSource, []byte(selectNextSourceTestC), 0o644); err != nil {
		t.Fatal(err)
	}

	cflags := pkgConfig(t, "--cflags")
	libs := pkgConfig(t, "--libs")
	args := append([]string{}, cflags...)
	args = append(args, "-I.", "pulse.c", testSource, "-o", testBinary)
	args = append(args, libs...)

	compile := exec.Command("cc", args...)
	compile.Dir = "."
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile C selector test: %v\n%s", err, output)
	}

	run := exec.Command(testBinary)
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("run C selector test: %v\n%s", err, output)
	}
}

func pkgConfig(t *testing.T, flag string) []string {
	t.Helper()

	cmd := exec.Command("pkg-config", flag, "libpulse")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pkg-config %s libpulse failed: %v\n%s", flag, err, output)
	}
	return strings.Fields(string(output))
}

const selectNextSourceTestC = `
#include "pulse.h"

#include <stdio.h>
#include <string.h>

static int assert_selected(const char *name, pulse_source_ref_t *sources,
                           int count, const char *default_source_name,
                           const char *want_name) {
  const pulse_source_ref_t *got =
      pulse_select_next_source(sources, count, default_source_name);
  if (!got) {
    fprintf(stderr, "%s: got NULL, want %s\n", name, want_name);
    return 1;
  }
  if (strcmp(got->name, want_name) != 0) {
    fprintf(stderr, "%s: got %s, want %s\n", name, got->name, want_name);
    return 1;
  }
  return 0;
}

static int assert_none(const char *name, pulse_source_ref_t *sources, int count,
                       const char *default_source_name) {
  const pulse_source_ref_t *got =
      pulse_select_next_source(sources, count, default_source_name);
  if (got) {
    fprintf(stderr, "%s: got %s, want NULL\n", name, got->name);
    return 1;
  }
  return 0;
}

int main(void) {
  pulse_source_ref_t next_higher[] = {
      {.index = 30, .name = "mic-c"},
      {.index = 10, .name = "mic-a"},
      {.index = 20, .name = "mic-b"},
  };
  if (assert_selected("next higher", next_higher, 3, "mic-a", "mic-b")) {
    return 1;
  }

  pulse_source_ref_t wraps[] = {
      {.index = 10, .name = "mic-a"},
      {.index = 30, .name = "mic-c"},
      {.index = 20, .name = "mic-b"},
  };
  if (assert_selected("wrap", wraps, 3, "mic-c", "mic-a")) {
    return 1;
  }

  pulse_source_ref_t monitor_anchor[] = {
      {.index = 10, .name = "mic-a"},
      {.index = 20, .name = "output.monitor"},
      {.index = 30, .name = "mic-b"},
  };
  if (assert_selected("monitor anchor", monitor_anchor, 3, "output.monitor",
                      "mic-b")) {
    return 1;
  }

  pulse_source_ref_t monitor_anchor_wrap[] = {
      {.index = 10, .name = "mic-a"},
      {.index = 20, .name = "mic-b"},
      {.index = 30, .name = "output.monitor"},
  };
  if (assert_selected("monitor anchor wrap", monitor_anchor_wrap, 3,
                      "output.monitor", "mic-a")) {
    return 1;
  }

  pulse_source_ref_t missing_default[] = {
      {.index = 20, .name = "mic-b"},
      {.index = 10, .name = "mic-a"},
  };
  if (assert_selected("missing default", missing_default, 2, "missing",
                      "mic-a")) {
    return 1;
  }

  pulse_source_ref_t single[] = {
      {.index = 10, .name = "mic-a"},
  };
  if (assert_selected("single source", single, 1, "mic-a", "mic-a")) {
    return 1;
  }

  pulse_source_ref_t monitors_only[] = {
      {.index = 10, .name = "output-a.monitor"},
      {.index = 20, .name = "output-b.monitor"},
  };
  if (assert_none("monitors only", monitors_only, 2, "output-a.monitor")) {
    return 1;
  }

  return 0;
}
`

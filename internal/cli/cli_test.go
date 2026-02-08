package cli

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/clutchski/dottie/internal/console"
	"github.com/clutchski/dottie/internal/hooks"
	"github.com/clutchski/dottie/internal/link"
	"github.com/stretchr/testify/assert"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestIndentLines(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{"single line", "this is broken\n", "    this is broken\n"},
		{"multiple lines", "line1\nline2\n", "    line1\n    line2\n"},
		{"no trailing newline", "hello", "    hello\n"},
		{"blank lines preserved", "line1\n\nline2\n", "    line1\n\n    line2\n"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, indentLines(tc.input, "    "))
		})
	}
}

func makeEvents(evs ...hooks.Event) <-chan hooks.Event {
	ch := make(chan hooks.Event, len(evs))
	for _, e := range evs {
		ch <- e
	}
	close(ch)
	return ch
}

func TestProcessRunHooks_VerboseSuccess(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Duration: 100 * time.Millisecond},
	)
	con := console.New(false)
	var stdout bytes.Buffer
	con.Stdout = &stdout

	processRunHooks(events, true, con, "hooks:")

	out := stripANSI(stdout.String())
	assert.Contains(t, out, "hooks:\n")
	assert.Contains(t, out, "✓ h1 (0.1s)")
	assert.NotContains(t, out, "done:")
	assert.NotContains(t, out, "start:")
}

func TestProcessRunHooks_VerboseFailure(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Phase: "pre-link", Duration: 0, Err: fmt.Errorf("exit 1"), Stderr: []byte("this is broken\n")},
	)
	con := console.New(false)
	var stdout, stderr bytes.Buffer
	con.Stdout = &stdout
	con.Stderr = &stderr

	processRunHooks(events, true, con, "hooks:")

	out := stripANSI(stdout.String())
	assert.Contains(t, out, "x h1 pre-link hook failed (0.0s)")
	assert.NotContains(t, out, "done:")
	errOut := stderr.String()
	assert.Contains(t, errOut, "this is broken")
}

func TestProcessRunHooks_NormalSummary(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Phase: "pre-link", Duration: 0, Err: fmt.Errorf("exit 1"), Stderr: []byte("this is broken\n")},
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h2.sh", Phase: "pre-link", Duration: 100 * time.Millisecond},
	)
	con := console.New(false)
	var stdout, stderr bytes.Buffer
	con.Stdout = &stdout
	con.Stderr = &stderr

	processRunHooks(events, false, con, "hooks:")

	out := stripANSI(stdout.String())
	assert.True(t, strings.HasPrefix(out, "hooks:\n"), "should start with heading")
	assert.Contains(t, out, "✓ h2")
	assert.Contains(t, out, "x h1 pre-link hook failed (0.0s)")
	assert.Contains(t, stderr.String(), "this is broken")
}

func TestProcessRunHooks_DropsStartEvents(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Started, Hook: "/hooks/h1.sh"},
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Duration: 100 * time.Millisecond},
	)
	con := console.New(false)
	var stdout bytes.Buffer
	con.Stdout = &stdout

	processRunHooks(events, true, con, "hooks:")

	out := stripANSI(stdout.String())
	assert.NotContains(t, out, "start:")
}

func TestProcessRunHooks_HeadingOnFirstEvent(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Duration: 100 * time.Millisecond},
	)
	con := console.New(false)
	var stdout bytes.Buffer
	con.Stdout = &stdout

	processRunHooks(events, true, con, "hooks:")

	out := stripANSI(stdout.String())
	assert.True(t, len(out) > 0 && out[:7] == "hooks:\n", "should start with heading")
}

func TestProcessRunHooks_NoHeadingWhenNoEvents(t *testing.T) {
	events := makeEvents() // empty
	con := console.New(false)
	var stdout bytes.Buffer
	con.Stdout = &stdout

	processRunHooks(events, true, con, "hooks:")

	assert.Equal(t, "", stdout.String())
}

func TestProcessHooksCommand_VerboseSuccess(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Duration: 100 * time.Millisecond},
	)

	var stdout bytes.Buffer
	code := processHooksCommand(events, "pre-link", true, &stdout)

	out := stripANSI(stdout.String())
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "✓ h1 (0.1s)")
	assert.NotContains(t, out, "done:")
}

func TestProcessHooksCommand_VerboseFailure(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Duration: 0, Err: fmt.Errorf("exit 1"), Stdout: []byte("stdout output\n")},
	)

	var stdout bytes.Buffer
	code := processHooksCommand(events, "pre-link", true, &stdout)

	out := stripANSI(stdout.String())
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "x h1 pre-link hook failed (0.0s)")
	assert.Contains(t, out, "stdout output")
}

func TestProcessHooksCommand_DropsStartEvents(t *testing.T) {
	events := makeEvents(
		hooks.Event{Kind: hooks.Started, Hook: "/hooks/h1.sh"},
		hooks.Event{Kind: hooks.Done, Hook: "/hooks/h1.sh", Duration: 100 * time.Millisecond},
	)

	var stdout bytes.Buffer
	processHooksCommand(events, "pre-link", true, &stdout)

	out := stripANSI(stdout.String())
	assert.NotContains(t, out, "start:")
}

func TestPrintStatusSummary_AllOk(t *testing.T) {
	statuses := []link.FileInfo{
		{Name: "vimrc", Status: link.FileStatusLinked},
		{Name: "bashrc", Status: link.FileStatusLinked},
	}

	var buf bytes.Buffer
	printStatusSummary(&buf, statuses)

	output := stripANSI(buf.String())
	assert.Equal(t, "dotfiles:\n  \u2713 2 ok\n", output)
}

func TestPrintStatusSummary_WithProblems(t *testing.T) {
	statuses := []link.FileInfo{
		{Name: "vimrc", Status: link.FileStatusLinked},
		{Name: "bashrc", Status: link.FileStatusMissing},
	}

	var buf bytes.Buffer
	printStatusSummary(&buf, statuses)

	output := stripANSI(buf.String())
	assert.Contains(t, output, "dotfiles:\n")
	assert.Contains(t, output, "  \u2713 1 ok\n")
	assert.Contains(t, output, "  x bashrc\n")
	assert.NotContains(t, output, "vimrc")
}

func TestPrintStatusSummary_Empty(t *testing.T) {
	var buf bytes.Buffer
	printStatusSummary(&buf, nil)

	output := buf.String()
	assert.Equal(t, "", output)
}

func TestPrintStatusSummary_AllBad(t *testing.T) {
	statuses := []link.FileInfo{
		{Name: "bashrc", Status: link.FileStatusMissing},
		{Name: "vimrc", Status: link.FileStatusMissing},
	}

	var buf bytes.Buffer
	printStatusSummary(&buf, statuses)

	output := stripANSI(buf.String())
	assert.Contains(t, output, "dotfiles:\n")
	assert.NotContains(t, output, "\u2713")
	assert.Contains(t, output, "  x bashrc\n")
	assert.Contains(t, output, "  x vimrc\n")
}

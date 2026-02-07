package cli

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/clutchski/dottie/internal/link"
	"github.com/stretchr/testify/assert"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
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

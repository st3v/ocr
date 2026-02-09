package main

import (
	"fmt"
	"io"
)

// Reporter handles progress and verbose output.
type Reporter struct {
	w       io.Writer
	verbose bool
}

// NewReporter creates a reporter that writes to w.
// If quiet is true, all output is suppressed.
// If verbose is true, extra details are shown.
func NewReporter(w io.Writer, quiet, verbose bool) *Reporter {
	if quiet {
		return &Reporter{w: io.Discard}
	}
	return &Reporter{w: w, verbose: verbose}
}

// Progress prints a progress message.
func (r *Reporter) Progress(format string, args ...any) {
	fmt.Fprintf(r.w, format, args...)
}

// Verbose prints a message only in verbose mode.
func (r *Reporter) Verbose(format string, args ...any) {
	if r.verbose {
		fmt.Fprintf(r.w, format, args...)
	}
}

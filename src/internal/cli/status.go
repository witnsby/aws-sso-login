package cli

import (
	"fmt"
	"os"
)

// statusKind is the kind of a human-readable status line on stderr.
type statusKind int

const (
	kindStep statusKind = iota
	kindSuccess
	kindWarn
	kindError
)

// Prefixes for status lines: symbols on a terminal, plain ASCII otherwise.
var (
	symbolPrefixes = map[statusKind]string{kindStep: "→", kindSuccess: "✓", kindWarn: "!", kindError: "✗"}
	asciiPrefixes  = map[statusKind]string{kindStep: "->", kindSuccess: "ok:", kindWarn: "warning:", kindError: "error:"}
	prefixColors   = map[statusKind]string{kindStep: "\x1b[2m", kindSuccess: "\x1b[32m", kindWarn: "\x1b[33m", kindError: "\x1b[31m"}
)

const colorReset = "\x1b[0m"

// reporter writes status lines (step, success, warning, error) to the
// stderrOut seam. It is the only place that knows about prefixes and colors.
type reporter struct {
	symbols bool // use →, ✓, !, ✗ instead of ASCII prefixes
	color   bool // color the prefix with ANSI escapes
	quiet   bool // drop step and success lines (process, export)
}

// report is the reporter for the running command. It is replaced in the
// root PersistentPreRunE once the command is known.
var report = &reporter{}

// newReporter builds the reporter for the named command from the stderr
// terminal check, NO_COLOR and TERM. process and export get a quiet
// reporter because their stdout is consumed by other programs.
func newReporter(cmdName string) *reporter {
	tty := stderrIsTerminal()
	return &reporter{
		symbols: tty,
		color:   tty && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb",
		quiet:   cmdName == "process" || cmdName == "export",
	}
}

// prefix returns the (possibly colored) prefix for kind.
func (r *reporter) prefix(kind statusKind) string {
	p := asciiPrefixes[kind]
	if r.symbols {
		p = symbolPrefixes[kind]
	}
	if r.color {
		p = prefixColors[kind] + p + colorReset
	}
	return p
}

// line formats one status line without a trailing newline.
func (r *reporter) line(kind statusKind, msg string) string {
	return r.prefix(kind) + " " + msg
}

func (r *reporter) write(kind statusKind, format string, args ...any) {
	fmt.Fprintln(stderrOut, r.line(kind, fmt.Sprintf(format, args...)))
}

// Step reports an action that starts.
func (r *reporter) Step(format string, args ...any) {
	if r.quiet {
		return
	}
	r.write(kindStep, format, args...)
}

// Success reports a finished result.
func (r *reporter) Success(format string, args ...any) {
	if r.quiet {
		return
	}
	r.write(kindSuccess, format, args...)
}

// Warn reports a warning. Warnings are shown even in quiet mode.
func (r *reporter) Warn(format string, args ...any) {
	r.write(kindWarn, format, args...)
}

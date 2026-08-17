// Package ux implements the [build.Reporter] port with a fancy Lip Gloss
// renderer and a plain ASCII renderer behind one constructor.
//
// [New] selects the renderer from [ColorMode] and [ProgressMode]. Explicit
// always/never values override the environment. [ColorModeAuto] uses this
// precedence (first match wins):
//
//  1. [ColorModeAlways] / [ColorModeNever] (the caller already chose)
//  2. NO_COLOR set to a non-empty value → no color
//  3. TERM=dumb → no color
//  4. w is not a terminal → no color
//  5. otherwise color (fancy renderer)
//
// [ProgressModeAuto] enables progress when w is a TTY; always/never
// override. Progress output never uses ANSI when progress is resolved off.
// The CLI can pass [ProgressModeNever] when stdout is not a TTY. This
// adapter only observes w.
//
// Plain progress is rate-limited: a percentage line is written at most once
// per 200ms, plus the first update and the 100% completion line, so piped
// output is not spammed when a download reports every buffer.
//
// Fancy output, [Summary], [VersionsTable], and [NewLogger] all draw from
// [DefaultPalette]. [NewLogger] is the only logger the CLI uses and follows
// [ColorMode].
//
// [Summary] and [VersionsTable] take the same [ColorMode] and write to an
// [io.Writer]. They never call lipgloss.Fprint, which would re-detect the
// writer and ignore the flag both ways. Styled output goes through
// [colorWriter]; --color=never (even on a TTY) uses plain fmt and emits
// zero ANSI; --color=always into a capture buffer still emits CSI.
package ux

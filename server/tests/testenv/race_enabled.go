//go:build race

// Package testenv exposes build-environment facts that tests need to adapt
// their expectations, such as whether the race detector is enabled.
package testenv

// RaceEnabled reports whether the binary was built with the race detector.
const RaceEnabled = true

//go:build !race

package learning

// raceEnabled reports whether the race detector is active. The race detector
// slows execution by roughly an order of magnitude, which can push a write past
// busy_timeout; the concurrency harness accounts for that.
const raceEnabled = false

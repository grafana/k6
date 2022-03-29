// Package metrics contains various k6 components that deal with metrics and
// thresholds.
package metrics

// TODO: move most things from the stats/ package here

// TODO: maybe even move the outputs to a sub-folder here? it may be worth it to
// do a new Output v2 implementation that uses channels and is more usable and
// easier to write? this way the old extensions can still work for a while, with
// an adapter and a deprecation notice

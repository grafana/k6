// Package env provides types to interact with environment setup.
package env

// LookupFunc defines a function to look up a key from the environment.
type LookupFunc func(key string) (string, bool)

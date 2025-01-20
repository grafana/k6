// Package tests contains integration tests that run k6 commands, and interact
// with standard I/O streams. They're the highest level tests we have, just
// below E2E tests that execute the k6 binary. Since they initialize all
// internal k6 components similarly to how a user would, they're very useful,
// but also very expensive to run.
package tests

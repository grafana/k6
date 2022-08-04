// Package cloudapi contains several things related to the k6 cloud - various
// data and config structures, a REST API client, log streaming logic, etc. They
// are all used in cloud tests (i.e. `k6 cloud`), and in local tests emitting
// their results to the k6 cloud output (i.e. `k6 run --out cloud`).
package cloudapi

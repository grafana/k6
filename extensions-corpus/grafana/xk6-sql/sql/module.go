// Package sql provides a javascript module for performing SQL actions against relational databases.
package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/modules"
)

// ImportPath contains module's JavaScript import path.
const ImportPath = "k6/x/sql"

// New creates a new instance of the extension's JavaScript module.
func New() modules.Module {
	return new(rootModule)
}

// rootModule is the global module object type. It is instantiated once per test
// run and will be used to create `k6/x/sql` module instances for each VU.
type rootModule struct{}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*rootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	instance := &module{}

	instance.exports.Default = instance
	instance.exports.Named = map[string]any{
		"open": instance.Open,
	}

	instance.vu = vu

	return instance
}

// module represents an instance of the JavaScript module for every VU.
type module struct {
	exports modules.Exports
	vu      modules.VU
}

// Exports is representation of ESM exports of a module.
func (mod *module) Exports() modules.Exports {
	return mod.exports
}

// KeyValue is a simple key-value pair.
type KeyValue map[string]any

func asSymbol(value sobek.Value) (*sobek.Symbol, bool) {
	sym, ok := value.(*sobek.Symbol)
	if ok {
		return sym, ok
	}

	obj, ok := value.(*sobek.Object)
	if !ok {
		return nil, false
	}

	valueOf, ok := sobek.AssertFunction(obj.Get("valueOf"))
	if !ok {
		return nil, false
	}

	ret, err := valueOf(obj)
	if err != nil {
		return nil, false
	}

	sym, ok = ret.(*sobek.Symbol)

	return sym, ok
}

// open establishes a connection to the specified database type using
// the provided connection string.
func (mod *module) Open(driverID sobek.Value, connectionString string, opts *options) (*Database, error) {
	driverSym, ok := asSymbol(driverID)

	if !ok {
		return nil, fmt.Errorf("%w: invalid driver parameter type", errUnsupportedDatabase)
	}

	registered, database := lookupDriver(driverSym)
	if !registered {
		return nil, fmt.Errorf("%w: %s", errUnsupportedDatabase, database)
	}

	db, err := sql.Open(database, connectionString)
	if err != nil {
		return nil, err
	}

	if err = opts.apply(db); err != nil {
		return nil, err
	}

	return &Database{db: db, ctx: mod.vu.Context}, nil
}

// Database is a database handle representing a pool of zero or more underlying connections.
type Database struct {
	db  *sql.DB
	ctx func() context.Context
}

// Query executes a query that returns rows, typically a SELECT.
func (dbase *Database) Query(query string, args ...any) ([]KeyValue, error) {
	return dbase.query(dbase.ctx(), query, args...)
}

// QueryWithTimeout executes a query (with a timeout) that returns rows, typically a SELECT.
// The timeout can be specified as a duration string.
func (dbase *Database) QueryWithTimeout(timeout string, query string, args ...any) ([]KeyValue, error) {
	ctx, cancel, err := dbase.parseTimeout(timeout)
	if err != nil {
		return nil, err
	}

	defer cancel()

	return dbase.query(ctx, query, args...)
}

// Exec executes a query without returning any rows.
func (dbase *Database) Exec(query string, args ...any) (sql.Result, error) {
	return dbase.exec(dbase.ctx(), query, args...)
}

// ExecWithTimeout executes a query (with a timeout) without returning any rows.
// The timeout can be specified as a duration string.
func (dbase *Database) ExecWithTimeout(timeout string, query string, args ...any) (sql.Result, error) {
	ctx, cancel, err := dbase.parseTimeout(timeout)
	if err != nil {
		return nil, err
	}

	defer cancel()

	return dbase.exec(ctx, query, args...)
}

// Close the database and prevents new queries from starting.
func (dbase *Database) Close() error {
	return dbase.db.Close()
}

func (dbase *Database) parseTimeout(timeout string) (context.Context, context.CancelFunc, error) {
	dur, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(dbase.ctx(), dur)

	return ctx, cancel, nil
}

func (dbase *Database) query(ctx context.Context, query string, args ...any) ([]KeyValue, error) {
	rows, err := dbase.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]any, len(cols))
	valuePtrs := make([]any, len(cols))
	result := make([]KeyValue, 0)

	for rows.Next() {
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		err = rows.Scan(valuePtrs...)
		if err != nil {
			return nil, err
		}

		data := make(KeyValue, len(cols))
		for i, colName := range cols {
			data[colName] = *valuePtrs[i].(*any) //nolint:forcetypeassert
		}

		result = append(result, data)
	}

	return result, nil
}

func (dbase *Database) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return dbase.db.ExecContext(ctx, query, args...)
}

var errUnsupportedDatabase = errors.New("unsupported database")

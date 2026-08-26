package sql

import (
	"database/sql"
	"time"

	"github.com/grafana/sobek"
)

// options represents connection related options for Open().
type options struct {
	ConnMaxIdleTime sobek.Value
	ConnMaxLifetime sobek.Value
	MaxIdleConns    sobek.Value
	MaxOpenConns    sobek.Value
}

func (o *options) apply(db *sql.DB) error {
	if o == nil {
		return nil
	}

	if o.ConnMaxIdleTime != nil {
		d, err := time.ParseDuration(o.ConnMaxIdleTime.String())
		if err != nil {
			return err
		}

		db.SetConnMaxIdleTime(d)
	}

	if o.ConnMaxLifetime != nil {
		d, err := time.ParseDuration(o.ConnMaxLifetime.String())
		if err != nil {
			return err
		}

		db.SetConnMaxLifetime(d)
	}

	if o.MaxIdleConns != nil {
		db.SetMaxIdleConns(int(o.MaxIdleConns.ToInteger()))
	}

	if o.MaxOpenConns != nil {
		db.SetMaxOpenConns(int(o.MaxOpenConns.ToInteger()))
	}

	return nil
}

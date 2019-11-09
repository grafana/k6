/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package timescaledb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
)

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

type dbThreshold struct {
	id        int
	threshold *stats.Threshold
}

type Collector struct {
	Pool   *pgx.ConnPool
	Config Config

	thresholds  map[string][]*dbThreshold
	buffer      []stats.Sample
	bufferLock  sync.Mutex
	wg          sync.WaitGroup
	semaphoreCh chan struct{}
}

// TimescaleDB database schema expected by this collector
const databaseSchema = `CREATE TABLE IF NOT EXISTS samples (
		ts timestamptz NOT NULL DEFAULT current_timestamp,
		metric varchar(128) NOT NULL,
		tags jsonb,
		value real
	);
	CREATE TABLE IF NOT EXISTS thresholds (
		id serial,
		ts timestamptz NOT NULL DEFAULT current_timestamp,
		metric varchar(128) NOT NULL,
		tags jsonb,
		threshold varchar(128) NOT NULL,
		abort_on_fail boolean DEFAULT FALSE,
		delay_abort_eval varchar(128),
		last_failed boolean DEFAULT FALSE
	);
	SELECT create_hypertable('samples', 'ts');
	CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples (ts DESC);
	CREATE INDEX IF NOT EXISTS idx_thresholds_ts ON thresholds (ts DESC);`

// New creates a new instance of the TimescaleDB collector by parsing the user supplied config
func New(conf Config, opts lib.Options) (*Collector, error) {
	connParams, err := pgx.ParseURI(conf.URL.String)
	if err != nil {
		fmt.Printf("TimescaleDB: Unable to parse config: %v", err)
		return nil, err
	}
	config := pgx.ConnPoolConfig{ConnConfig: connParams}
	pool, err := pgx.NewConnPool(config)
	if err != nil {
		fmt.Printf("TimescaleDB: Unable to create connection pool: %v", err)
		return nil, err
	}
	if conf.ConcurrentWrites.Int64 <= 0 {
		return nil, errors.New("TimescaleDB's ConcurrentWrites must be a positive number")
	}

	thresholds := make(map[string][]*dbThreshold)
	for name, t := range opts.Thresholds {
		for _, threshold := range t.Thresholds {
			thresholds[name] = append(thresholds[name], &dbThreshold{id: -1, threshold: threshold})
		}
	}

	return &Collector{
		Pool:        pool,
		Config:      conf,
		thresholds:  thresholds,
		semaphoreCh: make(chan struct{}, conf.ConcurrentWrites.Int64),
	}, nil
}

// Init creates a connection pool as well as database and schema if not already present
func (c *Collector) Init() error {
	conn, err := c.Pool.Acquire()
	if err != nil {
		logrus.WithError(err).Error("TimescaleDB: Couldn't acquire connection")
	}
	_, err = conn.Exec("CREATE DATABASE "+c.Config.db.String)
	if err != nil {
		logrus.WithError(err).Debug("TimescaleDB: Couldn't create database; most likely harmless")
	}
	_, err = conn.Exec(databaseSchema)
	if err != nil {
		logrus.WithError(err).Debug("TimescaleDB: Couldn't create database schema; most likely harmless")
	}

	// Insert thresholds
	for name, t := range c.thresholds {
		for _, threshold := range t {
			metric, _, tags := stats.ParseThresholdName(name)
			err = conn.QueryRow("INSERT INTO thresholds (metric, tags, threshold, abort_on_fail, delay_abort_eval, last_failed) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
				metric, tags, threshold.threshold.Source, threshold.threshold.AbortOnFail, threshold.threshold.AbortGracePeriod.String(), threshold.threshold.LastFailed).Scan(&threshold.id)
			if err != nil {
				logrus.WithError(err).Debug("TimescaleDB: Failed to insert threshold")
			}
		}
	}

	c.Pool.Release(conn)
	return nil
}

// Run is the main loop for the collector
func (c *Collector) Run(ctx context.Context) {
	logrus.Debug("TimescaleDB: Running!")
	ticker := time.NewTicker(time.Duration(c.Config.PushInterval.Duration))
	for {
		select {
		case <-ticker.C:
			c.wg.Add(1)
			go c.commit()
		case <-ctx.Done():
			c.wg.Add(1)
			go c.commit()
			c.wg.Wait()
			return
		}
	}
}

// Collect ingests samples and queues them for processing
func (c *Collector) Collect(scs []stats.SampleContainer) {
	c.bufferLock.Lock()
	defer c.bufferLock.Unlock()
	for _, sc := range scs {
		c.buffer = append(c.buffer, sc.GetSamples()...)
	}
}

// Link returns collector string representation for CLI display
func (c *Collector) Link() string {
	return c.Config.addr.String
}

// Commit processes queued samples and batch inserts data into TimescaleDB
func (c *Collector) commit() {
	defer c.wg.Done()
	c.bufferLock.Lock()
	samples := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()
	c.semaphoreCh <- struct{}{}
	defer func() {
		<-c.semaphoreCh
	}()

	logrus.Debug("TimescaleDB: Committing...")
	logrus.WithField("samples", len(samples)).Debug("TimescaleDB: Writing...")

	// Batch insert samples
	startTime := time.Now()
	batch := c.Pool.BeginBatch()
	for _, s := range samples {
		tags := s.Tags.CloneTags()
		batch.Queue("INSERT INTO samples (ts, metric, value, tags) VALUES ($1, $2, $3, $4)",
			[]interface{}{s.Time, s.Metric.Name, s.Value, tags},
			[]pgtype.OID{pgtype.TimestamptzOID, pgtype.VarcharOID, pgtype.Float4OID, pgtype.JSONBOID},
			nil)
	}

	// Batch threshold updates (update pass/fail status)
	for _, t := range c.thresholds {
		for _, threshold := range t {
			batch.Queue("UPDATE thresholds SET last_failed = $1 WHERE id = $2",
				[]interface{}{threshold.threshold.LastFailed, threshold.id},
				[]pgtype.OID{pgtype.BoolOID, pgtype.Int4OID},
				nil)
		}
	}

	if err := batch.Send(context.Background(), nil); err != nil {
		logrus.WithError(err).Error("TimescaleDB: Couldn't send batch of inserts")
	}
	if err := batch.Close(); err != nil {
		logrus.WithError(err).Error("TimescaleDB: Couldn't close batch and release connection")
	}

	t := time.Since(startTime)
	logrus.WithField("t", t).Debug("TimescaleDB: Batch written!")
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.SystemTagSet(0) // There are no required tags for this collector
}

// SetRunStatus does nothing in the TimescaleDB collector
func (c *Collector) SetRunStatus(status lib.RunStatus) {}

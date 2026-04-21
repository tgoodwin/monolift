package fixtures

import (
	"context"
	"database/sql"
	"sync"
)

type SQLRoot struct{ DB *sql.DB }

func (s *SQLRoot) Handle(ctx context.Context, n int) (int, error) { return n, nil }

var globalCounter int

func GlobalStore(n int) error {
	globalCounter = n
	return nil
}

var guardedMu sync.Mutex
var guardedCounter int

func SyncStore(n int) error {
	guardedMu.Lock()
	guardedCounter = n
	guardedMu.Unlock()
	return nil
}

type Worker struct {
	ch    chan int
	count int
}

func (w *Worker) Start(n int) error {
	go func() {
		for {
			<-w.ch
			w.count = n
		}
	}()
	return nil
}

type ConfigRoot struct{ Value int }

func (c *ConfigRoot) Handle(ctx context.Context, n int) (int, error) { return c.Value + n, nil }

type MutatingField struct{ Value int }

func (m *MutatingField) Handle(n int) error {
	m.Value = n
	return nil
}

func NoState(n int) error { return nil }

type DBApp struct{ DB *sql.DB }

func (d *DBApp) One() error   { return nil }
func (d *DBApp) Two() error   { return nil }
func (d *DBApp) Three() error { return nil }

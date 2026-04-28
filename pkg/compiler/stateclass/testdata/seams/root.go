package seams

import (
	"sync"
	"sync/atomic"
)

type Pipe struct {
	ch    chan int
	mu    sync.Mutex
	count atomic.Int64
}

func (p *Pipe) Write(v int) {
	p.ch <- v
}

func (p *Pipe) Read() int {
	return <-p.ch
}

func (p *Pipe) Both(v int) int {
	p.ch <- v
	return <-p.ch
}

func (p *Pipe) LockA() {
	p.mu.Lock()
	p.count.Add(1)
	p.mu.Unlock()
}

func (p *Pipe) LockB() {
	p.mu.Lock()
	p.count.Add(1)
	p.mu.Unlock()
}

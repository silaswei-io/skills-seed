package job

import (
	"errors"
	"sync"
)

var (
	ErrQueueFull = errors.New("dispatch queue is full")
	ErrClosed    = errors.New("dispatcher is closed")
)

type Task func()

type Dispatcher struct {
	mu     sync.RWMutex
	queue  chan Task
	closed bool
}

func NewDispatcher(capacity int) *Dispatcher {
	return &Dispatcher{queue: make(chan Task, capacity)}
}

// Submit reports overload and shutdown explicitly; it never claims that a rejected task was delivered.
func (d *Dispatcher) Submit(task Task) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return ErrClosed
	}
	select {
	case d.queue <- task:
		return nil
	default:
		return ErrQueueFull
	}
}

// Close transfers queue lifecycle ownership to the dispatcher and is idempotent.
func (d *Dispatcher) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	close(d.queue)
}

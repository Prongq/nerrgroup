// Package nerrgroup provides drop-in replacement for golang.org/x/sync/errgroup
// that's allow nested task scheduling
package nerrgroup

import (
	"context"
	"fmt"
	"sync"
)

type task func() error

type Group struct {
	sem     chan struct{}
	wg      sync.WaitGroup
	cancel  func()
	errOnce sync.Once
	err     error

	cond *sync.Cond
	todo []task
}

func New() *Group {
	return &Group{
		cond: sync.NewCond(new(sync.Mutex)),
	}
}

func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	return &Group{
		cancel: cancel,
		cond:   sync.NewCond(new(sync.Mutex)),
	}, ctx
}

func (s *Group) Go(fn task) {
	s.wg.Add(1)

	s.cond.L.Lock()
	s.todo = append(s.todo, fn)
	s.cond.Signal()
	s.cond.L.Unlock()
}

// Wait start worker pool for task executions and
// blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (s *Group) Wait() error {
	exitFlag := false

	go s.schedulingTasks(&exitFlag)

	s.wg.Wait()
	if s.cancel != nil {
		s.cancel()
	}

	s.cond.L.Lock()
	exitFlag = true
	s.cond.Signal()
	s.cond.L.Unlock()

	return s.err
}

// SetLimit limits the number of active goroutines in this group to at most n.
// A negative value indicates no limit.
//
// Any subsequent call to the Go method will block until it can add an active
// goroutine without exceeding the configured limit.
//
// The limit must not be modified while any goroutines in the group are active.
func (s *Group) SetLimit(n int) {
	if n < 0 {
		s.sem = nil
		return
	}
	if len(s.sem) != 0 {
		panic(fmt.Errorf(
			"nerrgroup: modify limit while %v goroutines in the group are still active",
			len(s.sem),
		))
	}

	s.sem = make(chan struct{}, n)
}

func (s *Group) done() {
	if s.sem != nil {
		<-s.sem
	}
	s.wg.Done()
	s.cond.Signal()
}

func (s *Group) schedulingTasks(exitFlag *bool) {
	for {
		if s.sem != nil {
			s.sem <- struct{}{}
		}

		s.cond.L.Lock()
		for !*exitFlag && len(s.todo) == 0 {
			s.cond.Wait()
		}

		if *exitFlag {
			s.cond.L.Unlock()
			return
		}

		go s.startTask(s.todo[0])
		s.todo = s.todo[1:]

		s.cond.L.Unlock()
	}
}

func (s *Group) startTask(fn task) {
	defer s.done()

	if err := fn(); err != nil {
		s.errOnce.Do(func() {
			s.err = err
			if s.cancel != nil {
				s.cancel()
			}
		})
	}
}

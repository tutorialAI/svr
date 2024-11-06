package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Task struct {
	ID      int
	timeout float64
	tries   int
}

type Result struct {
	taskID   int
	status   string
	taskTime float64
}

type Server struct {
	cfg       Config
	tasksCh   chan Task
	resultsCh chan Result
	quitCh    chan struct{}
	wg        sync.WaitGroup
}

type Config struct {
	maxTaskTries   int
	maxTaskTimeout float64
	workersCount   int
}

func main() {
	cfg := Config{
		maxTaskTries:   4,
		maxTaskTimeout: 2.0,
		workersCount:   10,
	}
	s := NewServer(cfg)
	go generateTasks(s)

	go func() {
		time.Sleep(5 * time.Second)
		s.Stop()
	}()

	s.Start()
}

func NewServer(cfg Config) *Server {
	return &Server{
		cfg:       cfg,
		tasksCh:   make(chan Task),
		resultsCh: make(chan Result),
		quitCh:    make(chan struct{}),
		wg:        sync.WaitGroup{},
	}
}

func (s *Server) Start() {
	fmt.Println("Server Starting...")

	ctx, cancel := context.WithCancel(context.Background())

	s.wg.Add(s.cfg.workersCount)
	for i := 0; i < s.cfg.workersCount; i++ {
		go worker(ctx, s)
	}

	go func() {
		s.wg.Wait()
		close(s.resultsCh)
	}()

	for v := range s.resultsCh {
		fmt.Println(v)
	}

	cancel()
}

func (s *Server) Stop() {
	s.quitCh <- struct{}{}
	close(s.quitCh)
}

func worker(ctx context.Context, s *Server) {
	defer s.wg.Done()

	for t := range s.tasksCh {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.maxTaskTimeout)*time.Second)

		select {
		case <-s.quitCh:
			fmt.Println("server stoped")
			// close(s.tasksCh)
			// close(s.resultsCh)
			cancel()
			return
		case s.resultsCh <- processTask(t):
		case <-ctx.Done():
			for trie := 1; trie <= s.cfg.maxTaskTries && trie <= t.tries; trie++ {
				t.timeout = -s.cfg.maxTaskTimeout

				if t.timeout <= s.cfg.maxTaskTimeout {
					s.resultsCh <- Result{taskID: t.ID, status: fmt.Sprintf("Sucssess after tries %d", trie)}
					break
				}
			}
			if t.timeout > s.cfg.maxTaskTimeout {
				s.resultsCh <- Result{taskID: t.ID, status: "Timeout", taskTime: t.timeout}
			}
		}

		cancel()
	}
}

func (s *Server) Submit(task Task) {
	s.tasksCh <- task
}

func processTask(task Task) Result {
	time.Sleep(time.Duration(task.timeout) * time.Second)

	return Result{taskID: task.ID, status: "Sucssess", taskTime: task.timeout}
}

func generateTasks(s *Server) {
	wg := sync.WaitGroup{}
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			rndTime := 0.5 + rand.Float64()*(6.0-0.5)
			rndTries := rand.Intn(s.cfg.maxTaskTries-1) + 1
			task := Task{ID: i, timeout: rndTime, tries: rndTries}
			s.Submit(task)

			wg.Done()
		}(i)
	}
	wg.Wait()
	close(s.tasksCh)
}

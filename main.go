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
		workersCount:   2,
	}
	s := NewServer(cfg)
	go generateTasks(s)

	go func() {
		time.Sleep(2 * time.Second)
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
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-s.quitCh:
			fmt.Println("stoped by quitCh")
			cancel()
			return
		}
	}()

	s.wg.Add(s.cfg.workersCount)
	for i := 0; i < s.cfg.workersCount; i++ {
		go worker(ctx, s, i)
	}

	go func() {
		s.wg.Wait()
		close(s.resultsCh)
	}()

	for v := range s.resultsCh {
		fmt.Println(v)
	}
}

func (s *Server) Stop() {
	s.quitCh <- struct{}{}
	close(s.quitCh)
}

func worker(ctx context.Context, s *Server, workerID int) {
	defer s.wg.Done()

	for t := range s.tasksCh {
		taskCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.maxTaskTimeout)*time.Second)

		select {
		case <-ctx.Done():
			fmt.Printf("server stoped, worker %d\n", workerID)
			return
		case s.resultsCh <- processTask(t):
		case <-taskCtx.Done():
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

func processTask(task Task) Result {
	time.Sleep(time.Duration(task.timeout) * time.Second)

	return Result{taskID: task.ID, status: "Sucssess", taskTime: task.timeout}
}

func (s *Server) Submit(id int) Task {
	rndTime := 0.5 + rand.Float64()*(6.0-0.5)
	rndTries := rand.Intn(s.cfg.maxTaskTries-1) + 1
	return Task{ID: id, timeout: rndTime, tries: rndTries}

}

func generateTasks(s *Server) {
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		select {
		case <-s.quitCh:
			fmt.Println("Server stopping, stopping task generation.")
			wg.Wait() // Дождаться завершения уже запущенных горутин
			close(s.tasksCh)
			return
		default:
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				select {
				case <-s.quitCh:
					return // завершить горутину, если сервер остановлен
				case s.tasksCh <- s.Submit(i):
					fmt.Printf("Task %d submitted\n", i)
				}
			}(i)
		}
	}

	wg.Wait()
	close(s.tasksCh)
}

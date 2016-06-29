package main

import (
	"log"
	"os/exec"
)

type WorkRequest struct {
	Info   *RequestInfo
	Script string
	Role   string
}

type Worker struct {
	ID          int
	Work        chan WorkRequest
	StatusChan  chan StatusMsg
	WorkerQueue chan chan WorkRequest
	QuitChan    chan bool
}

type StatusMsg struct {
	Request *WorkRequest `json:"request"`
	Worker  int          `json:"worker"`
	Status  TaskStatus   `json:"status"`
	Message string       `json:"message"`
}

func NewWorker(id int, workerQueue chan chan WorkRequest, statusChan chan StatusMsg) Worker {
	// Create, and return the worker.
	worker := Worker{
		ID:          id,
		Work:        make(chan WorkRequest),
		StatusChan:  statusChan,
		WorkerQueue: workerQueue,
		QuitChan:    make(chan bool),
	}

	return worker
}

func (w *Worker) Start() {
	go func() {
		for {
			// Add ourselves into the worker queue.
			w.WorkerQueue <- w.Work

			select {
			case work := <-w.Work:
				// Receive a work request.
				w.StatusChan <- StatusMsg{&work, w.ID, Running, ""}
				log.Printf("[debug] RUN: %s %s %s %s %s %s",
					work.Script, work.Info.Id, work.Info.Name,
					work.Info.Ip, work.Info.Mac, work.Role)
				err := exec.Command(work.Script, work.Info.Id, work.Info.Name,
					work.Info.Ip, work.Info.Mac, work.Role).Run()
				if err != nil {
					w.StatusChan <- StatusMsg{&work, w.ID, Failed, err.Error()}
				} else {
					w.StatusChan <- StatusMsg{&work, w.ID, Complete, ""}
				}
			case <-w.QuitChan:
				// We have been asked to stop.
				log.Printf("worker%d stopping\n", w.ID)
				return
			}
		}
	}()
}

func (w *Worker) Stop() {
	go func() {
		w.QuitChan <- true
	}()
}

type Dispatcher struct {
	Storage     Storage
	WorkQueue   chan WorkRequest
	WorkerQueue chan chan WorkRequest
	StatusChan  chan StatusMsg
	QuitChan    chan bool
	NumWorkers  int
}

func NewDispatcher(numWorkers int, storage Storage) *Dispatcher {
	d := Dispatcher{
		Storage:     storage,
		WorkQueue:   make(chan WorkRequest, 100),
		StatusChan:  make(chan StatusMsg, 100),
		NumWorkers:  numWorkers,
		WorkerQueue: make(chan chan WorkRequest, numWorkers),
		QuitChan:    make(chan bool),
	}

	return &d
}

func (d *Dispatcher) Dispatch(info *RequestInfo, role string, script string) error {
	d.WorkQueue <- WorkRequest{
		Info:   info,
		Script: script,
		Role:   role,
	}
	return nil
}

func (d *Dispatcher) Start() {
	// Now, create all of our workers.
	for i := 0; i < d.NumWorkers; i++ {
		log.Printf("Creating worker %d", i)
		worker := NewWorker(i, d.WorkerQueue, d.StatusChan)
		worker.Start()
	}

	go func() {
		for {
			select {
			case work := <-d.WorkQueue:
				log.Println("[debug] Received work requeust")
				go func() {
					d.StatusChan <- StatusMsg{&work, -1, Pending, ""}
					worker := <-d.WorkerQueue

					log.Println("[debug] Dispatching work request")
					worker <- work
				}()
			case update := <-d.StatusChan:
				err := d.Storage.Put(update.Request.Info.Id, update)
				if err != nil {
					log.Printf("[error] Unable to update storage with status for '%s' : %s",
						update.Request.Info.Id, err)
				} else {
					log.Printf("[debug] Storage updated for '%s'", update.Request.Info.Id)
				}
			case <-d.QuitChan:
				log.Println("[info] Stopping dispatcher")
				return
			}
		}
	}()
}

func (d *Dispatcher) Stop() {
	go func() {
		d.QuitChan <- true
	}()
}

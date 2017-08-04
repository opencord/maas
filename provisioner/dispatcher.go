// Copyright 2016 Open Networking Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"os/exec"
	"time"
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
	Request   *WorkRequest `json:"request"`
	Worker    int          `json:"worker"`
	Status    TaskStatus   `json:"status"`
	Message   string       `json:"message"`
	Timestamp int64        `json:"timestamp"`
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
				w.StatusChan <- StatusMsg{&work, w.ID, Running, "", time.Now().Unix()}
				log.Debugf("RUN: %s %s %s %s %s %s",
					work.Script, work.Info.Id, work.Info.Name,
					work.Info.Ip, work.Info.Mac, work.Role)
				err := exec.Command(work.Script, work.Info.Id, work.Info.Name,
					work.Info.Ip, work.Info.Mac, work.Role).Run()
				if err != nil {
					w.StatusChan <- StatusMsg{&work, w.ID, Failed, err.Error(),
						time.Now().Unix()}
				} else {
					w.StatusChan <- StatusMsg{&work, w.ID, Complete, "",
						time.Now().Unix()}
				}
			case <-w.QuitChan:
				// We have been asked to stop.
				log.Infof("worker%d stopping\n", w.ID)
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
		log.Infof("Creating worker %d", i)
		worker := NewWorker(i, d.WorkerQueue, d.StatusChan)
		worker.Start()
	}

	go func() {
		for {
			select {
			case work := <-d.WorkQueue:
				log.Debugf("Received work requeust")
				d.StatusChan <- StatusMsg{&work, -1, Pending, "", time.Now().Unix()}
				go func() {
					worker := <-d.WorkerQueue

					log.Debugf("Dispatching work request")
					worker <- work
				}()
			case update := <-d.StatusChan:
				err := d.Storage.Put(update.Request.Info.Id, update)
				if err != nil {
					log.Errorf("Unable to update storage with status for '%s' : %s",
						update.Request.Info.Id, err)
				} else {
					log.Debugf("Storage updated for '%s'", update.Request.Info.Id)
				}
			case <-d.QuitChan:
				log.Infof("Stopping dispatcher")
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

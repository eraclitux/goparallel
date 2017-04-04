// Copyright (c) 2015 Andrea Masi. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE.txt file.

// Package parallel simplifies use of parallel
// (as not concurrent) workers that run on their own core.
// Number of workers is adjusted at runtime in base of numbers of cores.
// This paradigm is particulary uselfull in presence of heavy,
// indipended tasks.
//
// Go's high level functionalities are used to implement parallelism
// with concurrency (channel, no locks).
package parallel

// NOTE Usefull for debugging on Linux: pidstat -tu  -C '<pid-name>'  1

import (
	"errors"
	"os"
	"os/signal"
	"runtime"

	"github.com/eraclitux/trace"
)

// Tasker interface models an heavy task that have to be
// executed from a worker.
type Tasker interface {
	Execute()
}

// ErrTasksNotCompleted says that not all tasks where completed.
var ErrTasksNotCompleted = errors.New("SIGINT received, not all tasks have been completed")

var workersNumber int = runtime.NumCPU()
var jobsQueue chan Tasker
var doneChan chan struct{}

func populateQueue(jobsQueue chan<- Tasker, jobs []Tasker, prematureEnd chan<- struct{}) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	for _, t := range jobs {
		select {
		case <-signalChan:
			// Abort jobs queue evaluation.
			// Taskers already sended will be finished
			// and an error will be returned.
			trace.Traceln("parallel: received SIGINT")
			prematureEnd <- struct{}{}
			close(jobsQueue)
			return
		default:
			jobsQueue <- t
		}
	}
	trace.Traceln("close jobsQueue")
	close(jobsQueue)
}

// parallelizeWorkers creates a goroutine for every worker
// which will call Execute() method.
func parallelizeWorkers(jobsQueue <-chan Tasker, doneChan chan<- struct{}) {
	for i := 0; i < workersNumber; i++ {
		go evaluateQueue(jobsQueue, doneChan)
	}
}

// evaluateQueue does jobs in sequence on its own goroutine
// on a single core.
func evaluateQueue(jobsQueue <-chan Tasker, doneChan chan<- struct{}) {
	for j := range jobsQueue {
		j.Execute()
	}
	doneChan <- struct{}{}
}

func init() {
	// Use all cores.
	// FIXME default in 1.5?
	runtime.GOMAXPROCS(WorkersNumber)
	// TODO Timeout a public accessible time out setting.
}

// RunBlocking starts the goroutines that will execute Taskers.
// It is intended to run blocking in the main goroutine.
// []T does not convert to []Tasker implicitly even is T implements
// Tasker. We need to iterate on []Tasker making an explicit cast.
// http://golang.org/doc/faq#convert_slice_of_interface
func RunBlocking(jobs []Tasker) (err error) {
	trace.Traceln("WorkersNumber:", workersNumber)
	prematureEnd := make(chan struct{})
	jobsQueue := make(chan Tasker, workersNumber)
	doneChan := make(chan struct{}, workersNumber)
	var totalDone int
	go populateQueue(jobsQueue, jobs, prematureEnd)
	go parallelizeWorkers(jobsQueue, doneChan)
	for {
		select {
		// TODO case timeout, returns error.
		case <-doneChan:
			totalDone++
		case <-prematureEnd:
			err = ErrTasksNotCompleted
		}
		if totalDone == workersNumber {
			// We can assume that jobsQueue is closed and
			// that no goroutine is operating on []Tasker.
			break
		}
	}
	return
}

// TODO has a non blocking version a sense (API semplification, performance etc.)? Es:
// When using RunBlocking one must wait that all tasks are done
// and put separate results togherther in the end. RunNonBlocking avoids that.
// func RunNonBlocking(jobs <-chan Tasker) (results chan<- Resulter) {
//code
//code
// Comunicate to callers that we are done.
// close(results)
//}

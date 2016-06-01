package main

type TaskStatus uint8

const (
	Pending TaskStatus = iota
	Running
	Complete
	Failed
)

func (s TaskStatus) String() string {
	switch s {
	case Pending:
		return "PENDING"
	case Running:
		return "RUNNING"
	case Complete:
		return "COMPLETE"
	case Failed:
		return "FAILED"
	}
	return "INVALID TASK STATUS"
}

type Task struct {
	nodeId string
	status TaskStatus
}

type TaskQueueEntry struct {
	previous *TaskQueueEntry
	next     *TaskQueueEntry
	task     *Task
}

type TaskQueue struct {
}

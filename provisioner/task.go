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

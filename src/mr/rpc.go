package mr

import "hash/fnv"

// RPC definitions

// Task types
type TaskType int

const (
	MapTask TaskType = iota
	ReduceTask
	WaitTask // no task available, wait
	ExitTask // all done, worker should exit
)

// GetTask RPC - Worker requests a task from Coordinator
type GetTaskArgs struct {
}

type GetTaskReply struct {
	TaskType TaskType
	TaskID   int
	File     string // input file for Map task
	NReduce  int    // number of reduce tasks
	NMap     int    // number of map tasks (for reduce to know intermediate files)
}

// TaskDone RPC - Worker reports task completion
type TaskDoneArgs struct {
	TaskType TaskType
	TaskID   int
}

type TaskDoneReply struct {
}

type KeyValue struct {
	Key   string
	Value string
}

func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

package mr

import (
	"sync"
	"time"
)

type TaskState int

const (
	Idle TaskState = iota
	InProgress
	Completed
)

type Phase int

const (
	MapPhase Phase = iota
	ReducePhase
	Done
)

type Task struct {
	Type      TaskType // Map or Reduce
	ID        int
	File      string // for Map task
	State     TaskState
	StartTime time.Time
}

type Coordinator struct {
	files       []string
	nReduce     int
	mapTasks    []Task
	reduceTasks []Task
	phase       Phase
	mu          sync.Mutex
}

func MakeCoordinator(files []string, nReduce int) *Coordinator {
	var c = &Coordinator{
		files:       files,
		nReduce:     nReduce,
		mapTasks:    make([]Task, len(files)),
		reduceTasks: make([]Task, nReduce),
		phase:       MapPhase,
		mu:          sync.Mutex{},
	}
	for i := range c.mapTasks {
		c.mapTasks[i].ID = i
		c.mapTasks[i].File = files[i]
		c.mapTasks[i].Type = MapTask
	}
	for i := range c.reduceTasks {
		c.reduceTasks[i].ID = i
		c.reduceTasks[i].Type = ReduceTask
	}
	go c.checkTimeouts()
	return c
}

func (c *Coordinator) GetTask(args *GetTaskArgs, reply *GetTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.phase {
	case MapPhase:
		for i, task := range c.mapTasks {
			if task.State == Idle {
				reply.TaskID = task.ID
				reply.TaskType = task.Type
				reply.File = task.File
				reply.NReduce = c.nReduce
				reply.NMap = len(c.files)
				c.mapTasks[i].State = InProgress
				c.mapTasks[i].StartTime = time.Now()

				return nil
			}
		}
		reply.TaskType = WaitTask
	case ReducePhase:
		for i, task := range c.reduceTasks {
			if task.State == Idle {
				reply.TaskID = task.ID
				reply.TaskType = task.Type
				reply.File = task.File
				reply.NReduce = c.nReduce
				reply.NMap = len(c.files)
				c.reduceTasks[i].State = InProgress
				c.reduceTasks[i].StartTime = time.Now()

				return nil
			}
		}
		reply.TaskType = WaitTask
	default:
		reply.TaskType = ExitTask
	}

	return nil
}

func (c *Coordinator) TaskDone(args *TaskDoneArgs, reply *TaskDoneReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch args.TaskType {
	case MapTask:
		c.mapTasks[args.TaskID].State = Completed
	case ReduceTask:
		c.reduceTasks[args.TaskID].State = Completed
	}

	if c.phase == MapPhase && allCompleted(c.mapTasks) {
		c.phase = ReducePhase
	}

	if c.phase == ReducePhase && allCompleted(c.reduceTasks) {
		c.phase = Done
	}
	return nil
}

func (c *Coordinator) Done() bool {
	return c.phase == Done
}

func (c *Coordinator) checkTimeouts() {
	for {
		time.Sleep(time.Second)

		c.mu.Lock()
		if c.phase == Done {
			c.mu.Unlock()
			return
		}

		var now = time.Now()
		for i, task := range c.mapTasks {
			if task.State == InProgress && now.After(task.StartTime.Add(time.Second*10)) {
				c.mapTasks[i].State = Idle
			}
		}

		for i, task := range c.reduceTasks {
			if task.State == InProgress && now.After(task.StartTime.Add(time.Second*10)) {
				c.reduceTasks[i].State = Idle
			}
		}

		c.mu.Unlock()
	}
}
func allCompleted(tasks []Task) bool {
	for _, task := range tasks {
		if task.State != Completed {
			return false
		}
	}
	return true
}

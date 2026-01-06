package mr

// TODO: Implement Coordinator

type Coordinator struct {
	// your fields here
}

func MakeCoordinator(files []string, nReduce int) *Coordinator {
	// TODO: implement
	return nil
}

func (c *Coordinator) GetTask(args *GetTaskArgs, reply *GetTaskReply) error {
	// TODO: implement
	return nil
}

func (c *Coordinator) TaskDone(args *TaskDoneArgs, reply *TaskDoneReply) error {
	// TODO: implement
	return nil
}

func (c *Coordinator) Done() bool {
	// TODO: implement
	return false
}

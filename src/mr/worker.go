package mr

import (
	"encoding/json"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
	"time"
)

// TODO: Implement Worker

type MapFunc func(string, string) []KeyValue
type ReduceFunc func(string, []string) string

func Worker(mapf MapFunc, reducef ReduceFunc) {
	for {
		reply := getTaskFromCoordinator()

		switch reply.TaskType {
		case MapTask:
			doMap(reply, mapf)
			reportTaskDone(reply)
		case ReduceTask:
			doReduce(reply, reducef)
			reportTaskDone(reply)
		case WaitTask:
			time.Sleep(time.Second)
		case ExitTask:
			return
		}

	}
}

func getTaskFromCoordinator() GetTaskReply {
	client, err := rpc.Dial("unix", "/tmp/mr-coordinator")
	if err != nil {
		return GetTaskReply{TaskType: ExitTask}
	}
	args := &GetTaskArgs{}
	reply := &GetTaskReply{}
	err = client.Call("Coordinator.GetTask", args, reply)
	if err != nil {
		return GetTaskReply{TaskType: ExitTask}
	}
	return *reply
}

func reportTaskDone(prev GetTaskReply) {
	client, _ := rpc.Dial("unix", "/tmp/mr-coordinator")
	args := &TaskDoneArgs{
		TaskType: prev.TaskType,
		TaskID:   prev.TaskID,
	}
	reply := &TaskDoneReply{}
	client.Call("Coordinator.TaskDone", args, reply)
}

func doMap(reply GetTaskReply, mapf MapFunc) {
	content, err := os.ReadFile(reply.File)
	if err != nil {
		log.Fatal(err)
	}
	text := string(content)
	kvs := mapf(reply.File, text)

	buckets := make([][]KeyValue, reply.NReduce)
	for _, kv := range kvs {
		reduceTaskID := ihash(kv.Key) % reply.NReduce
		buckets[reduceTaskID] = append(buckets[reduceTaskID], kv)
	}

	for reduceTaskID, bucket := range buckets {
		tmpFile, _ := os.CreateTemp(".", "mr-tmp-*")
		enc := json.NewEncoder(tmpFile)
		for _, kv := range bucket {
			enc.Encode(&kv)
		}
		tmpFile.Close()
		filename := fmt.Sprintf("mr-%d-%d", reply.TaskID, reduceTaskID)
		os.Rename(tmpFile.Name(), filename)
	}
}

func doReduce(reply GetTaskReply, reducef ReduceFunc) {
	pattern := fmt.Sprintf("mr-*-%d", reply.TaskID)
	files, _ := filepath.Glob(pattern)
	m := make(map[string][]string)
	for _, file := range files {
		f, _ := os.Open(file)
		dec := json.NewDecoder(f)
		for {
			var kv KeyValue
			err := dec.Decode(&kv)
			if err != nil {
				break
			}
			m[kv.Key] = append(m[kv.Key], kv.Value)
		}
		f.Close()
	}
	tmpFile, _ := os.CreateTemp(".", "mr-out-*")
	for k, v := range m {
		str := reducef(k, v)
		fmt.Fprintf(tmpFile, "%s %s\n", k, str)
	}
	tmpFile.Close()
	filename := fmt.Sprintf("mr-out-%d", reply.TaskID)
	os.Rename(tmpFile.Name(), filename)
}

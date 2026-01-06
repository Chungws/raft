package mr

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
)

// ============================================================
// Test helpers - Map/Reduce functions for Word Count
// ============================================================

func mapFunc(filename string, contents string) []KeyValue {
	ff := func(r rune) bool { return !unicode.IsLetter(r) }
	words := strings.FieldsFunc(contents, ff)

	kva := []KeyValue{}
	for _, w := range words {
		kv := KeyValue{Key: w, Value: "1"}
		kva = append(kva, kv)
	}
	return kva
}

func reduceFunc(key string, values []string) string {
	return strconv.Itoa(len(values))
}

// ============================================================
// Step 1: RPC Types Test
// ============================================================

func TestKeyValueType(t *testing.T) {
	// KeyValue struct should exist with Key and Value fields
	kv := KeyValue{Key: "hello", Value: "1"}

	if kv.Key != "hello" {
		t.Errorf("KeyValue.Key = %v, want 'hello'", kv.Key)
	}
	if kv.Value != "1" {
		t.Errorf("KeyValue.Value = %v, want '1'", kv.Value)
	}
}

func TestTaskTypes(t *testing.T) {
	// TaskType constants should be defined
	if MapTask == ReduceTask {
		t.Error("MapTask and ReduceTask should be different")
	}
	if WaitTask == ExitTask {
		t.Error("WaitTask and ExitTask should be different")
	}
}

func TestGetTaskRPCTypes(t *testing.T) {
	// GetTaskArgs and GetTaskReply should exist
	args := GetTaskArgs{}
	reply := GetTaskReply{
		TaskType: MapTask,
		TaskID:   0,
		File:     "input.txt",
		NReduce:  10,
		NMap:     5,
	}

	_ = args // args can be empty
	if reply.TaskType != MapTask {
		t.Errorf("reply.TaskType = %v, want MapTask", reply.TaskType)
	}
}

func TestTaskDoneRPCTypes(t *testing.T) {
	// TaskDoneArgs and TaskDoneReply should exist
	args := TaskDoneArgs{
		TaskType: MapTask,
		TaskID:   0,
	}
	reply := TaskDoneReply{}

	if args.TaskType != MapTask {
		t.Errorf("args.TaskType = %v, want MapTask", args.TaskType)
	}
	_ = reply
}

// ============================================================
// Step 2: Coordinator Tests
// ============================================================

func TestMakeCoordinator(t *testing.T) {
	files := []string{"file1.txt", "file2.txt"}
	nReduce := 3

	c := MakeCoordinator(files, nReduce)

	if c == nil {
		t.Fatal("MakeCoordinator returned nil")
	}

	// Should not be done immediately
	if c.Done() {
		t.Error("Coordinator should not be done immediately after creation")
	}
}

func TestCoordinatorAssignsMapTasks(t *testing.T) {
	// Create temp input files
	tmpDir := t.TempDir()
	files := createTestFiles(t, tmpDir, 3)

	c := MakeCoordinator(files, 2)
	defer cleanupCoordinator()

	// Request tasks - should get MapTask
	args := GetTaskArgs{}
	reply := GetTaskReply{}

	err := c.GetTask(&args, &reply)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if reply.TaskType != MapTask {
		t.Errorf("Expected MapTask, got %v", reply.TaskType)
	}
}

func TestCoordinatorAssignsAllMapTasks(t *testing.T) {
	tmpDir := t.TempDir()
	files := createTestFiles(t, tmpDir, 3)

	c := MakeCoordinator(files, 2)
	defer cleanupCoordinator()

	// Request all map tasks
	assignedTasks := make(map[int]bool)
	for i := 0; i < len(files); i++ {
		args := GetTaskArgs{}
		reply := GetTaskReply{}

		c.GetTask(&args, &reply)

		if reply.TaskType != MapTask {
			t.Errorf("Expected MapTask, got %v", reply.TaskType)
		}
		assignedTasks[reply.TaskID] = true
	}

	// All tasks should be assigned
	if len(assignedTasks) != len(files) {
		t.Errorf("Expected %d unique tasks, got %d", len(files), len(assignedTasks))
	}
}

func TestCoordinatorWaitsWhenNoTask(t *testing.T) {
	tmpDir := t.TempDir()
	files := createTestFiles(t, tmpDir, 2)

	c := MakeCoordinator(files, 2)
	defer cleanupCoordinator()

	// Assign all map tasks
	for i := 0; i < len(files); i++ {
		args := GetTaskArgs{}
		reply := GetTaskReply{}
		c.GetTask(&args, &reply)
	}

	// Next request should get WaitTask
	args := GetTaskArgs{}
	reply := GetTaskReply{}
	c.GetTask(&args, &reply)

	if reply.TaskType != WaitTask {
		t.Errorf("Expected WaitTask when no tasks available, got %v", reply.TaskType)
	}
}

func TestCoordinatorTransitionsToReducePhase(t *testing.T) {
	tmpDir := t.TempDir()
	files := createTestFiles(t, tmpDir, 2)

	c := MakeCoordinator(files, 2)
	defer cleanupCoordinator()

	// Complete all map tasks
	for i := 0; i < len(files); i++ {
		args := GetTaskArgs{}
		reply := GetTaskReply{}
		c.GetTask(&args, &reply)

		doneArgs := TaskDoneArgs{TaskType: MapTask, TaskID: reply.TaskID}
		doneReply := TaskDoneReply{}
		c.TaskDone(&doneArgs, &doneReply)
	}

	// Next request should get ReduceTask
	args := GetTaskArgs{}
	reply := GetTaskReply{}
	c.GetTask(&args, &reply)

	if reply.TaskType != ReduceTask {
		t.Errorf("Expected ReduceTask after map phase, got %v", reply.TaskType)
	}
}

func TestCoordinatorDone(t *testing.T) {
	tmpDir := t.TempDir()
	files := createTestFiles(t, tmpDir, 2)
	nReduce := 2

	c := MakeCoordinator(files, nReduce)
	defer cleanupCoordinator()

	// Complete all map tasks
	for i := 0; i < len(files); i++ {
		args := GetTaskArgs{}
		reply := GetTaskReply{}
		c.GetTask(&args, &reply)

		doneArgs := TaskDoneArgs{TaskType: MapTask, TaskID: reply.TaskID}
		doneReply := TaskDoneReply{}
		c.TaskDone(&doneArgs, &doneReply)
	}

	// Complete all reduce tasks
	for i := 0; i < nReduce; i++ {
		args := GetTaskArgs{}
		reply := GetTaskReply{}
		c.GetTask(&args, &reply)

		doneArgs := TaskDoneArgs{TaskType: ReduceTask, TaskID: reply.TaskID}
		doneReply := TaskDoneReply{}
		c.TaskDone(&doneArgs, &doneReply)
	}

	// Coordinator should be done
	if !c.Done() {
		t.Error("Coordinator should be done after all tasks completed")
	}
}

func TestCoordinatorTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	tmpDir := t.TempDir()
	files := createTestFiles(t, tmpDir, 1)

	c := MakeCoordinator(files, 1)
	defer cleanupCoordinator()

	// Get a task but don't complete it
	args := GetTaskArgs{}
	reply := GetTaskReply{}
	c.GetTask(&args, &reply)

	firstTaskID := reply.TaskID

	// Wait for timeout (>10 seconds)
	time.Sleep(11 * time.Second)

	// Task should be reassigned
	args2 := GetTaskArgs{}
	reply2 := GetTaskReply{}
	c.GetTask(&args2, &reply2)

	if reply2.TaskType == WaitTask {
		t.Error("Task should be reassigned after timeout")
	}
	if reply2.TaskID != firstTaskID {
		t.Errorf("Expected task %d to be reassigned, got %d", firstTaskID, reply2.TaskID)
	}
}

// ============================================================
// Step 3: Worker Tests (Integration)
// ============================================================

func TestWorkerDoesMapTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	inputFile := filepath.Join(tmpDir, "input.txt")
	os.WriteFile(inputFile, []byte("hello world hello"), 0644)

	files := []string{inputFile}
	nReduce := 2

	c := MakeCoordinator(files, nReduce)
	defer cleanupCoordinator()

	// Simulate worker
	args := GetTaskArgs{}
	reply := GetTaskReply{}
	c.GetTask(&args, &reply)

	if reply.TaskType != MapTask {
		t.Fatalf("Expected MapTask, got %v", reply.TaskType)
	}

	// Worker reads file and applies map function
	content, _ := os.ReadFile(reply.File)
	kva := mapFunc(reply.File, string(content))

	// Should produce key-value pairs
	if len(kva) != 3 {
		t.Errorf("Expected 3 key-value pairs, got %d", len(kva))
	}
}

func TestIntermediateFileNaming(t *testing.T) {
	// Intermediate files should follow mr-X-Y naming
	mapTaskID := 2
	reduceTaskID := 5

	filename := fmt.Sprintf("mr-%d-%d", mapTaskID, reduceTaskID)

	if filename != "mr-2-5" {
		t.Errorf("Expected 'mr-2-5', got '%s'", filename)
	}
}

func TestIhash(t *testing.T) {
	// ihash function should exist and be deterministic
	key := "hello"

	h1 := ihash(key)
	h2 := ihash(key)

	if h1 != h2 {
		t.Error("ihash should be deterministic")
	}

	// Different keys should (usually) have different hashes
	h3 := ihash("world")
	if h1 == h3 {
		t.Log("Warning: 'hello' and 'world' have same hash (unlikely but possible)")
	}
}

// ============================================================
// Step 4: Full Integration Test
// ============================================================

func TestWordCount(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input files
	os.WriteFile(filepath.Join(tmpDir, "input1.txt"), []byte("hello world hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "input2.txt"), []byte("world go go go"), 0644)

	files := []string{
		filepath.Join(tmpDir, "input1.txt"),
		filepath.Join(tmpDir, "input2.txt"),
	}
	nReduce := 2

	c := MakeCoordinator(files, nReduce)
	defer cleanupCoordinator()

	// Change to tmpDir for intermediate files
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Run workers
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Worker(mapFunc, reduceFunc)
		}()
	}

	// Wait for completion
	for !c.Done() {
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()

	// Check output files
	expected := map[string]string{
		"hello": "2",
		"world": "2",
		"go":    "3",
	}

	result := readOutputFiles(t, tmpDir, nReduce)

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("Word '%s': expected count %s, got %s", k, v, result[k])
		}
	}
}

func TestParallelWorkers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple input files
	for i := 0; i < 5; i++ {
		content := strings.Repeat("word ", 100)
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("input%d.txt", i)), []byte(content), 0644)
	}

	files := []string{}
	for i := 0; i < 5; i++ {
		files = append(files, filepath.Join(tmpDir, fmt.Sprintf("input%d.txt", i)))
	}
	nReduce := 3

	c := MakeCoordinator(files, nReduce)
	defer cleanupCoordinator()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Run multiple workers in parallel
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Worker(mapFunc, reduceFunc)
		}()
	}

	// Wait for completion with timeout
	done := make(chan bool)
	go func() {
		for !c.Done() {
			time.Sleep(100 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
		// success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout: parallel workers didn't complete in time")
	}

	wg.Wait()

	// Verify result
	result := readOutputFiles(t, tmpDir, nReduce)
	if result["word"] != "500" {
		t.Errorf("Expected 'word' count 500, got %s", result["word"])
	}
}

func TestWorkerCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping crash test in short mode")
	}

	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "input1.txt"), []byte("crash test"), 0644)

	files := []string{filepath.Join(tmpDir, "input1.txt")}
	nReduce := 1

	c := MakeCoordinator(files, nReduce)
	defer cleanupCoordinator()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// First worker gets task then "crashes" (doesn't complete)
	args := GetTaskArgs{}
	reply := GetTaskReply{}
	c.GetTask(&args, &reply)
	// Don't call TaskDone - simulating crash

	// Wait for timeout
	time.Sleep(11 * time.Second)

	// Second worker should be able to complete
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Worker(mapFunc, reduceFunc)
	}()

	// Wait for completion with timeout
	done := make(chan bool)
	go func() {
		for !c.Done() {
			time.Sleep(100 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
		// success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout: work should complete after crash recovery")
	}

	wg.Wait()
}

// ============================================================
// Helper functions
// ============================================================

func createTestFiles(t *testing.T, dir string, n int) []string {
	files := make([]string, n)
	for i := 0; i < n; i++ {
		path := filepath.Join(dir, fmt.Sprintf("input%d.txt", i))
		err := os.WriteFile(path, []byte(fmt.Sprintf("content %d", i)), 0644)
		if err != nil {
			t.Fatal(err)
		}
		files[i] = path
	}
	return files
}

func cleanupCoordinator() {
	os.Remove("/tmp/mr-coordinator")
}

func readOutputFiles(t *testing.T, dir string, nReduce int) map[string]string {
	result := make(map[string]string)

	for i := 0; i < nReduce; i++ {
		filename := filepath.Join(dir, fmt.Sprintf("mr-out-%d", i))
		content, err := os.ReadFile(filename)
		if err != nil {
			continue // file might not exist if no keys mapped to this reduce task
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Split(line, " ")
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}

	return result
}

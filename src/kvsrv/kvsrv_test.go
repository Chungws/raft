package kvsrv

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// Test helpers
// ============================================================

type testServer struct {
	kv       *KVServer
	listener net.Listener
	server   *rpc.Server
}

func startTestServer(t *testing.T) *testServer {
	kv := MakeKVServer()

	server := rpc.NewServer()
	server.Register(kv)

	sockname := fmt.Sprintf("/tmp/kvsrv-%d-%d", os.Getpid(), rand.Int())
	os.Remove(sockname)
	listener, err := net.Listen("unix", sockname)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.ServeConn(conn)
		}
	}()

	return &testServer{
		kv:       kv,
		listener: listener,
		server:   server,
	}
}

func (ts *testServer) stop() {
	ts.listener.Close()
}

func (ts *testServer) sockname() string {
	return ts.listener.Addr().String()
}

func makeClerk(sockname string) *Clerk {
	return MakeClerk(sockname)
}

// ============================================================
// Step 2.1: Basic KV Server Tests
// ============================================================

func TestRPCTypes(t *testing.T) {
	// GetArgs and GetReply should exist
	getArgs := GetArgs{Key: "x"}
	getReply := GetReply{Value: "hello"}

	if getArgs.Key != "x" {
		t.Errorf("GetArgs.Key = %v, want 'x'", getArgs.Key)
	}
	if getReply.Value != "hello" {
		t.Errorf("GetReply.Value = %v, want 'hello'", getReply.Value)
	}

	// PutAppendArgs and PutAppendReply should exist
	putArgs := PutAppendArgs{
		Key:   "y",
		Value: "world",
	}
	putReply := PutAppendReply{}

	if putArgs.Key != "y" {
		t.Errorf("PutAppendArgs.Key = %v, want 'y'", putArgs.Key)
	}
	_ = putReply
}

func TestMakeKVServer(t *testing.T) {
	kv := MakeKVServer()
	if kv == nil {
		t.Fatal("MakeKVServer returned nil")
	}
}

func TestBasicPutGet(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	ck.Put("x", "hello")
	v := ck.Get("x")

	if v != "hello" {
		t.Errorf("Get(x) = %v, want 'hello'", v)
	}
}

func TestGetNonExistent(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	v := ck.Get("nonexistent")
	if v != "" {
		t.Errorf("Get(nonexistent) = %v, want ''", v)
	}
}

func TestPutOverwrite(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	ck.Put("x", "first")
	ck.Put("x", "second")
	v := ck.Get("x")

	if v != "second" {
		t.Errorf("Get(x) = %v, want 'second'", v)
	}
}

func TestAppend(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	ck.Put("x", "hello")
	ck.Append("x", " world")
	v := ck.Get("x")

	if v != "hello world" {
		t.Errorf("Get(x) = %v, want 'hello world'", v)
	}
}

func TestAppendToEmpty(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	ck.Append("empty", "value")
	v := ck.Get("empty")

	if v != "value" {
		t.Errorf("Get(empty) = %v, want 'value'", v)
	}
}

func TestMultipleKeys(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	ck.Put("a", "1")
	ck.Put("b", "2")
	ck.Put("c", "3")

	if ck.Get("a") != "1" {
		t.Errorf("Get(a) failed")
	}
	if ck.Get("b") != "2" {
		t.Errorf("Get(b) failed")
	}
	if ck.Get("c") != "3" {
		t.Errorf("Get(c) failed")
	}
}

// ============================================================
// Step 2.2: Clerk Tests
// ============================================================

func TestClerkCreation(t *testing.T) {
	ck := MakeClerk("/tmp/dummy-socket")
	if ck == nil {
		t.Fatal("MakeClerk returned nil")
	}
}

// ============================================================
// Step 2.3: Concurrency Tests
// ============================================================

func TestConcurrentPut(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	var wg sync.WaitGroup
	n := 10

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ck := makeClerk(ts.sockname())
			ck.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
		}(i)
	}

	wg.Wait()

	// Verify all puts succeeded
	ck := makeClerk(ts.sockname())
	for i := 0; i < n; i++ {
		v := ck.Get(fmt.Sprintf("key%d", i))
		expected := fmt.Sprintf("value%d", i)
		if v != expected {
			t.Errorf("Get(key%d) = %v, want %v", i, v, expected)
		}
	}
}

func TestConcurrentAppend(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	var wg sync.WaitGroup
	n := 10

	ck := makeClerk(ts.sockname())
	ck.Put("counter", "")

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ck := makeClerk(ts.sockname())
			ck.Append("counter", "x")
		}()
	}

	wg.Wait()

	// Verify all appends happened
	v := ck.Get("counter")
	if len(v) != n {
		t.Errorf("len(counter) = %d, want %d", len(v), n)
	}
}

func TestConcurrentMixed(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	var wg sync.WaitGroup

	// Multiple clerks doing mixed operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ck := makeClerk(ts.sockname())

			key := fmt.Sprintf("key%d", id)
			ck.Put(key, "initial")

			for j := 0; j < 10; j++ {
				ck.Append(key, ".")
				ck.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	ck := makeClerk(ts.sockname())
	for i := 0; i < 5; i++ {
		v := ck.Get(fmt.Sprintf("key%d", i))
		expected := "initial.........."
		if v != expected {
			t.Errorf("key%d = %v, want %v", i, v, expected)
		}
	}
}

// ============================================================
// Step 2.4: Duplicate Detection Tests
// ============================================================

func TestDuplicateDetectionRPCTypes(t *testing.T) {
	// PutAppendArgs should have ClientId and SeqNum for duplicate detection
	args := PutAppendArgs{
		Key:      "x",
		Value:    "y",
		ClientId: 12345,
		SeqNum:   1,
	}

	if args.ClientId != 12345 {
		t.Errorf("ClientId = %v, want 12345", args.ClientId)
	}
	if args.SeqNum != 1 {
		t.Errorf("SeqNum = %v, want 1", args.SeqNum)
	}
}

func TestDuplicatePut(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	// Simulate duplicate Put by calling RPC directly
	client, _ := rpc.Dial("unix", ts.sockname())
	defer client.Close()

	args := &PutAppendArgs{
		Key:      "x",
		Value:    "first",
		Op:       "Put",
		ClientId: 1,
		SeqNum:   1,
	}

	// First call
	reply1 := &PutAppendReply{}
	client.Call("KVServer.PutAppend", args, reply1)

	// Duplicate call with same ClientId and SeqNum
	reply2 := &PutAppendReply{}
	client.Call("KVServer.PutAppend", args, reply2)

	// Value should be "first", not applied twice
	ck := makeClerk(ts.sockname())
	v := ck.Get("x")
	if v != "first" {
		t.Errorf("Get(x) = %v, want 'first'", v)
	}
}

func TestDuplicateAppend(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())
	ck.Put("x", "start")

	// Simulate duplicate Append
	client, _ := rpc.Dial("unix", ts.sockname())
	defer client.Close()

	args := &PutAppendArgs{
		Key:      "x",
		Value:    "-append",
		Op:       "Append",
		ClientId: 2,
		SeqNum:   1,
	}

	// First call
	reply1 := &PutAppendReply{}
	client.Call("KVServer.PutAppend", args, reply1)

	// Duplicate call
	reply2 := &PutAppendReply{}
	client.Call("KVServer.PutAppend", args, reply2)

	// Should only append once
	v := ck.Get("x")
	if v != "start-append" {
		t.Errorf("Get(x) = %v, want 'start-append' (duplicate append should be ignored)", v)
	}
}

func TestSequentialSeqNums(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	client, _ := rpc.Dial("unix", ts.sockname())
	defer client.Close()

	clientId := int64(100)

	// SeqNum 1
	args1 := &PutAppendArgs{Key: "x", Value: "a", Op: "Put", ClientId: clientId, SeqNum: 1}
	client.Call("KVServer.PutAppend", args1, &PutAppendReply{})

	// SeqNum 2
	args2 := &PutAppendArgs{Key: "x", Value: "b", Op: "Put", ClientId: clientId, SeqNum: 2}
	client.Call("KVServer.PutAppend", args2, &PutAppendReply{})

	// Retry SeqNum 1 (should be ignored)
	client.Call("KVServer.PutAppend", args1, &PutAppendReply{})

	ck := makeClerk(ts.sockname())
	v := ck.Get("x")
	if v != "b" {
		t.Errorf("Get(x) = %v, want 'b'", v)
	}
}

func TestMultipleClients(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	var wg sync.WaitGroup
	var counter int32 = 0
	n := 5

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()
			ck := makeClerk(ts.sockname())

			for j := 0; j < 10; j++ {
				ck.Append("shared", "x")
				atomic.AddInt32(&counter, 1)
			}
		}(i)
	}

	wg.Wait()

	ck := makeClerk(ts.sockname())
	v := ck.Get("shared")

	expected := int(atomic.LoadInt32(&counter))
	if len(v) != expected {
		t.Errorf("len(shared) = %d, want %d", len(v), expected)
	}
}

// ============================================================
// Integration Tests
// ============================================================

func TestReliability(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	// Many operations
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i%10)
		val := fmt.Sprintf("val%d", i)
		ck.Put(key, val)

		if ck.Get(key) != val {
			t.Errorf("Put/Get mismatch at iteration %d", i)
		}
	}
}

func TestLongValues(t *testing.T) {
	ts := startTestServer(t)
	defer ts.stop()

	ck := makeClerk(ts.sockname())

	longVal := string(make([]byte, 10000))
	for i := range longVal {
		longVal = longVal[:i] + "x" + longVal[i+1:]
	}

	ck.Put("long", longVal)
	v := ck.Get("long")

	if v != longVal {
		t.Errorf("Long value mismatch: got len=%d, want len=%d", len(v), len(longVal))
	}
}

func TestStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	ts := startTestServer(t)
	defer ts.stop()

	var wg sync.WaitGroup
	duration := 2 * time.Second
	done := make(chan bool)

	// Start workers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ck := makeClerk(ts.sockname())
			key := fmt.Sprintf("stress%d", id)

			for {
				select {
				case <-done:
					return
				default:
					ck.Put(key, "value")
					ck.Append(key, "x")
					ck.Get(key)
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(done)
	wg.Wait()
}

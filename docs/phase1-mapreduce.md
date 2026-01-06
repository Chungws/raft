# Phase 1: MapReduce (MIT Lab 1)

> **예상 시간**: ~15시간
> **난이도**: Moderate
> **논문**: [MapReduce: Simplified Data Processing on Large Clusters](https://pdos.csail.mit.edu/6.824/papers/mapreduce.pdf)

## 목표

분산 MapReduce 시스템 구현 - Coordinator가 작업을 분배하고, 여러 Worker가 병렬로 처리

## 개념 이해

```
Input Files    Map Phase         Shuffle        Reduce Phase    Output
┌──────┐      ┌─────────┐                      ┌──────────┐
│ file1│ ───► │ Map Task│ ──┐              ┌─► │Reduce(A) │ ──► output-A
└──────┘      └─────────┘   │   ┌──────┐   │   └──────────┘
┌──────┐      ┌─────────┐   ├─► │Bucket│ ──┤   ┌──────────┐
│ file2│ ───► │ Map Task│ ──┤   └──────┘   └─► │Reduce(B) │ ──► output-B
└──────┘      └─────────┘   │                  └──────────┘
┌──────┐      ┌─────────┐   │
│ file3│ ───► │ Map Task│ ──┘
└──────┘      └─────────┘
```

---

## Step 1.1: MapReduce 개념 이해

### 학습 내용
- [ ] MapReduce 논문 읽기
- [ ] Map, Reduce 함수 개념 이해
- [ ] Word Count 예제 분석

### Word Count 예제
```go
// Map: (filename, contents) → list of (word, "1")
func Map(filename string, contents string) []KeyValue {
    words := strings.Fields(contents)
    kva := []KeyValue{}
    for _, w := range words {
        kva = append(kva, KeyValue{w, "1"})
    }
    return kva
}

// Reduce: (word, list of counts) → total count
func Reduce(key string, values []string) string {
    return strconv.Itoa(len(values))
}
```

---

## Step 1.2: 순차 실행 버전

### 체크리스트
- [ ] Map 함수 구현
- [ ] Reduce 함수 구현
- [ ] 단일 프로세스 MapReduce 테스트

### 순차 실행 흐름
```go
func Sequential(files []string, mapf MapFunc, reducef ReduceFunc) {
    // 1. Map phase
    intermediate := []KeyValue{}
    for _, file := range files {
        content := readFile(file)
        kva := mapf(file, content)
        intermediate = append(intermediate, kva...)
    }

    // 2. Sort by key
    sort.Slice(intermediate, func(i, j int) bool {
        return intermediate[i].Key < intermediate[j].Key
    })

    // 3. Reduce phase
    for each unique key {
        values := getAllValuesForKey(key)
        output := reducef(key, values)
        writeOutput(key, output)
    }
}
```

---

## Step 1.3: Coordinator (Master)

### 체크리스트
- [ ] Coordinator 구조체 정의
- [ ] Task 상태 관리 (idle, in-progress, completed)
- [ ] Worker 등록 및 Task 할당
- [ ] Task timeout 및 재할당 (10초)

### 구조체 설계
```go
type TaskState int

const (
    Idle TaskState = iota
    InProgress
    Completed
)

type Task struct {
    Type      TaskType  // Map or Reduce
    ID        int
    File      string    // for Map task
    State     TaskState
    StartTime time.Time
}

type Coordinator struct {
    mu          sync.Mutex
    mapTasks    []Task
    reduceTasks []Task
    nReduce     int
    phase       Phase  // Map, Reduce, Done
}
```

### RPC 인터페이스
```go
// Worker가 Task 요청
func (c *Coordinator) GetTask(args *GetTaskArgs, reply *GetTaskReply) error

// Worker가 완료 보고
func (c *Coordinator) TaskDone(args *TaskDoneArgs, reply *TaskDoneReply) error
```

---

## Step 1.4: Worker

### 체크리스트
- [ ] Worker 구조체 정의
- [ ] Coordinator에 Task 요청
- [ ] Map Task 실행 및 중간 파일 생성
- [ ] Reduce Task 실행 및 결과 출력

### Worker 루프
```go
func Worker(mapf MapFunc, reducef ReduceFunc) {
    for {
        task := getTaskFromCoordinator()

        switch task.Type {
        case MapTask:
            doMap(task, mapf)
        case ReduceTask:
            doReduce(task, reducef)
        case WaitTask:
            time.Sleep(time.Second)
        case ExitTask:
            return
        }

        reportTaskDone(task)
    }
}
```

### 중간 파일 명명 규칙
```
mr-X-Y
- X: Map task 번호
- Y: Reduce task 번호 (hash(key) % nReduce)
```

---

## Step 1.5: 분산 실행

### 체크리스트
- [ ] RPC 통신 구현
- [ ] 병렬 Worker 실행
- [ ] Worker 장애 처리 (timeout)
- [ ] 전체 테스트 통과

### 장애 처리
```go
// Coordinator: 10초 timeout 체크
func (c *Coordinator) checkTimeouts() {
    for {
        time.Sleep(time.Second)
        c.mu.Lock()
        for i, task := range c.mapTasks {
            if task.State == InProgress &&
               time.Since(task.StartTime) > 10*time.Second {
                c.mapTasks[i].State = Idle  // 재할당 가능
            }
        }
        c.mu.Unlock()
    }
}
```

---

## 테스트

```bash
cd src/mr

# 순차 실행 테스트
go run mrsequential.go wc.so pg*.txt

# 분산 실행 테스트
go test -race

# 개별 테스트
go test -race -run TestBasic
go test -race -run TestCrash
```

---

## 주의사항

1. **임시 파일 사용**: 중간 파일 작성 시 atomic write (임시 파일 → rename)
2. **JSON 인코딩**: 중간 파일은 JSON으로 인코딩
3. **해시 함수**: `ihash(key) % nReduce`로 파티션 결정

---

## 체크리스트

- [ ] MapReduce 논문 읽음
- [ ] 순차 실행 버전 동작
- [ ] Coordinator 구현 완료
- [ ] Worker 구현 완료
- [ ] 모든 테스트 통과 (`go test -race`)

## 다음 단계

MapReduce를 마쳤다면 → [Phase 2: KV Server](phase2-kv-server.md)

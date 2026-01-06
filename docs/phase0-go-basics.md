# Phase 0: Go 기초 (선택)

> **예상 시간**: ~10시간
> **난이도**: Easy
> **선수 조건**: 프로그래밍 기초

## 목표

분산 시스템 구현에 필요한 Go 언어 핵심 기능 습득

## Step 0.1: Go 언어 기본

### 학습 내용
- [ ] [Go Tour](https://go.dev/tour/) 완료
- [ ] [Effective Go](https://go.dev/doc/effective_go) 읽기
- [ ] 기본 문법, 타입, 함수, 메서드

### 핵심 개념
```go
// 구조체와 메서드
type Server struct {
    mu   sync.Mutex
    data map[string]string
}

func (s *Server) Get(key string) string {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.data[key]
}
```

---

## Step 0.2: 동시성 프로그래밍

### 학습 내용
- [ ] goroutine 이해 및 실습
- [ ] channel 통신 패턴
- [ ] sync.Mutex, sync.WaitGroup
- [ ] `go test -race` 사용법

### goroutine 예제
```go
func main() {
    go func() {
        fmt.Println("Hello from goroutine")
    }()
    time.Sleep(time.Second)
}
```

### channel 패턴
```go
// 결과 수집
results := make(chan int, 10)

for i := 0; i < 10; i++ {
    go func(n int) {
        results <- n * n
    }(i)
}

for i := 0; i < 10; i++ {
    fmt.Println(<-results)
}
```

### Mutex 사용
```go
type Counter struct {
    mu    sync.Mutex
    count int
}

func (c *Counter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}
```

### 경쟁 상태 감지
```bash
go test -race ./...
```

---

## Step 0.3: 네트워크 & RPC

### 학습 내용
- [ ] net/rpc 패키지 이해
- [ ] 간단한 RPC 서버/클라이언트 작성
- [ ] encoding/gob 직렬화

### RPC 서버 예제
```go
type Args struct {
    A, B int
}

type Reply struct {
    Result int
}

type Calculator int

func (c *Calculator) Add(args *Args, reply *Reply) error {
    reply.Result = args.A + args.B
    return nil
}

// 서버 시작
func startServer() {
    calc := new(Calculator)
    rpc.Register(calc)
    l, _ := net.Listen("tcp", ":1234")
    go rpc.Accept(l)
}
```

### RPC 클라이언트 예제
```go
func call() {
    client, _ := rpc.Dial("tcp", "localhost:1234")
    args := &Args{A: 3, B: 5}
    reply := &Reply{}
    client.Call("Calculator.Add", args, reply)
    fmt.Println(reply.Result) // 8
}
```

---

## 체크리스트

- [ ] Go Tour 완료
- [ ] goroutine으로 병렬 작업 실행 가능
- [ ] channel로 goroutine 간 통신 가능
- [ ] Mutex로 공유 자원 보호 가능
- [ ] 간단한 RPC 서버/클라이언트 작성 가능
- [ ] `go test -race` 사용 가능

## 다음 단계

Go 기초를 마쳤다면 → [Phase 1: MapReduce](phase1-mapreduce.md)

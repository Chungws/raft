# Phase 2: Key/Value Server (MIT Lab 2)

> **예상 시간**: ~10시간
> **난이도**: Moderate
> **핵심**: 클라이언트-서버 RPC, 중복 요청 처리

## 목표

단일 서버 Key/Value 저장소 구현 - 동시성 처리와 중복 요청 감지

## 개념 이해

```
┌──────────┐         RPC          ┌──────────┐
│  Client  │ ──────────────────►  │ KVServer │
│  (Clerk) │  Get/Put/Append      │          │
└──────────┘ ◄────────────────── └──────────┘
                  Reply
```

---

## Step 2.1: 기본 KV 서버

### 체크리스트
- [ ] KVServer 구조체 정의
- [ ] Get(key) → value
- [ ] Put(key, value)
- [ ] Append(key, value)

### 구조체 설계
```go
type KVServer struct {
    mu   sync.Mutex
    data map[string]string
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) error {
    kv.mu.Lock()
    defer kv.mu.Unlock()

    reply.Value = kv.data[args.Key]
    return nil
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) error {
    kv.mu.Lock()
    defer kv.mu.Unlock()

    kv.data[args.Key] = args.Value
    return nil
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) error {
    kv.mu.Lock()
    defer kv.mu.Unlock()

    kv.data[args.Key] += args.Value
    return nil
}
```

---

## Step 2.2: 클라이언트 (Clerk)

### 체크리스트
- [ ] Clerk 구조체 정의
- [ ] RPC 호출 래퍼
- [ ] 재시도 로직

### 구조체 설계
```go
type Clerk struct {
    server   *labrpc.ClientEnd
    clientId int64
    seqNum   int64
}

func (ck *Clerk) Get(key string) string {
    args := &GetArgs{Key: key}
    reply := &GetReply{}

    for {
        ok := ck.server.Call("KVServer.Get", args, reply)
        if ok {
            return reply.Value
        }
        time.Sleep(100 * time.Millisecond)
    }
}

func (ck *Clerk) Put(key, value string) {
    ck.PutAppend(key, value, "Put")
}

func (ck *Clerk) Append(key, value string) {
    ck.PutAppend(key, value, "Append")
}
```

---

## Step 2.3: 동시성 처리

### 체크리스트
- [ ] 락을 통한 동시 접근 제어
- [ ] 경쟁 상태 테스트 (`-race`)

### 동시성 테스트
```bash
go test -race -run TestConcurrent
```

### 주의사항
- 모든 공유 데이터 접근 시 락 사용
- `defer mu.Unlock()` 패턴 권장
- 락 범위는 최소화

---

## Step 2.4: 중복 요청 감지

### 문제 상황
```
Client ──Put(x,1)──► Server (처리됨)
       ◄──────────── (응답 손실)
Client ──Put(x,1)──► Server (중복 처리!)
```

### 체크리스트
- [ ] 클라이언트 ID + 요청 번호
- [ ] 중복 요청 필터링
- [ ] Exactly-once 시맨틱

### 해결 방법
```go
type KVServer struct {
    mu       sync.Mutex
    data     map[string]string
    // 중복 감지용
    lastSeq  map[int64]int64  // clientId → last seqNum
    lastReply map[int64]string // clientId → last reply (for Get)
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) error {
    kv.mu.Lock()
    defer kv.mu.Unlock()

    // 중복 요청 체크
    if args.SeqNum <= kv.lastSeq[args.ClientId] {
        return nil  // 이미 처리됨
    }

    // 처리
    if args.Op == "Put" {
        kv.data[args.Key] = args.Value
    } else {
        kv.data[args.Key] += args.Value
    }

    // 기록
    kv.lastSeq[args.ClientId] = args.SeqNum
    return nil
}
```

### 클라이언트 측
```go
func (ck *Clerk) PutAppend(key, value, op string) {
    ck.seqNum++
    args := &PutAppendArgs{
        Key:      key,
        Value:    value,
        Op:       op,
        ClientId: ck.clientId,
        SeqNum:   ck.seqNum,
    }

    for {
        reply := &PutAppendReply{}
        ok := ck.server.Call("KVServer.PutAppend", args, reply)
        if ok {
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
}
```

---

## 테스트

```bash
cd src/kvsrv

# 전체 테스트
go test -race

# 개별 테스트
go test -race -run TestBasic
go test -race -run TestConcurrent
```

---

## 체크리스트

- [ ] 기본 Get/Put/Append 동작
- [ ] 동시 요청 처리
- [ ] 중복 요청 감지 (Exactly-once)
- [ ] 모든 테스트 통과 (`go test -race`)

## 다음 단계

KV Server를 마쳤다면 → [Phase 3: Raft](phase3-raft.md)

이 Lab에서 배운 중복 요청 감지는 Lab 4 (KV Raft)에서 다시 사용됩니다!

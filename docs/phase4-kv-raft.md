# Phase 4: Fault-tolerant KV Service (MIT Lab 4)

> **예상 시간**: ~25시간
> **난이도**: Hard
> **선수 조건**: Phase 3 (Raft) 완료
> **핵심**: 선형성(Linearizability) 보장

## 목표

Raft 위에 장애 허용 KV 서비스 구축 - 노드가 죽어도 데이터가 유지됨!

## 개념 이해

```
┌─────────┐      ┌─────────┐      ┌─────────┐
│ Client  │      │ Client  │      │ Client  │
└────┬────┘      └────┬────┘      └────┬────┘
     │                │                │
     └───────────┬────┴────────────────┘
                 │
                 ▼
     ┌───────────────────────┐
     │      KV Service       │
     ├───────────────────────┤
     │  ┌─────┐ ┌─────┐ ┌─────┐
     │  │KV   │ │KV   │ │KV   │
     │  │Srv 1│ │Srv 2│ │Srv 3│
     │  └──┬──┘ └──┬──┘ └──┬──┘
     │     │       │       │
     │  ┌──┴──┐ ┌──┴──┐ ┌──┴──┐
     │  │Raft │ │Raft │ │Raft │
     │  │  1  │ │  2  │ │  3  │
     │  └─────┘ └─────┘ └─────┘
     └───────────────────────┘
```

---

## Step 4A: KV Service without Snapshots

### 체크리스트
- [ ] KVServer 구조체 (Raft 포함)
- [ ] 클라이언트 요청 처리 (Get, Put, Append)
- [ ] Leader 리다이렉션
- [ ] 중복 요청 감지 (Exactly-once)

### KVServer 구조체
```go
type Op struct {
    Type     string // "Get", "Put", "Append"
    Key      string
    Value    string
    ClientId int64
    SeqNum   int64
}

type KVServer struct {
    mu      sync.Mutex
    me      int
    rf      *raft.Raft
    applyCh chan raft.ApplyMsg
    dead    int32

    maxraftstate int

    // KV 저장소
    data map[string]string

    // 중복 감지
    lastSeq   map[int64]int64    // clientId → last seqNum
    lastReply map[int64]string   // clientId → last reply

    // 요청 대기
    waitCh map[int]chan Op       // logIndex → channel
}
```

### 요청 처리 흐름
```go
func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
    op := Op{
        Type:     "Get",
        Key:      args.Key,
        ClientId: args.ClientId,
        SeqNum:   args.SeqNum,
    }

    // 1. Raft에 제출
    index, _, isLeader := kv.rf.Start(op)
    if !isLeader {
        reply.Err = ErrWrongLeader
        return
    }

    // 2. Apply 대기
    kv.mu.Lock()
    ch := make(chan Op, 1)
    kv.waitCh[index] = ch
    kv.mu.Unlock()

    select {
    case appliedOp := <-ch:
        if appliedOp.ClientId == op.ClientId && appliedOp.SeqNum == op.SeqNum {
            kv.mu.Lock()
            reply.Value = kv.data[args.Key]
            reply.Err = OK
            kv.mu.Unlock()
        } else {
            reply.Err = ErrWrongLeader
        }
    case <-time.After(500 * time.Millisecond):
        reply.Err = ErrWrongLeader
    }

    // 3. 정리
    kv.mu.Lock()
    delete(kv.waitCh, index)
    kv.mu.Unlock()
}
```

### Apply 루프
```go
func (kv *KVServer) applier() {
    for msg := range kv.applyCh {
        if msg.CommandValid {
            op := msg.Command.(Op)

            kv.mu.Lock()

            // 중복 체크
            if op.SeqNum > kv.lastSeq[op.ClientId] {
                switch op.Type {
                case "Put":
                    kv.data[op.Key] = op.Value
                case "Append":
                    kv.data[op.Key] += op.Value
                }
                kv.lastSeq[op.ClientId] = op.SeqNum
            }

            // 대기 중인 요청에 알림
            if ch, ok := kv.waitCh[msg.CommandIndex]; ok {
                ch <- op
            }

            kv.mu.Unlock()
        }
    }
}
```

### Clerk (클라이언트)
```go
type Clerk struct {
    servers  []*labrpc.ClientEnd
    leaderId int
    clientId int64
    seqNum   int64
}

func (ck *Clerk) Get(key string) string {
    ck.seqNum++
    args := &GetArgs{
        Key:      key,
        ClientId: ck.clientId,
        SeqNum:   ck.seqNum,
    }

    for {
        reply := &GetReply{}
        ok := ck.servers[ck.leaderId].Call("KVServer.Get", args, reply)

        if ok && reply.Err == OK {
            return reply.Value
        }

        // 다른 서버 시도
        ck.leaderId = (ck.leaderId + 1) % len(ck.servers)
        time.Sleep(100 * time.Millisecond)
    }
}
```

### 테스트
```bash
go test -race -run TestBasic4A
go test -race -run TestConcurrent4A
go test -race -run TestUnreliable4A
```

---

## Step 4B: KV Service with Snapshots

### 체크리스트
- [ ] Raft 로그 크기 모니터링
- [ ] Snapshot 생성 (KV 상태 + 세션 테이블)
- [ ] Snapshot 복구

### Snapshot 생성
```go
func (kv *KVServer) applier() {
    for msg := range kv.applyCh {
        if msg.CommandValid {
            // ... 기존 apply 로직

            // Snapshot 체크
            if kv.maxraftstate != -1 &&
               kv.rf.GetStateSize() >= kv.maxraftstate {
                kv.takeSnapshot(msg.CommandIndex)
            }
        } else if msg.SnapshotValid {
            kv.applySnapshot(msg.Snapshot)
        }
    }
}

func (kv *KVServer) takeSnapshot(index int) {
    w := new(bytes.Buffer)
    e := labgob.NewEncoder(w)
    e.Encode(kv.data)
    e.Encode(kv.lastSeq)

    kv.rf.Snapshot(index, w.Bytes())
}
```

### Snapshot 복구
```go
func (kv *KVServer) applySnapshot(snapshot []byte) {
    if snapshot == nil || len(snapshot) < 1 {
        return
    }

    r := bytes.NewBuffer(snapshot)
    d := labgob.NewDecoder(r)

    var data map[string]string
    var lastSeq map[int64]int64

    if d.Decode(&data) != nil || d.Decode(&lastSeq) != nil {
        return
    }

    kv.data = data
    kv.lastSeq = lastSeq
}
```

### 테스트
```bash
go test -race -run TestSnapshotRPC4B
go test -race -run TestSnapshotRecover4B
go test -race -run TestSnapshotUnreliable4B
```

---

## 주의사항

### 선형성 (Linearizability)
- 모든 읽기는 가장 최근 쓰기 결과를 반영해야 함
- Get도 Raft 로그에 기록해야 함 (stale read 방지)

### 일반적인 버그
1. **중복 요청 처리 누락** → 같은 요청 여러 번 적용
2. **Leader 변경 감지 실패** → 잘못된 응답
3. **Snapshot에 세션 테이블 누락** → 복구 후 중복 적용

---

## 체크리스트

- [ ] Step 4A: KV Service without Snapshots 완료
- [ ] Step 4B: KV Service with Snapshots 완료
- [ ] 전체 테스트 통과: `go test -race`

## 다음 단계

KV Raft를 마쳤다면 → [Phase 5: Sharded KV](phase5-sharded-kv.md)

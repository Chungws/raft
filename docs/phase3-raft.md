# Phase 3: Raft (MIT Lab 3)

> **예상 시간**: ~40시간
> **난이도**: Hard
> **논문**: [In Search of an Understandable Consensus Algorithm (Extended)](https://raft.github.io/raft.pdf)
> **시각화**: [raft.github.io](https://raft.github.io/)

## 목표

Raft 합의 알고리즘 완전 구현 - 분산 시스템의 핵심!

**핵심 원칙**: Figure 2를 정확히 따를 것!

## Raft 개요

```
┌─────────────────────────────────────────────────────────────┐
│                         Raft                                 │
├─────────────────────────────────────────────────────────────┤
│  Leader Election     Log Replication     Safety             │
│  (3A)                (3B)                                   │
├─────────────────────────────────────────────────────────────┤
│  Persistence         Log Compaction                         │
│  (3C)                (3D)                                   │
└─────────────────────────────────────────────────────────────┘
```

## 노드 상태

```
                  timeout, start election
            ┌─────────────────────────────────┐
            │                                 ▼
        ┌───────┐                        ┌───────────┐
   ────►│Follower│                        │ Candidate │
        └───────┘                        └───────────┘
            ▲                                 │
            │         discovers               │ receives votes from
            │         current leader          │ majority of servers
            │         or new term             ▼
            │                            ┌────────┐
            └────────────────────────────│ Leader │
                                         └────────┘
```

---

## Step 3.0: 기본 구조

### 체크리스트
- [ ] 디렉토리 구조 설정
- [ ] Raft 구조체 기본 필드 (Figure 2)
- [ ] NodeState enum (Follower, Candidate, Leader)
- [ ] ApplyMsg 구조체

### Raft 구조체 (Figure 2)
```go
type NodeState int

const (
    Follower NodeState = iota
    Candidate
    Leader
)

type LogEntry struct {
    Command interface{}
    Term    int
    Index   int
}

type Raft struct {
    mu        sync.Mutex
    peers     []*labrpc.ClientEnd
    persister *Persister
    me        int
    dead      int32

    // Persistent state (Figure 2)
    currentTerm int
    votedFor    int        // -1 if none
    log         []LogEntry

    // Volatile state
    commitIndex int
    lastApplied int

    // Leader only (reinitialized after election)
    nextIndex  []int
    matchIndex []int

    // Additional
    state           NodeState
    electionTimeout time.Time
    applyCh         chan ApplyMsg
}
```

### ApplyMsg
```go
type ApplyMsg struct {
    CommandValid bool
    Command      interface{}
    CommandIndex int

    SnapshotValid bool
    Snapshot      []byte
    SnapshotTerm  int
    SnapshotIndex int
}
```

---

## Step 3A: Leader Election

### 체크리스트
- [ ] Election timeout (랜덤 150-300ms)
- [ ] RequestVote RPC
- [ ] 투표 조건 구현
- [ ] 선거 진행 (병렬 goroutine)
- [ ] Heartbeat

### RequestVote RPC
```go
type RequestVoteArgs struct {
    Term         int
    CandidateId  int
    LastLogIndex int
    LastLogTerm  int
}

type RequestVoteReply struct {
    Term        int
    VoteGranted bool
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    reply.Term = rf.currentTerm
    reply.VoteGranted = false

    // 1. term이 작으면 거부
    if args.Term < rf.currentTerm {
        return
    }

    // 2. term이 크면 follower로 전환
    if args.Term > rf.currentTerm {
        rf.currentTerm = args.Term
        rf.state = Follower
        rf.votedFor = -1
    }

    // 3. 투표 조건 검사
    // - 아직 투표 안했거나, 같은 candidate에게 투표했음
    // - candidate의 로그가 최신임
    if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) &&
       rf.isLogUpToDate(args.LastLogIndex, args.LastLogTerm) {
        reply.VoteGranted = true
        rf.votedFor = args.CandidateId
        rf.resetElectionTimeout()
    }
}
```

### 로그 최신성 비교 (Election Restriction)
```go
func (rf *Raft) isLogUpToDate(candidateIndex, candidateTerm int) bool {
    lastIndex := rf.getLastLogIndex()
    lastTerm := rf.getLastLogTerm()

    // term이 다르면 term이 큰 쪽이 최신
    // term이 같으면 index가 큰 쪽이 최신
    if candidateTerm != lastTerm {
        return candidateTerm > lastTerm
    }
    return candidateIndex >= lastIndex
}
```

### 선거 시작
```go
func (rf *Raft) startElection() {
    rf.mu.Lock()
    rf.state = Candidate
    rf.currentTerm++
    rf.votedFor = rf.me
    rf.resetElectionTimeout()

    term := rf.currentTerm
    lastLogIndex := rf.getLastLogIndex()
    lastLogTerm := rf.getLastLogTerm()
    rf.mu.Unlock()

    votes := 1  // 자신에게 투표

    for i := range rf.peers {
        if i == rf.me {
            continue
        }

        go func(server int) {
            args := &RequestVoteArgs{
                Term:         term,
                CandidateId:  rf.me,
                LastLogIndex: lastLogIndex,
                LastLogTerm:  lastLogTerm,
            }
            reply := &RequestVoteReply{}

            if rf.sendRequestVote(server, args, reply) {
                rf.mu.Lock()
                defer rf.mu.Unlock()

                if reply.Term > rf.currentTerm {
                    rf.currentTerm = reply.Term
                    rf.state = Follower
                    rf.votedFor = -1
                    return
                }

                if rf.state == Candidate && reply.VoteGranted {
                    votes++
                    if votes > len(rf.peers)/2 {
                        rf.state = Leader
                        rf.initLeaderState()
                        go rf.sendHeartbeats()
                    }
                }
            }
        }(i)
    }
}
```

### 테스트
```bash
go test -race -run TestInitialElection3A
go test -race -run TestReElection3A
go test -race -run TestManyElections3A
```

---

## Step 3B: Log Replication

### 체크리스트
- [ ] LogEntry 구조체
- [ ] AppendEntries RPC
- [ ] 로그 일관성 검사
- [ ] Commit 처리
- [ ] Apply

### AppendEntries RPC
```go
type AppendEntriesArgs struct {
    Term         int
    LeaderId     int
    PrevLogIndex int
    PrevLogTerm  int
    Entries      []LogEntry
    LeaderCommit int
}

type AppendEntriesReply struct {
    Term    int
    Success bool

    // 빠른 롤백 최적화 (선택)
    ConflictIndex int
    ConflictTerm  int
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    reply.Term = rf.currentTerm
    reply.Success = false

    // 1. term 검사
    if args.Term < rf.currentTerm {
        return
    }

    if args.Term > rf.currentTerm {
        rf.currentTerm = args.Term
        rf.votedFor = -1
    }
    rf.state = Follower
    rf.resetElectionTimeout()

    // 2. 로그 일관성 검사
    if args.PrevLogIndex > rf.getLastLogIndex() {
        reply.ConflictIndex = rf.getLastLogIndex() + 1
        reply.ConflictTerm = -1
        return
    }

    if args.PrevLogIndex > 0 &&
       rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
        reply.ConflictTerm = rf.log[args.PrevLogIndex].Term
        // 해당 term의 첫 index 찾기
        for i := args.PrevLogIndex; i > 0; i-- {
            if rf.log[i-1].Term != reply.ConflictTerm {
                reply.ConflictIndex = i
                break
            }
        }
        return
    }

    // 3. 로그 append (충돌 시 덮어쓰기)
    for i, entry := range args.Entries {
        index := args.PrevLogIndex + 1 + i
        if index <= rf.getLastLogIndex() {
            if rf.log[index].Term != entry.Term {
                rf.log = rf.log[:index]
                rf.log = append(rf.log, args.Entries[i:]...)
                break
            }
        } else {
            rf.log = append(rf.log, args.Entries[i:]...)
            break
        }
    }

    // 4. commitIndex 업데이트
    if args.LeaderCommit > rf.commitIndex {
        rf.commitIndex = min(args.LeaderCommit, rf.getLastLogIndex())
        rf.applyLogs()
    }

    reply.Success = true
}
```

### Leader의 로그 복제
```go
func (rf *Raft) sendAppendEntries(server int) {
    rf.mu.Lock()
    if rf.state != Leader {
        rf.mu.Unlock()
        return
    }

    prevLogIndex := rf.nextIndex[server] - 1
    prevLogTerm := rf.log[prevLogIndex].Term
    entries := rf.log[rf.nextIndex[server]:]

    args := &AppendEntriesArgs{
        Term:         rf.currentTerm,
        LeaderId:     rf.me,
        PrevLogIndex: prevLogIndex,
        PrevLogTerm:  prevLogTerm,
        Entries:      entries,
        LeaderCommit: rf.commitIndex,
    }
    rf.mu.Unlock()

    reply := &AppendEntriesReply{}
    if rf.peers[server].Call("Raft.AppendEntries", args, reply) {
        rf.mu.Lock()
        defer rf.mu.Unlock()

        if reply.Term > rf.currentTerm {
            rf.currentTerm = reply.Term
            rf.state = Follower
            rf.votedFor = -1
            return
        }

        if reply.Success {
            rf.nextIndex[server] = prevLogIndex + len(entries) + 1
            rf.matchIndex[server] = rf.nextIndex[server] - 1
            rf.updateCommitIndex()
        } else {
            // 빠른 롤백
            if reply.ConflictTerm == -1 {
                rf.nextIndex[server] = reply.ConflictIndex
            } else {
                // ConflictTerm의 마지막 index 찾기
                rf.nextIndex[server] = reply.ConflictIndex
            }
        }
    }
}
```

### Commit 처리 (Figure 8 주의!)
```go
func (rf *Raft) updateCommitIndex() {
    // 현재 term의 로그만 직접 commit 가능
    for n := rf.getLastLogIndex(); n > rf.commitIndex; n-- {
        if rf.log[n].Term != rf.currentTerm {
            continue
        }

        count := 1
        for i := range rf.peers {
            if i != rf.me && rf.matchIndex[i] >= n {
                count++
            }
        }

        if count > len(rf.peers)/2 {
            rf.commitIndex = n
            rf.applyLogs()
            break
        }
    }
}
```

### 테스트
```bash
go test -race -run TestBasicAgree3B
go test -race -run TestFailAgree3B
go test -race -run TestFailNoAgree3B
go test -race -run TestRejoin3B
```

---

## Step 3C: Persistence

### 체크리스트
- [ ] Persister 인터페이스
- [ ] 영속화 대상: currentTerm, votedFor, log[]
- [ ] persist() 호출 시점
- [ ] 복구 (readPersist)

### 영속화
```go
func (rf *Raft) persist() {
    w := new(bytes.Buffer)
    e := labgob.NewEncoder(w)
    e.Encode(rf.currentTerm)
    e.Encode(rf.votedFor)
    e.Encode(rf.log)
    rf.persister.SaveRaftState(w.Bytes())
}

func (rf *Raft) readPersist(data []byte) {
    if data == nil || len(data) < 1 {
        return
    }
    r := bytes.NewBuffer(data)
    d := labgob.NewDecoder(r)

    var currentTerm int
    var votedFor int
    var log []LogEntry

    if d.Decode(&currentTerm) != nil ||
       d.Decode(&votedFor) != nil ||
       d.Decode(&log) != nil {
        // error handling
    } else {
        rf.currentTerm = currentTerm
        rf.votedFor = votedFor
        rf.log = log
    }
}
```

### persist() 호출 시점
- currentTerm 변경 시
- votedFor 변경 시
- log 변경 시

### 테스트
```bash
go test -race -run TestPersist13C
go test -race -run TestPersist23C
go test -race -run TestPersist33C
```

---

## Step 3D: Log Compaction (Snapshot)

### 체크리스트
- [ ] Snapshot 메타데이터
- [ ] 로그 트리밍 (index offset)
- [ ] InstallSnapshot RPC
- [ ] Persistence 통합

### Snapshot 구조
```go
// Raft 구조체에 추가
type Raft struct {
    // ... 기존 필드

    lastIncludedIndex int
    lastIncludedTerm  int
}

// 실제 log index → 배열 index
func (rf *Raft) getLogIndex(index int) int {
    return index - rf.lastIncludedIndex
}
```

### InstallSnapshot RPC
```go
type InstallSnapshotArgs struct {
    Term              int
    LeaderId          int
    LastIncludedIndex int
    LastIncludedTerm  int
    Data              []byte
}

type InstallSnapshotReply struct {
    Term int
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    reply.Term = rf.currentTerm

    if args.Term < rf.currentTerm {
        return
    }

    if args.Term > rf.currentTerm {
        rf.currentTerm = args.Term
        rf.votedFor = -1
    }
    rf.state = Follower
    rf.resetElectionTimeout()

    if args.LastIncludedIndex <= rf.lastIncludedIndex {
        return
    }

    // 로그 트리밍
    if args.LastIncludedIndex < rf.getLastLogIndex() {
        rf.log = rf.log[rf.getLogIndex(args.LastIncludedIndex)+1:]
    } else {
        rf.log = []LogEntry{}
    }

    rf.lastIncludedIndex = args.LastIncludedIndex
    rf.lastIncludedTerm = args.LastIncludedTerm

    rf.persister.SaveStateAndSnapshot(rf.encodeState(), args.Data)

    // 상위 서비스에 snapshot 전달
    rf.applyCh <- ApplyMsg{
        SnapshotValid: true,
        Snapshot:      args.Data,
        SnapshotTerm:  args.LastIncludedTerm,
        SnapshotIndex: args.LastIncludedIndex,
    }
}
```

### 테스트
```bash
go test -race -run TestSnapshotBasic3D
go test -race -run TestSnapshotInstall3D
go test -race -run TestSnapshotInstallCrash3D
```

---

## 디버깅 팁

### DPrintf 패턴
```go
const Debug = true

func DPrintf(format string, a ...interface{}) {
    if Debug {
        log.Printf(format, a...)
    }
}

// 사용 예시
DPrintf("[%d] term=%d state=%v: received vote from %d",
    rf.me, rf.currentTerm, rf.state, server)
```

### 일반적인 버그
1. **락을 잡고 RPC 호출** → 데드락
2. **election timeout 리셋 누락** → 불필요한 재선거
3. **Figure 2 조건 누락** → 미묘한 버그
4. **persist() 호출 누락** → 크래시 후 상태 손실

---

## 체크리스트

- [ ] Step 3A: Leader Election 완료
- [ ] Step 3B: Log Replication 완료
- [ ] Step 3C: Persistence 완료
- [ ] Step 3D: Log Compaction 완료
- [ ] 전체 테스트 통과: `go test -race`

## 다음 단계

Raft를 마쳤다면 → [Phase 4: KV Raft](phase4-kv-raft.md)

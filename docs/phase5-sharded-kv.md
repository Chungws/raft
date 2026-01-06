# Phase 5: Sharded Key/Value Service (MIT Lab 5)

> **예상 시간**: ~30시간
> **난이도**: Hard
> **선수 조건**: Phase 4 (KV Raft) 완료
> **핵심**: 샤딩, 설정 변경, 샤드 마이그레이션

## 목표

샤딩된 분산 KV 서비스 구축 - 수평 확장 가능한 시스템!

## 개념 이해

```
                    ┌─────────────────┐
                    │ Shard Controller│
                    │   (Raft 복제)    │
                    └────────┬────────┘
                             │ Config
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   Group 100     │ │   Group 101     │ │   Group 102     │
│ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
│ │ Shards 0,3  │ │ │ │ Shards 1,4  │ │ │ │ Shards 2,5  │ │
│ │ (Raft 복제) │ │ │ │ (Raft 복제) │ │ │ │ (Raft 복제) │ │
│ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

### 샤딩 개념
- **Shard**: 키 공간의 부분집합 (예: 10개 샤드)
- **Group**: Raft로 복제된 서버 그룹
- **Config**: 샤드 → 그룹 매핑

```
key2shard(key) = hash(key) % NShards
Shards: [G100, G101, G102, G100, G101, G102, G100, G101, G102, G100]
         S0    S1    S2    S3    S4    S5    S6    S7    S8    S9
```

---

## Step 5A: Shard Controller

### 체크리스트
- [ ] ShardController 구조체 (Raft 기반)
- [ ] Config 구조체
- [ ] Join, Leave, Move, Query 연산
- [ ] 샤드 리밸런싱

### Config 구조체
```go
const NShards = 10

type Config struct {
    Num    int                // 설정 번호
    Shards [NShards]int       // shard → gid
    Groups map[int][]string   // gid → servers
}
```

### ShardController 구조체
```go
type ShardCtrler struct {
    mu      sync.Mutex
    me      int
    rf      *raft.Raft
    applyCh chan raft.ApplyMsg

    configs []Config  // 설정 히스토리
}
```

### 연산

#### Join - 새 그룹 추가
```go
type JoinArgs struct {
    Servers map[int][]string  // gid → servers
}

func (sc *ShardCtrler) Join(args *JoinArgs, reply *JoinReply) {
    // 1. 현재 config 복사
    // 2. 새 그룹 추가
    // 3. 샤드 리밸런싱
    // 4. 새 config 저장
}
```

#### Leave - 그룹 제거
```go
type LeaveArgs struct {
    GIDs []int
}

func (sc *ShardCtrler) Leave(args *LeaveArgs, reply *LeaveReply) {
    // 1. 현재 config 복사
    // 2. 그룹 제거
    // 3. 해당 그룹의 샤드 재분배
    // 4. 새 config 저장
}
```

#### Move - 샤드 이동
```go
type MoveArgs struct {
    Shard int
    GID   int
}

func (sc *ShardCtrler) Move(args *MoveArgs, reply *MoveReply) {
    // 특정 샤드를 특정 그룹으로 이동
}
```

#### Query - 설정 조회
```go
type QueryArgs struct {
    Num int  // -1이면 최신
}

func (sc *ShardCtrler) Query(args *QueryArgs, reply *QueryReply) {
    // 해당 번호의 config 반환
}
```

### 리밸런싱 알고리즘
```go
func (sc *ShardCtrler) rebalance(config *Config) {
    // 목표: 그룹 간 샤드 수 차이가 1 이하

    gids := []int{} // 활성 그룹 목록
    for gid := range config.Groups {
        gids = append(gids, gid)
    }

    if len(gids) == 0 {
        // 모든 샤드를 0(무효)으로
        for i := range config.Shards {
            config.Shards[i] = 0
        }
        return
    }

    // 그룹별 샤드 수 계산
    avg := NShards / len(gids)
    extra := NShards % len(gids)

    // 과잉 샤드 → 부족 그룹으로 이동
    // ... 구현
}
```

### 테스트
```bash
go test -race -run TestBasic5A
go test -race -run TestMulti5A
```

---

## Step 5B: Sharded KV Server

### 체크리스트
- [ ] ShardKV 구조체
- [ ] 샤드 소유권 검사
- [ ] 설정 변경 감지
- [ ] 샤드 마이그레이션
- [ ] 가비지 컬렉션

### ShardKV 구조체
```go
type ShardKV struct {
    mu           sync.Mutex
    me           int
    rf           *raft.Raft
    applyCh      chan raft.ApplyMsg
    gid          int
    ctrlers      []*labrpc.ClientEnd
    maxraftstate int

    // 샤드별 데이터
    data       [NShards]map[string]string
    shardState [NShards]ShardState  // Serving, Pulling, Pushing, Invalid

    // 현재 설정
    config     shardctrler.Config
    prevConfig shardctrler.Config

    // 중복 감지
    lastSeq [NShards]map[int64]int64
}

type ShardState int

const (
    Serving ShardState = iota
    Pulling  // 다른 그룹에서 가져오는 중
    Pushing  // 다른 그룹으로 보내는 중
    Invalid  // 담당하지 않음
)
```

### 요청 처리
```go
func (kv *ShardKV) Get(args *GetArgs, reply *GetReply) {
    shard := key2shard(args.Key)

    kv.mu.Lock()
    // 1. 샤드 소유권 체크
    if kv.config.Shards[shard] != kv.gid {
        reply.Err = ErrWrongGroup
        kv.mu.Unlock()
        return
    }

    // 2. 샤드 상태 체크
    if kv.shardState[shard] != Serving {
        reply.Err = ErrWrongGroup
        kv.mu.Unlock()
        return
    }
    kv.mu.Unlock()

    // 3. Raft에 제출
    // ... Lab 4와 유사
}
```

### 설정 변경 감지
```go
func (kv *ShardKV) configPoller() {
    for !kv.killed() {
        kv.mu.Lock()
        currentNum := kv.config.Num
        kv.mu.Unlock()

        // 다음 설정 조회
        newConfig := kv.mck.Query(currentNum + 1)

        if newConfig.Num == currentNum+1 {
            // 설정 변경을 Raft에 제출
            kv.rf.Start(ConfigOp{Config: newConfig})
        }

        time.Sleep(100 * time.Millisecond)
    }
}
```

### 샤드 마이그레이션
```go
// 새 설정 적용 시
func (kv *ShardKV) applyConfig(newConfig Config) {
    for shard := 0; shard < NShards; shard++ {
        oldGid := kv.config.Shards[shard]
        newGid := newConfig.Shards[shard]

        if oldGid != kv.gid && newGid == kv.gid {
            // 다른 그룹에서 가져와야 함
            kv.shardState[shard] = Pulling
        } else if oldGid == kv.gid && newGid != kv.gid {
            // 다른 그룹으로 보내야 함
            kv.shardState[shard] = Pushing
        }
    }

    kv.prevConfig = kv.config
    kv.config = newConfig
}

// 샤드 가져오기
func (kv *ShardKV) shardPuller() {
    for !kv.killed() {
        kv.mu.Lock()
        for shard, state := range kv.shardState {
            if state == Pulling {
                go kv.pullShard(shard)
            }
        }
        kv.mu.Unlock()
        time.Sleep(100 * time.Millisecond)
    }
}

// RPC: 샤드 데이터 요청
type PullShardArgs struct {
    ConfigNum int
    Shard     int
}

type PullShardReply struct {
    Data    map[string]string
    LastSeq map[int64]int64
    Err     Err
}
```

### 가비지 컬렉션
```go
// 마이그레이션 완료 후 이전 데이터 삭제
func (kv *ShardKV) shardGC() {
    // Pushing 상태의 샤드 중
    // 새 담당 그룹이 수신 완료한 것 삭제
}
```

### 테스트
```bash
go test -race -run TestStaticShards5B
go test -race -run TestJoinLeave5B
go test -race -run TestSnapshot5B
go test -race -run TestConcurrent5B
```

---

## 주의사항

### 설정 변경 순서
- Config는 순차적으로 적용 (N → N+1)
- 모든 샤드 마이그레이션 완료 후 다음 Config 적용

### 마이그레이션 중 요청 처리
- Pulling 상태: ErrWrongGroup 반환
- Pushing 상태: 계속 처리 (데이터가 아직 있으므로)

### 일반적인 버그
1. **Config 점프** → 중간 상태 누락
2. **데드락** → 마이그레이션 양방향 대기
3. **중복 마이그레이션** → 데이터 불일치

---

## 체크리스트

- [ ] Step 5A: Shard Controller 완료
- [ ] Step 5B: Sharded KV Server 완료
- [ ] 전체 테스트 통과: `go test -race`

## 완료!

축하합니다! MIT 6.5840 전체 Lab을 완료했습니다!

구현한 시스템:
```
┌────────────────────────────────────────────┐
│          Sharded KV Service                │
│  (수평 확장 가능한 분산 KV 저장소)           │
├────────────────────────────────────────────┤
│          KV Raft Service                   │
│  (장애 허용 KV 서비스)                      │
├────────────────────────────────────────────┤
│              Raft                          │
│  (합의 알고리즘)                            │
├────────────────────────────────────────────┤
│            MapReduce                       │
│  (분산 데이터 처리)                         │
└────────────────────────────────────────────┘
```

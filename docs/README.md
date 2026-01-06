# MIT 6.5840 Distributed Systems - Full Curriculum

MIT 6.5840 (구 6.824) 분산 시스템 과정을 TDD 방식으로 완전 구현

## 전체 구조

```
┌─────────────────────────────────────────────────────────────┐
│  Phase 0: Go 기초                          (~10시간)        │
├─────────────────────────────────────────────────────────────┤
│  Phase 1: Lab 1 - MapReduce                (~15시간)        │
├─────────────────────────────────────────────────────────────┤
│  Phase 2: Lab 2 - Key/Value Server         (~10시간)        │
├─────────────────────────────────────────────────────────────┤
│  Phase 3: Lab 3 - Raft (3A~3D)             (~40시간) ★      │
├─────────────────────────────────────────────────────────────┤
│  Phase 4: Lab 4 - KV Raft (4A~4B)          (~25시간)        │
├─────────────────────────────────────────────────────────────┤
│  Phase 5: Lab 5 - Sharded KV (5A~5B)       (~30시간)        │
└─────────────────────────────────────────────────────────────┘
                                        Total: ~130시간
```

## 진행 상황

- [ ] [Phase 0: Go 기초](phase0-go-basics.md)
- [ ] [Phase 1: MapReduce](phase1-mapreduce.md) (Lab 1)
- [ ] [Phase 2: KV Server](phase2-kv-server.md) (Lab 2)
- [ ] [Phase 3: Raft](phase3-raft.md) (Lab 3)
  - [ ] 3A: Leader Election
  - [ ] 3B: Log Replication
  - [ ] 3C: Persistence
  - [ ] 3D: Log Compaction
- [ ] [Phase 4: KV Raft](phase4-kv-raft.md) (Lab 4)
  - [ ] 4A: Without Snapshots
  - [ ] 4B: With Snapshots
- [ ] [Phase 5: Sharded KV](phase5-sharded-kv.md) (Lab 5)
  - [ ] 5A: Shard Controller
  - [ ] 5B: Sharded KV Server

## 예상 소요 시간

| Phase | Lab | 난이도 | 예상 시간 |
|-------|-----|--------|----------|
| 0 | Go 기초 | Easy | ~10시간 |
| 1 | MapReduce | Moderate | ~15시간 |
| 2 | KV Server | Moderate | ~10시간 |
| 3 | Raft | **Hard** | ~40시간 |
| 4 | KV Raft | Hard | ~25시간 |
| 5 | Sharded KV | Hard | ~30시간 |
| | **Total** | | **~130시간** |

## 디렉토리 구조

```
mit-6.5840/
├── docs/
│   ├── README.md              # 이 파일
│   ├── phase0-go-basics.md
│   ├── phase1-mapreduce.md
│   ├── phase2-kv-server.md
│   ├── phase3-raft.md
│   ├── phase4-kv-raft.md
│   ├── phase5-sharded-kv.md
│   └── resources.md
├── src/
│   ├── mr/                    # Lab 1
│   ├── kvsrv/                 # Lab 2
│   ├── raft/                  # Lab 3
│   ├── kvraft/                # Lab 4
│   ├── shardctrler/           # Lab 5A
│   ├── shardkv/               # Lab 5B
│   └── labrpc/
└── go.mod
```

## 빠른 시작

```bash
# 테스트 실행 예시
cd src/raft && go test -race -run TestInitialElection
```

## 참고 자료

[resources.md](resources.md) 참조

---

**Current Phase**: Phase 0 - Go 기초
**Last Updated**: 2026-01-06

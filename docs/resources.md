# 참고 자료

## 공식 자료

### MIT 6.5840
- [MIT 6.5840 Home](https://pdos.csail.mit.edu/6.824/)
- [Lab Schedule](https://pdos.csail.mit.edu/6.824/schedule.html)
- [Lab Guidance](https://pdos.csail.mit.edu/6.824/labs/guidance.html)
- [General Information](https://pdos.csail.mit.edu/6.824/general.html)

---

## 필수 논문

| 순서 | 논문 | 링크 | Phase |
|------|------|------|-------|
| 1 | MapReduce (2004) | [PDF](https://pdos.csail.mit.edu/6.824/papers/mapreduce.pdf) | Phase 1 |
| 2 | GFS (2003) | [PDF](https://pdos.csail.mit.edu/6.824/papers/gfs.pdf) | 배경지식 |
| 3 | **Raft Extended (2014)** | [PDF](https://raft.github.io/raft.pdf) | Phase 3 |
| 4 | ZooKeeper (2010) | [PDF](https://pdos.csail.mit.edu/6.824/papers/zookeeper.pdf) | 참고 |
| 5 | Spanner (2012) | [PDF](https://pdos.csail.mit.edu/6.824/papers/spanner.pdf) | 참고 |

---

## Raft 관련

### 필수
- [Raft 논문 (Extended Version)](https://raft.github.io/raft.pdf) - **반드시 Figure 2 숙지!**
- [Raft 시각화](https://raft.github.io/) - 인터랙티브 데모

### 추천
- [Students' Guide to Raft](https://thesquareplanet.com/blog/students-guide-to-raft/) - 구현 팁
- [Raft Q&A](https://thesquareplanet.com/blog/raft-qa/) - 자주 묻는 질문
- [Instructors' Guide to Raft](https://thesquareplanet.com/blog/instructors-guide-to-raft/)

### 디버깅
- [Debugging by Pretty Printing](https://blog.josejg.com/debugging-pretty/) - 로그 시각화

---

## Go 언어

### 입문
- [Go Tour](https://go.dev/tour/) - 공식 튜토리얼
- [Go by Example](https://gobyexample.com/) - 예제 기반 학습

### 심화
- [Effective Go](https://go.dev/doc/effective_go) - Go 스타일 가이드
- [Go Concurrency Patterns](https://go.dev/blog/pipelines) - 동시성 패턴

### 테스트
- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)
- [Race Detector](https://go.dev/doc/articles/race_detector) - `-race` 플래그

---

## 분산 시스템 일반

### 책
- *Designing Data-Intensive Applications* - Martin Kleppmann
- *Distributed Systems* - Maarten van Steen

### 온라인 강의
- [MIT 6.824 강의 영상](https://www.youtube.com/playlist?list=PLrw6a1wE39_tb2fErI4-WkMbsvGQk9_UB) (2020)

---

## 커뮤니티 구현체

### 참고용 (직접 구현 후 참고할 것!)
- [etcd/raft](https://github.com/etcd-io/etcd/tree/main/raft) - 프로덕션급
- [hashicorp/raft](https://github.com/hashicorp/raft) - 실용적 라이브러리

---

## 디버깅 도구

### 로그 시각화
```go
const Debug = true

func DPrintf(format string, a ...interface{}) {
    if Debug {
        log.Printf(format, a...)
    }
}
```

### 테스트 실행
```bash
# 단일 테스트
go test -race -run TestName

# 반복 테스트 (안정성 확인)
for i in {1..100}; do
    go test -race -run TestName || break
done

# 전체 테스트
go test -race

# 로그 저장
go test -race > out.log 2>&1
```

### 경쟁 상태 감지
```bash
go test -race ./...
```

---

## 팁 모음

### Raft 구현
1. **Figure 2를 정확히 따를 것** - 조건 하나라도 빠지면 버그
2. **락을 잡고 RPC 호출하지 말 것** - 데드락 유발
3. **election timeout은 랜덤하게** - 150-300ms
4. **persist() 호출 잊지 말 것** - 상태 변경 시마다

### 디버깅
1. **로그에 term, state 포함** - 상태 추적 용이
2. **타임스탬프 추가** - 시간 순서 파악
3. **노드 ID 구분** - 어떤 노드의 로그인지
4. **반복 테스트** - 간헐적 버그 발견

### 일반
1. **작은 단위로 테스트** - 큰 테스트 전에 작은 테스트 통과
2. **커밋 자주** - 동작하는 버전 보존
3. **논문 다시 읽기** - 막히면 논문으로 돌아가기

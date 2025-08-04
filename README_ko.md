## Languages | 언어

- [English](README.md)
- [한국어](README_ko.md) (현재)

---

# pg-metako

Go로 작성된 분산형 고가용성 PostgreSQL 클러스터 관리 시스템입니다.

## 개요

pg-metako는 다음과 같은 기능을 제공하는 분산형 PostgreSQL 클러스터 관리 시스템입니다:

- **분산 아키텍처**: 각 PostgreSQL 노드가 자체 pg-metako 인스턴스를 실행하여 진정한 분산 조정 제공
- **고가용성**: 분산 합의와 조정을 통한 자동 페일오버
- **지능적 라우팅**: 로컬 노드 선호도와 로드 밸런싱을 통한 쿼리 라우팅
- **상태 모니터링**: 클러스터 전체에서 구성 가능한 임계값을 통한 지속적인 상태 확인
- **확장성**: N-노드 분산 PostgreSQL 클러스터 지원
- **조정**: 페일오버 결정을 위한 노드 간 통신 및 합의
- **관찰 가능성**: 모든 노드에서 포괄적인 로깅 및 메트릭

## 기능

### 핵심 기능
- ✅ 분산 PostgreSQL 클러스터 관리 (최소 2개 노드)
- ✅ 분산 합의를 통한 승격 기능을 갖춘 마스터-슬레이브 아키텍처
- ✅ 모든 클러스터 노드에서 지속적인 상태 확인
- ✅ 분산 조정과 합의를 통한 자동 페일오버
- ✅ 로컬 노드 선호도를 통한 지능적 쿼리 라우팅
- ✅ 로드 밸런싱 알고리즘 (라운드 로빈, 최소 연결)
- ✅ 노드 간 통신 및 조정 API

### 구성 관리
- ✅ YAML 구성 파일
- ✅ 동적 구성 재로딩
- ✅ 포괄적인 검증

### 안정성 및 모니터링
- ✅ 네트워크 파티션의 우아한 처리
- ✅ 모든 작업에 대한 포괄적인 로깅
- ✅ 연결 통계 및 모니터링
- ✅ 주기적인 클러스터 상태 보고

## 빠른 시작

### 사전 요구사항

- Go 1.24 이상 (로컬 개발용)
- Docker 및 Docker Compose (컨테이너화된 배포용)
- 복제가 구성된 PostgreSQL 12+ (로컬 개발용)

### Docker 배포 (권장)

pg-metako를 실행하는 가장 쉬운 방법은 Docker Compose를 사용하는 것입니다:

1. 저장소 복제:
```bash
git clone <repository-url>
cd pg-metako
```

2. 환경 설정 및 서비스 시작:
```bash
./scripts/deploy.sh start
```

3. 서비스 상태 확인:
```bash
./scripts/deploy.sh status
```

### 로컬 개발

1. 저장소 복제:
```bash
git clone <repository-url>
cd pg-metako
```

2. 애플리케이션 빌드:
```bash
# Makefile 사용 (권장)
make build

# 또는 Go 직접 사용
go build -o bin/pg-metako ./cmd/pg-metako
```

3. 예제 구성으로 실행:
```bash
./bin/pg-metako --config configs/config.yaml
```

### 구성

분산 배포를 위한 YAML 구성 파일을 생성하세요:

```yaml
# 노드 식별 - 이 pg-metako 인스턴스를 식별
identity:
  node_name: "pg-metako-node1"    # 이 노드의 고유 이름
  local_db_host: "localhost"      # 로컬 PostgreSQL 인스턴스 호스트
  local_db_port: 5432            # 로컬 PostgreSQL 인스턴스 포트
  api_host: "0.0.0.0"            # 노드 간 API 호스트
  api_port: 8080                 # 노드 간 API 포트

# 로컬 PostgreSQL 데이터베이스 구성
local_db:
  name: "postgres-node1"
  host: "localhost"
  port: 5432
  role: "master"                 # 로컬 PostgreSQL 인스턴스의 역할
  username: "postgres"
  password: "password"
  database: "myapp"

# 클러스터의 다른 pg-metako 노드들
cluster_members:
  - node_name: "pg-metako-node2"
    api_host: "192.168.1.102"
    api_port: 8080
    role: "slave"
  
  - node_name: "pg-metako-node3"
    api_host: "192.168.1.103"
    api_port: 8080
    role: "slave"

# 분산 합의를 위한 조정 설정
coordination:
  heartbeat_interval: "10s"        # 클러스터 멤버십을 위한 하트비트 간격
  communication_timeout: "5s"     # 노드 간 통신 타임아웃
  failover_timeout: "30s"         # 페일오버 조정 타임아웃
  min_consensus_nodes: 2          # 페일오버 합의에 필요한 최소 노드 수
  local_node_preference: 0.8      # 로컬 노드 선호도 가중치 (0.0 ~ 1.0)

# 상태 확인 구성
health_check:
  interval: "30s"          # 노드 상태를 확인하는 주기
  timeout: "5s"            # 각 상태 확인의 타임아웃
  failure_threshold: 3     # 비정상으로 표시하기 전 연속 실패 횟수

# 로드 밸런서 구성
load_balancer:
  algorithm: "round_robin" # 옵션: round_robin, least_connected
  read_timeout: "10s"      # 읽기 쿼리 타임아웃
  write_timeout: "10s"     # 쓰기 쿼리 타임아웃

# 보안 구성
security:
  tls_enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
```

## 아키텍처

### 분산 아키텍처

pg-metako는 각 PostgreSQL 노드가 자체 pg-metako 인스턴스를 실행하는 분산 아키텍처를 사용합니다. 이는 진정한 분산 조정을 제공하고 단일 장애점을 제거합니다.

### 구성 요소

1. **구성 관리자**: 분산 YAML 구성을 로드하고 검증
2. **로컬 데이터베이스 연결 관리자**: 로컬 PostgreSQL 인스턴스에 대한 연결 관리
3. **상태 확인기**: 구성 가능한 간격으로 모든 클러스터 노드의 상태 모니터링
4. **분산 복제 관리자**: 노드 간 페일오버 조정 및 합의 처리
5. **조정 API**: 노드 간 통신 및 클러스터 멤버십 관리 제공
6. **통합 라우터**: 로컬 노드 선호도와 지능적 로드 밸런싱을 통한 쿼리 라우팅

### 분산 조정

- **클러스터 멤버십**: 각 노드는 다른 모든 pg-metako 노드에 대한 인식을 유지
- **하트비트 시스템**: 정기적인 하트비트로 클러스터 상태를 보장하고 노드 장애를 감지
- **합의 프로토콜**: 페일오버 결정 및 마스터 승격을 위한 분산 합의
- **로컬 노드 선호도**: 가능한 경우 쿼리를 로컬 PostgreSQL 인스턴스로 우선 라우팅

### 쿼리 라우팅

- **쓰기 쿼리**: 항상 현재 마스터 노드로 라우팅
  - `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER` 등
- **읽기 쿼리**: 정상 슬레이브 노드에 분산
  - `SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `WITH`

### 로드 밸런싱 알고리즘

1. **라운드 로빈**: 모든 정상 슬레이브에 쿼리를 균등하게 분산
2. **최소 연결**: 활성 연결이 가장 적은 슬레이브로 라우팅

### 분산 페일오버 프로세스

1. **장애 감지**: 여러 pg-metako 노드가 상태 확인을 통해 마스터 장애를 감지
2. **합의 시작**: 노드들이 통신하여 마스터 장애에 대한 합의에 도달
3. **리더 선출**: 분산 합의가 다음 기준에 따라 승격할 최적 후보를 선택:
   - PostgreSQL 복제 지연
   - 노드 상태 및 가용성
   - 로컬 노드 선호도 가중치
4. **조정된 승격**: 선택된 노드가 로컬 PostgreSQL 인스턴스를 마스터로 승격
5. **클러스터 재구성**: 모든 노드가 새 마스터를 사용하도록 라우팅 업데이트
6. **복제 재시작**: 나머지 슬레이브들이 새 마스터를 따르도록 재구성
7. **상태 동기화**: 클러스터 상태가 모든 pg-metako 노드에서 동기화

## 배포

pg-metako는 두 가지 배포 모델을 지원합니다:

1. **Docker 배포 (개발/테스트)**: Docker Compose를 사용한 빠른 설정
2. **분산 배포 (프로덕션)**: 각 PostgreSQL 노드가 자체 pg-metako 인스턴스를 실행

### 빠른 Docker 설정

개발 및 테스트를 위해 Docker Compose를 사용하세요:

```bash
# 모든 서비스 설정 및 시작
./scripts/deploy.sh start

# 상태 확인
./scripts/deploy.sh status
```

📖 **자세한 Docker 배포 지침은 [Docker 배포 가이드](docs/docker-deployment.md)를 참조하세요**

### 프로덕션 배포

프로덕션 환경에서는 각 PostgreSQL 노드가 자체 pg-metako 인스턴스를 실행하는 분산 모드로 배포하여 진정한 고가용성을 제공하세요.

📖 **자세한 프로덕션 배포 지침은 [분산 배포 가이드](docs/distributed-deployment.md)를 참조하세요**

## 사용법

### 명령줄 옵션

```bash
./bin/pg-metako [옵션]

옵션:
  --config string    구성 파일 경로 (기본값 "configs/config.yaml")
  --version         버전 정보 표시
```

### 애플리케이션 실행

1. **기본 구성으로 시작**:
```bash
./bin/pg-metako
```

2. **사용자 정의 구성으로 시작**:
```bash
./bin/pg-metako --config /path/to/config.yaml
```

3. **버전 확인**:
```bash
./bin/pg-metako --version
```

### 모니터링

애플리케이션은 JSON 형식의 포괄적인 구조화된 로깅을 제공합니다:

```
{"time":"2025-08-03T22:41:20.552941+09:00","level":"INFO","msg":"Loading configuration from configs/config.yaml"}
{"time":"2025-08-03T22:41:20.553267+09:00","level":"INFO","msg":"Configuration loaded successfully with 3 nodes"}
{"time":"2025-08-03T22:41:20.553269+09:00","level":"INFO","msg":"Added node master-1 (master) to cluster"}
{"time":"2025-08-03T22:41:20.553271+09:00","level":"INFO","msg":"Added node slave-1 (slave) to cluster"}
{"time":"2025-08-03T22:41:20.553276+09:00","level":"INFO","msg":"Health monitoring started"}
{"time":"2025-08-03T22:41:20.553277+09:00","level":"INFO","msg":"Failover monitoring started"}
{"time":"2025-08-03T22:41:20.553278+09:00","level":"INFO","msg":"Application started successfully"}
```

주기적인 상태 보고서에는 다음이 포함됩니다:
- 현재 마스터 노드
- 모든 노드의 상태
- 쿼리 통계 (총계, 읽기, 쓰기, 실패)
- 전체 클러스터 상태

## 개발

### 빠른 시작

```bash
# 애플리케이션 빌드
make build

# 테스트 실행
make test

# 예제 구성으로 실행
./bin/pg-metako --config configs/config.yaml
```

📖 **자세한 개발 설정, 프로젝트 구조, 테스트 및 기여 가이드라인은 [개발 가이드](docs/development.md)를 참조하세요**

## PostgreSQL 설정

PostgreSQL 마스터-슬레이브 복제 구성을 위해:

📖 **자세한 PostgreSQL 설정 지침은 [PostgreSQL 설정 가이드](docs/postgresql-setup.md)를 참조하세요**

## 문제 해결

일반적인 문제와 해결책을 위해:

📖 **포괄적인 문제 해결 정보는 [문제 해결 가이드](docs/troubleshooting.md)를 참조하세요**

## 라이선스

이 프로젝트는 MIT 라이선스 하에 라이선스됩니다 - 자세한 내용은 [LICENSE](LICENSE) 파일을 참조하세요.
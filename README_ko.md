## Languages | 언어

- [English](README.md)
- [한국어](README_ko.md) (현재)

---

# pg-metako

Go로 작성된 고가용성 PostgreSQL 복제 관리 시스템입니다.

## 개요

pg-metako는 다음과 같은 기능을 제공하는 강력한 PostgreSQL 복제 관리 시스템입니다:

- **고가용성**: 마스터 노드 장애 시 자동 페일오버
- **로드 밸런싱**: 다양한 알고리즘을 통한 지능적인 쿼리 라우팅
- **상태 모니터링**: 구성 가능한 임계값을 통한 지속적인 상태 확인
- **확장성**: N-노드 복제 설정 지원
- **관찰 가능성**: 포괄적인 로깅 및 메트릭

## 기능

### 핵심 기능
- ✅ N-노드 PostgreSQL 복제 지원 (최소 2개 노드)
- ✅ 자동 승격 기능을 갖춘 마스터-슬레이브 아키텍처
- ✅ 구성 가능한 간격으로 지속적인 상태 확인
- ✅ 구성 가능한 실패 임계값을 통한 자동 페일오버
- ✅ 쿼리 라우팅 (쓰기는 마스터로, 읽기는 슬레이브로)
- ✅ 로드 밸런싱 알고리즘 (라운드 로빈, 최소 연결)

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
./bin/pg-metako --config configs/example.yaml
```

### 구성

YAML 구성 파일을 생성하세요 (`configs/example.yaml` 참조):

```yaml
# 데이터베이스 노드 구성
nodes:
  - name: "master-1"
    host: "localhost"
    port: 5432
    role: "master"
    username: "postgres"
    password: "password"
    database: "myapp"
  
  - name: "slave-1"
    host: "localhost"
    port: 5433
    role: "slave"
    username: "postgres"
    password: "password"
    database: "myapp"

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

### 구성 요소

1. **구성 관리자**: YAML 구성을 로드하고 검증
2. **데이터베이스 연결 관리자**: PostgreSQL 연결 관리
3. **상태 확인기**: 구성 가능한 간격으로 노드 상태 모니터링
4. **복제 관리자**: 페일오버 및 슬레이브 승격 처리
5. **쿼리 라우터**: 유형 및 로드 밸런싱에 따른 쿼리 라우팅

### 쿼리 라우팅

- **쓰기 쿼리**: 항상 현재 마스터 노드로 라우팅
  - `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER` 등
- **읽기 쿼리**: 정상 슬레이브 노드에 분산
  - `SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `WITH`

### 로드 밸런싱 알고리즘

1. **라운드 로빈**: 모든 정상 슬레이브에 쿼리를 균등하게 분산
2. **최소 연결**: 활성 연결이 가장 적은 슬레이브로 라우팅

### 페일오버 프로세스

1. 상태 확인기가 마스터 장애를 감지 (연속 실패가 임계값 초과)
2. 복제 관리자가 승격할 정상 슬레이브를 선택
3. 슬레이브가 마스터로 승격 (PostgreSQL 승격 명령)
4. 나머지 슬레이브들이 새 마스터를 따르도록 재구성
5. 쿼리 라우터가 새 마스터를 사용하도록 라우팅 업데이트

## Docker 배포

### 개요

pg-metako는 다음을 포함한 완전한 Docker 기반 배포 솔루션을 제공합니다:
- 자동 복제 설정을 갖춘 PostgreSQL 마스터 및 슬레이브 컨테이너
- pg-metako 애플리케이션 컨테이너
- 데이터베이스 관리를 위한 선택적 pgAdmin
- 영구 데이터 볼륨
- 환경 변수를 통한 쉬운 구성

### 빠른 Docker 설정

1. **환경 설정**:
```bash
./scripts/deploy.sh setup
```

2. **구성 편집** (선택사항):
```bash
# .env 파일을 편집하여 비밀번호와 포트를 사용자 정의
nano .env
```

3. **모든 서비스 시작**:
```bash
./scripts/deploy.sh start
```

4. **상태 확인**:
```bash
./scripts/deploy.sh status
```

### Docker 서비스

Docker Compose 설정에는 다음이 포함됩니다:

- **postgres-master**: PostgreSQL 마스터 데이터베이스 (포트 5432)
- **postgres-slave1**: PostgreSQL 슬레이브 데이터베이스 (포트 5433)
- **postgres-slave2**: PostgreSQL 슬레이브 데이터베이스 (포트 5434)
- **pg-metako**: 메인 애플리케이션 컨테이너 (포트 8080)
- **pgadmin**: 데이터베이스 관리 인터페이스 (포트 8081, 선택사항)

### 환경 구성

`.env.example`을 `.env`로 복사하고 사용자 정의하세요:

```bash
# PostgreSQL 구성
POSTGRES_DB=myapp
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_secure_password

# 복제 구성
POSTGRES_REPLICATION_USER=replicator
POSTGRES_REPLICATION_PASSWORD=your_replication_password

# 포트 구성
POSTGRES_MASTER_PORT=5432
POSTGRES_SLAVE1_PORT=5433
POSTGRES_SLAVE2_PORT=5434
PG_METAKO_PORT=8080
```

### 배포 스크립트 명령

`scripts/deploy.sh` 스크립트는 쉬운 관리를 제공합니다:

```bash
# 환경 파일 설정
./scripts/deploy.sh setup

# 모든 서비스 시작
./scripts/deploy.sh start

# 모든 서비스 중지
./scripts/deploy.sh stop

# 서비스 재시작
./scripts/deploy.sh restart

# 서비스 상태 표시
./scripts/deploy.sh status

# 로그 표시 (모든 서비스)
./scripts/deploy.sh logs

# 특정 서비스 로그 표시
./scripts/deploy.sh logs pg-metako

# pgAdmin과 함께 시작
./scripts/deploy.sh admin

# 모든 컨테이너와 볼륨 정리
./scripts/deploy.sh cleanup
```

### 데이터 지속성

Docker 볼륨이 데이터 지속성을 보장합니다:
- `postgres_master_data`: 마스터 데이터베이스 데이터
- `postgres_slave1_data`: 슬레이브 1 데이터베이스 데이터
- `postgres_slave2_data`: 슬레이브 2 데이터베이스 데이터
- `pg_metako_logs`: 애플리케이션 로그
- `pgadmin_data`: pgAdmin 구성

### 서비스 접근

배포 후 서비스는 다음에서 사용할 수 있습니다:
- **PostgreSQL 마스터**: `localhost:5432`
- **PostgreSQL 슬레이브 1**: `localhost:5433`
- **PostgreSQL 슬레이브 2**: `localhost:5434`
- **pg-metako 애플리케이션**: `localhost:8080`
- **pgAdmin** (활성화된 경우): `http://localhost:8081`

### Docker 문제 해결

**서비스가 시작되지 않는 경우**:
```bash
# Docker가 실행 중인지 확인
docker info

# 서비스 로그 확인
./scripts/deploy.sh logs

# 서비스 재시작
./scripts/deploy.sh restart
```

**데이터베이스 연결 문제**:
```bash
# PostgreSQL 마스터 상태 확인
docker-compose exec postgres-master pg_isready -U postgres

# 복제 상태 확인
docker-compose exec postgres-master psql -U postgres -c "SELECT * FROM pg_stat_replication;"
```

**모든 것 재설정**:
```bash
# 정리하고 새로 시작
./scripts/deploy.sh cleanup
./scripts/deploy.sh start
```

## 사용법

### 명령줄 옵션

```bash
./bin/pg-metako [옵션]

옵션:
  --config string    구성 파일 경로 (기본값 "configs/example.yaml")
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

애플리케이션은 포괄적인 로깅을 제공합니다:

```
2025/08/03 15:46:51 configs/example.yaml에서 구성 로딩 중
2025/08/03 15:46:51 3개 노드로 구성이 성공적으로 로드됨
2025/08/03 15:46:51 노드 master-1 (master)을 클러스터에 추가함
2025/08/03 15:46:51 노드 slave-1 (slave)을 클러스터에 추가함
2025/08/03 15:46:51 상태 모니터링 시작됨
2025/08/03 15:46:51 페일오버 모니터링 시작됨
2025/08/03 15:46:51 pg-metako가 성공적으로 시작됨
```

주기적인 상태 보고서에는 다음이 포함됩니다:
- 현재 마스터 노드
- 모든 노드의 상태
- 쿼리 통계 (총계, 읽기, 쓰기, 실패)
- 전체 클러스터 상태

## 개발

### 프로젝트 구조

이 프로젝트는 [golang-standards/project-layout](https://github.com/golang-standards/project-layout)을 따릅니다:

```
├── cmd/pg-metako/          # 메인 애플리케이션
├── internal/               # 비공개 애플리케이션 패키지
│   ├── config/            # 구성 관리
│   ├── database/          # 데이터베이스 연결 처리
│   ├── health/            # 상태 확인
│   ├── metako/            # 메인 애플리케이션 오케스트레이션
│   ├── replication/       # 복제 및 페일오버
│   └── routing/           # 쿼리 라우팅 및 로드 밸런싱
├── configs/               # 구성 예제
├── docs/                  # 문서
├── bin/                   # 컴파일된 바이너리
├── go.mod                 # Go 모듈 정의
├── go.sum                 # Go 모듈 체크섬
└── README.md              # 프로젝트 문서
```

### Makefile

이 프로젝트는 빌드 자동화를 위한 포괄적인 Makefile을 포함합니다. 사용 가능한 모든 대상을 확인하려면:

```bash
make help
```

#### 일반적인 명령

**빌드 및 테스트:**
```bash
make build          # 애플리케이션 빌드
make test           # 모든 테스트 실행
make test-coverage  # 커버리지 보고서와 함께 테스트 실행
make check          # 포맷, vet, 테스트 실행
```

**개발:**
```bash
make setup          # 개발 환경 설정
make dev-setup      # 추가 개발 도구와 함께 설정
make fmt            # Go 코드 포맷
make vet            # go vet 실행
make lint           # 린터 실행 (golangci-lint 필요)
```

**Docker:**
```bash
make docker-build   # Docker 이미지 빌드
make docker-run     # Docker 컨테이너 실행
make docker-compose-up    # Docker Compose로 서비스 시작
make docker-compose-down  # Docker Compose로 서비스 중지
```

**크로스 플랫폼 빌드:**
```bash
make build-all      # 모든 플랫폼용 빌드
make build-linux    # Linux용 빌드
make build-windows  # Windows용 빌드
make build-darwin   # macOS용 빌드
```

**릴리스 및 정리:**
```bash
make release        # 릴리스 빌드 생성
make clean          # 빌드 아티팩트 정리
make ci             # CI 파이프라인 실행
```

### 테스트

모든 테스트 실행:
```bash
# Makefile 사용 (권장)
make test

# 또는 Go 직접 사용
go test ./...
```

상세한 출력으로 테스트 실행:
```bash
# Makefile 사용
make test-verbose

# 또는 Go 직접 사용
go test -v ./...
```

특정 패키지에 대한 테스트 실행:
```bash
go test -v ./internal/config
```

### 개발 원칙

이 프로젝트는 테스트 주도 개발(TDD)과 Kent Beck의 "Tidy First" 원칙을 따릅니다:
- Red-Green-Refactor 사이클
- 포괄적인 테스트 커버리지
- 깨끗하고 읽기 쉬운 코드
- 관심사의 분리

## PostgreSQL 설정

### 마스터 구성

`postgresql.conf`에 추가:
```
wal_level = replica
max_wal_senders = 3
wal_keep_segments = 64
```

`pg_hba.conf`에 추가:
```
host replication replicator 0.0.0.0/0 md5
```

### 슬레이브 구성

`recovery.conf` 생성:
```
standby_mode = 'on'
primary_conninfo = 'host=master_ip port=5432 user=replicator'
```

## 문제 해결

### 일반적인 문제

1. **연결 거부됨**: PostgreSQL이 실행 중이고 접근 가능한지 확인
2. **인증 실패**: 구성에서 사용자명/비밀번호 확인
3. **정상 슬레이브 없음**: 슬레이브 노드 연결성 및 복제 상태 확인
4. **페일오버가 작동하지 않음**: 실패 임계값 및 상태 확인 설정 확인

### 로그

모든 작업은 타임스탬프와 컨텍스트와 함께 로그됩니다:
- 구성 로딩 및 검증
- 노드 상태 변경
- 페일오버 이벤트 및 슬레이브 승격
- 쿼리 라우팅 결정
- 연결 통계

## 기여

1. 저장소 포크
2. 기능 브랜치 생성
3. 새 기능에 대한 테스트 작성
4. TDD 원칙에 따라 기능 구현
5. 모든 테스트가 통과하는지 확인
6. 풀 리퀘스트 제출

## 라이선스

이 프로젝트는 MIT 라이선스 하에 라이선스됩니다 - 자세한 내용은 [LICENSE](LICENSE) 파일을 참조하세요.
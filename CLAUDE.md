# hunsu

HTML 로 디자인하는 로컬 디자인 도구. 결정과 측정값은 `PLAN.md` 가 기준이다. 기획과 다르게 구현하려면 `PLAN.md` 부터 고친다.

## 명령

```bash
go build -o hunsu .   # 실행파일은 ./hunsu
go test ./...
node --test web/
```

## 브랜치·커밋

- main 에서 직접 작업하지 않는다. 작업마다 main 에서 브랜치를 만든다: `feat/…`, `fix/…`, `docs/…`.
- 모든 커밋은 그 자체로 동작한다. 빌드와 위 테스트가 모두 통과할 때만 커밋하고, 깨진 중간 상태는 커밋하지 않는다.

## 버전

Semantic Versioning 을 따른다. main 에 병합하면 병합 커밋에 annotated 태그 `vMAJOR.MINOR.PATCH` 를 단다. 메시지는 그 병합의 요약 한 줄이다. `hunsu -version` 은 Go 가 이 태그로 넣은 값을 보인다.

사용자가 맞닿는 면을 기준으로 올린다: 실행 인자, MCP 도구 이름·인자, 디자인 폴더와 `.hunsu/` 형식, 사용자 설정 폴더의 파일, MCP 등록 주소.

- MAJOR: 그 면이 바뀌어 기존 폴더·설정·등록이 그대로 동작하지 않는다.
- MINOR: 기능을 더했다. 기존 것은 그대로 동작한다.
- PATCH: 결함을 고쳤거나 동작 약속은 그대로인 개선이다.
- 문서만 바꾼 병합에는 달지 않는다.

태그를 달 때 `.claude-plugin/plugin.json` 의 `version` 도 같은 값으로 올린다. 올리지 않으면 플러그인 사용자에게 업데이트가 가지 않는다.

태그는 main 을 push 할 때 함께 올린다: `git push --follow-tags origin main`.

## 설계

**SRP**

- 타입·함수·파일은 바뀌는 이유가 하나다. 책임을 "그리고" 없이 말할 수 없으면 나눈다.
- HTTP 핸들러는 요청 해석과 응답만 한다.
- 패키지는 책임 단위로 나눈다. `utils`, `common`, `helpers` 는 쓰지 않는다.

**OCP**

- 기능은 기존 코드를 고치지 않고 더해서 넣는다.
- 변하는 지점만 쓰는 쪽에 작은 인터페이스로 둔다: LLM provider(`llm.Provider`), 진행 이벤트 해석, 시계·ID. 근거 없는 인터페이스는 만들지 않는다.
- 같은 분기를 여러 곳에 복제하지 않는다.
- 핵심 패키지는 특정 LLM·에이전트를 전제하지 않는다. provider 패키지(`internal/claude`)는 `main.go` 에서만 참조하고, 모델 이름 같은 고유 값은 provider 안에 둔다. MCP 도 특정 클라이언트를 전제하지 않는다.

**TDD**

- 실패하는 테스트 없이 제품 코드를 쓰지 않는다. 버그도 재현 테스트부터.
- 순서: 테스트 → 기대한 이유로 실패 확인 → 최소 구현 → 리팩터링.
- 실제 LLM 은 부르지 않는다. 핵심 패키지는 `llm.Provider` 대역, claude provider 는 testdata 의 가짜 실행 파일.
- DOM 코드는 얇게, 판단 로직은 순수 함수로 빼서 `node --test`.

## 테스트

테스트는 기능 설명서다. 이름만 읽어도 무엇을 하고, 할 수 있고, 할 수 없는지 알 수 있어야 한다.

- 대상 함수당 테이블 하나. 행 이름은 동작을 서술하는 한국어 문장. 할 수 없는 것도 행으로 둔다.
- 행 하나에 규칙 하나. 같은 규칙을 여러 행·계층에서 반복하지 않는다.
- 행에는 입력과 기대값만. 준비·검증은 루프에 한 번, 공통 준비는 `t.Helper()` 함수로.
- 결과는 한 번에 비교한다 (`reflect.DeepEqual`, `errors.Is`).
- 공개 동작만, 프로젝트 코드만 검증한다. 표준 라이브러리·CodeMirror·브라우저·LLM 의 동작은 검증하지 않는다. 외부 도구가 궁금하면 스파이크로 확인하고 `PLAN.md` 에 남긴다.
- given/when/then 주석은 달지 않는다.

```go
tests := []struct {
	name    string
	path    string
	wantErr error
}{
	{"프로젝트 안의 파일 경로를 허용한다", "landing.html", nil},
	{"폴더 밖 경로는 거부한다", "../secret.txt", ErrOutsideProject},
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		if _, err := p.Resolve(tt.path); !errors.Is(err, tt.wantErr) {
			t.Errorf("Resolve(%q) error = %v, want %v", tt.path, err, tt.wantErr)
		}
	})
}
```

JS 는 `table(t, rows, check)` 로 같은 형태를 쓴다.

## 주석

- 코드로 설명할 수 있으면 이름·타입·함수로 설명한다.
- 주석은 코드로 알 수 없는 이유만: 외부 도구의 의외의 동작, 의도한 제약, 알려진 한계.
- 주석 처리된 코드와 TODO 는 남기지 않는다. 할 일은 `PLAN.md` 마일스톤에.

## 의존성

- Go 는 표준 라이브러리만 (테스트 포함).
- 프론트엔드는 빌드 없는 vanilla JS. 외부 라이브러리는 CodeMirror 5 만.

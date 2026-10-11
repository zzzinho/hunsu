# hunsu

HTML로 디자인하는 로컬 디자인 도구. 기획·결정·스파이크 결과는 `PLAN.md` 가 기준이다. 기획과 다르게 구현해야 하면 먼저 `PLAN.md` 를 고친다.

## 명령

```bash
go build -o hunsu .   # 실행파일은 항상 ./hunsu
go test ./...
node --test web/      # 프론트엔드 순수 로직
```

## 설계 원칙

### SRP (단일 책임)

- 타입·함수·파일은 바뀌는 이유가 하나여야 한다. 책임을 "그리고" 없이 한 문장으로 말할 수 없으면 나눈다.
- HTTP 핸들러는 요청 해석과 응답 작성만 한다. 판단, 파일 접근, LLM 실행은 각 책임을 가진 타입에 맡긴다.
- 패키지는 책임 단위로 나눈다 (예: 프로젝트 파일 접근, LLM provider, 세션 저장, HTTP). `utils`, `common`, `helpers` 같은 이름으로 나누지 않는다.

### OCP (개방-폐쇄)

- 기능을 추가할 때 기존 코드를 고치는 대신 새 코드를 더하는 구조로 만든다.
- 변하는 지점은 그것을 사용하는 쪽에 작은 인터페이스로 둔다. 지금 확인된 변하는 지점:
  - LLM provider (`llm.Provider`. PoC 는 Claude Code CLI, 이후 Anthropic API·Codex 등. 테스트에서는 대역)
  - 진행 이벤트 해석 (stream-json 이벤트 종류가 늘어남)
  - 시계·ID 생성 (테스트에서 고정)
- 같은 종류에 대한 분기(switch)를 여러 곳에 복제하지 않는다. 종류가 늘 때 고칠 곳이 한 군데여야 한다.
- 변할 근거가 없는 곳에 미리 인터페이스를 만들지 않는다.
- 핵심 패키지는 특정 LLM 이나 에이전트를 전제하지 않는다. provider 구현 패키지(`internal/claude`)는 조립 지점(`main.go`)에서만 참조하고, 모델 이름 같은 provider 고유 값은 provider 안에 둔다. MCP 쪽도 특정 클라이언트(Claude Code 등)를 전제하지 않는다.

### TDD

- 실패하는 테스트 없이 제품 코드를 쓰지 않는다.
- 순서: 테스트 추가 → 실행해서 기대한 이유로 실패하는지 확인 → 통과할 만큼만 구현 → 통과 상태를 유지하며 리팩터링.
- 버그를 고칠 때도 버그를 재현하는 테스트 행을 먼저 추가한다.
- 테스트는 실제 LLM 을 호출하지 않는다. 핵심 패키지는 `llm.Provider` 대역을 쓰고, claude provider 는 testdata 에 기록해 둔 stream-json 을 출력하는 가짜 실행 파일로 대신한다.
- 브라우저 DOM 을 다루는 코드는 얇게 두고, 판단 로직은 순수 함수 모듈로 빼서 `node --test` 로 테스트한다.

## 테스트 작성 규칙

테스트는 기능 설명서다. 테스트 이름만 읽어도 이 기능이 무엇을 하고, 무엇을 할 수 있고, 무엇을 할 수 없는지 알 수 있어야 한다.

- 테이블 기반으로 쓴다. 대상 함수(메서드)당 테이블 하나.
- 행 이름은 동작을 서술하는 한국어 문장이다. 할 수 있는 것과 할 수 없는 것을 모두 행으로 둔다.
  - 좋음: `"프로젝트 안의 파일 경로를 허용한다"`, `"심볼릭 링크로 폴더 밖을 가리키면 거부한다"`
  - 나쁨: `"case1"`, `"error"`, `"정상 동작"`
- 행 하나는 규칙 하나만 증명한다. 같은 규칙을 여러 행이나 여러 계층에서 반복해 증명하지 않는다.
- 행에는 입력과 기대값만 둔다. 준비·실행·검증 코드는 루프 본문에 한 번만 쓰고, 공통 준비는 `t.Helper()` 를 쓰는 함수로 뺀다.
- 결과는 한 번에 비교한다 (`reflect.DeepEqual`, `errors.Is`). 필드마다 따로 비교하지 않는다.
- 공개 동작을 검증한다. 내부 구현에 기대지 않는다.
- 프로젝트 코드만 검증한다. 표준 라이브러리, CodeMirror, 브라우저, claude CLI 가 제대로 동작하는지는 검증하지 않는다.
  - 검증한다: 우리 코드가 내리는 판단과 그 결과. 예: `Resolve` 가 폴더 밖 경로를 거부하는지, stream-json 이벤트를 어떤 진행 단계로 바꾸는지.
  - 검증하지 않는다: 라이브러리 자체의 동작. 예: `json.Marshal` 이 JSON 을 만드는지, `http.ServeMux` 가 경로를 맞게 찾는지, claude 가 Edit 를 잘 하는지.
  - 외부 도구의 동작이 궁금하면 테스트가 아니라 스파이크로 확인하고, 결과는 `PLAN.md` 에 남긴다.
- given/when/then 같은 주석을 달지 않는다. 행 이름이 설명이다.

```go
func TestProject_Resolve(t *testing.T) {
	p := newTestProject(t)
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{"프로젝트 안의 파일 경로를 허용한다", "landing.html", nil},
		{"하위 폴더의 파일 경로를 허용한다", "pages/about.html", nil},
		{"상위 폴더로 나가는 경로는 거부한다", "../secret.txt", ErrOutsideProject},
		{"절대 경로는 거부한다", "/etc/hosts", ErrOutsideProject},
		{"심볼릭 링크로 폴더 밖을 가리키면 거부한다", "link/secret.txt", ErrOutsideProject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := p.Resolve(tt.path); !errors.Is(err, tt.wantErr) {
				t.Errorf("Resolve(%q) error = %v, want %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
```

JS 도 같은 규칙을 따른다.

```js
test('stepOf', async (t) => {
  for (const [name, event, want] of [
    ['Read 도구 호출은 "파일 읽는 중"으로 표시한다', readEvent, '파일 읽는 중'],
    ['모르는 이벤트는 표시하지 않는다', { type: 'unknown' }, null],
  ]) await t.test(name, () => assert.equal(stepOf(event), want));
});
```

## 주석

- 코드로 설명할 수 있는 것은 이름, 타입, 함수 분리로 설명한다. 주석을 쓰고 싶어지면 먼저 이름을 고친다.
- 주석은 코드만 봐서는 알 수 없는 이유만 쓴다: 외부 도구의 직관에 어긋나는 동작, 의도적인 제약, 알려진 한계.
  - 좋음: `// --system-prompt-snapshot 기본값(on)이면 resume 할 때 처음 디자인 규칙이 계속 쓰인다.`
  - 나쁨: `// 파일을 읽는다`
- 주석 처리된 코드와 TODO 를 남기지 않는다. 할 일은 `PLAN.md` 마일스톤에 적는다.

## 의존성

- Go 는 표준 라이브러리만 쓴다. 테스트도 마찬가지다 (testify 등 쓰지 않음).
- 프론트엔드는 빌드 단계 없는 vanilla JS. 외부 라이브러리는 CodeMirror 5 만 파일로 포함한다.

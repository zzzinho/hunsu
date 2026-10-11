# hunsu 기획

HTML 로 디자인하는 로컬 디자인 도구. 원본을 직접 고치거나 LLM 에게 말로 고치게 한다.

- 상태: MVP(0~3) 완료. v1 로드맵 확정 (10).
- v1: 다른 사람이 설치해서 혼자 디자인 하나를 만들고 전달할 수 있다.

## 1. 목표

- 디자인 한 장 = HTML 파일 하나. 고치면 렌더 화면에 바로 반영.
- 채팅으로 요청하면 LLM 이 파일을 직접 고친다. 대화는 이어진다.
- 렌더 화면에서 요소를 클릭해 "이거" 를 지정한다.
- 모든 페이지가 디자인 시스템을 공유하고, LLM 도 따른다.
- macOS 단일 실행파일 `./hunsu`. 로컬 전용이며 서버 배포는 고려하지 않는다.

**비목표**: 서버 배포, 무한 캔버스, 컴포넌트·레이어, 디자인 시스템 전용 편집 화면, 협업·클라우드, 속성 패널, 코드 서명·공증, Windows.

## 2. 완료 기준

1. `./hunsu ~/designs` → 브라우저가 열린다.
2. "디자인 시스템 만들어줘" → `DESIGN.md`, `design.css` 생성.
3. "랜딩 페이지 만들어줘" → 진행 상황이 보이고, 디자인 시스템을 쓴 페이지가 렌더된다.
4. 버튼 선택 → "더 크게" → 그 버튼만 커진다.
5. 에디터에서 고쳐 저장 → 바로 반영.
6. `design.css` 값 변경 → 모든 페이지에 반영.
7. "아까 버튼 색 되돌려" → 맥락을 기억해 되돌린다.
8. [되돌리기] → 직전 요청 전 상태로 복원.

## 3. 결정 사항

| 항목 | 결정 | 이유 |
|---|---|---|
| 실행 형태 | CLI `./hunsu [폴더]` (기본: 현재 디렉터리) | 셸 PATH 를 물려받는다. `.app` 은 Finder 실행 시 PATH 문제 |
| 언어 | Go, 외부 의존성 0 | embed·HTTP·프로세스 실행이 표준 라이브러리. 바이너리 약 10MB |
| 프론트엔드 | 빌드 없는 vanilla JS + CodeMirror 5 | 바이너리에 그대로 포함. CodeMirror 6 은 번들러 필요 |
| 페이지 | 루트 폴더의 `.html` 만. 숨김 파일·심볼릭 링크 제외 | 모든 페이지가 `design.css` 를 같은 경로로 링크 |
| 디자인 시스템 | `DESIGN.md`(글 규칙) + `design.css`(CSS 변수·공통 클래스) | 브라우저가 그대로 적용. 변환 단계 불필요 |
| LLM provider | `llm.Provider` 뒤에 둔다. `claude -p`, `codex exec`. 고르는 곳은 `main.go` 하나 | API 등으로 추가 (5) |
| LLM 편집 범위 | 일부 수정, 페이지 재디자인, 새 페이지, 여러 페이지, 빈 프로젝트 전체 생성 | 일부 수정만으로는 디자인 도구가 안 된다 |
| LLM 편집 방식 | LLM 이 자기 편집 도구로 파일을 직접 수정 | 바뀐 부분만 쓴다. 전체 재생성은 느리다 |
| 대화 단위 | 프로젝트 하나에 대화 하나. 요청마다 현재 페이지를 알린다 | 여러 페이지 요청이 맥락을 공유 |
| 모델 | provider 가 목록과 기본값(첫 번째)을 정한다. 화면은 `/api/models` | claude provider: opus(기본)·sonnet·haiku. haiku 는 2배 빠르고 20배 싸다 (11) |
| 캔버스 | 파일 1개 미리보기 + 모바일/데스크톱 너비 | 다중 프레임은 MVP 핵심과 무관 |
| 외부 에이전트 | MCP 서버 `/mcp` | 클라이언트(Claude Code, Codex 등)에 묶이지 않는 표준 (7) |
| 배포 (v1) | GitHub Release + Homebrew tap. 서명 없음 | brew 설치는 Gatekeeper 에 막히지 않는다. Apple 개발자 등록 불필요 |
| provider 연결 (v1) | 각 CLI 의 로그인을 쓴다. hunsu 는 키를 갖지 않는다 | `claude auth status --json`, `codex login status` 로 확인하고 로그인 명령을 안내 |
| UI 언어 (v1) | 영어 기본, 설정에서 한국어 | v1 은 다른 사람이 쓴다 |
| 버저닝 (v1) | 내장 타임라인 (git 미사용) | 의존성 없음. 디자이너도 쓸 수 있다 |
| 렌더 확인 (v1) | `screenshot` MCP 도구, 시점은 LLM 이 판단 | 품질을 올리되 작은 수정의 시간·비용은 늘리지 않는다 |
| 직접 조작 (v1) | 텍스트만 렌더 화면에서 수정. 속성 패널은 하지 않는다 | 문구 수정에 LLM(7초~1분 이상)을 쓰지 않게 |

## 4. 아키텍처

```
./hunsu (127.0.0.1:7777, -port 로 변경)
 ├─ /              UI (embed.FS)
 ├─ /p/*           디자인 폴더 정적 서빙 → iframe 렌더
 ├─ /api/files·file 목록·읽기·쓰기·새 파일
 ├─ /api/chat      LLM 실행, 진행 이벤트 스트림. 연결을 끊으면 취소
 ├─ /api/models    provider 의 모델 목록
 ├─ /api/undo·chat/reset  되돌리기, 새 대화
 ├─ /api/events    열린 파일·design.css 변경 알림 (SSE)
 ├─ /api/ui·ui/commands  브라우저 화면 상태, 브라우저로 보내는 명령
 ├─ /mcp           외부 에이전트용 MCP 서버
 └─ llm.Provider   claude -p 또는 codex exec (작업 폴더 = 디자인 폴더)
```

```
designs/
  DESIGN.md, design.css   # 디자인 시스템
  *.html                  # 페이지
  assets/                 # 이미지·폰트
  .hunsu/session          # 대화 ID
  .hunsu/history/000001/  # 되돌리기 스냅샷 (최근 50개)
  .hunsu/versions/000001/ # 버전: files/, meta.json, thumb.png (최근 100개)
```

- 파일 변경은 0.5초 폴링으로 감지한다. LLM·에디터·외부 에디터 수정을 한 방법으로 잡는다.
- 저장은 임시 파일에 쓰고 이름을 바꾼다. 반쯤 쓴 파일을 읽지 않게.

## 5. LLM provider

hunsu 는 특정 LLM·에이전트를 전제하지 않는다. `claude -p`, `codex exec` 는 provider 구현일 뿐이다.

| 방향 | 통로 | 현재 | 교체·추가 |
|---|---|---|---|
| hunsu → LLM (브라우저 채팅) | `llm.Provider` | `internal/claude`, `internal/codex` | API 등. `main.go` 의 `providers` |
| 에이전트 → hunsu | MCP | 모든 MCP 클라이언트 | 등록만 하면 된다 |

**계약** (`internal/llm`)

- `Run(ctx, Request, emit)`: `Request.Dir` 의 파일을 직접 고치고, 공통 이벤트(start·think·read·write·answer·tool·retry·reply·fail)로 진행을 알린다. 대화 ID 는 `Started` 의 `SessionID` 로 알리고 다음 `Request.SessionID` 로 받는다. ctx 취소 시 멈춘다.
- `Models()`: 고를 수 있는 모델. 첫 번째가 기본. 비면 화면이 선택을 숨긴다.
- 도구: hunsu 가 LLM 에게 주는 도구는 MCP 서버(`/mcp/render`, 화면 확인 도구만)로 건넨다. provider 는 요청마다 `Request.Tools`(`llm.MCPServer`) 로 받아 붙이고, 쓰는 동안 `tool` 이벤트로 알린다. Chrome 이 없으면 건네지 않는다.
- 프롬프트 조립, 동시 실행 막기, 스냅샷, 직접 고친 파일 알림은 `internal/chat` 이 맡는다.

**추가 방법**

1. `internal/<이름>` 에 `llm.Provider` 를 구현한다. 실행·이벤트 해석·모델·격리는 이 안에 둔다.
2. `main.go` 의 `provider()` 에서 고른다.
3. 핵심 패키지는 고치지 않는다. 고쳐야 하면 `llm` 계약부터 고친다.

편집 도구가 없는 API 형 provider 는 디자인 폴더로 한정된 파일 도구와 도구 호출 루프를 provider 안에 둔다. 격리와 폴더 밖 접근 차단은 provider 가 보장한다.

## 6. 브라우저 채팅

**claude provider** (`internal/claude`)

```
claude -p [--resume <id>] --safe-mode --restricted --strict-mcp-config
  --tools "Read,Edit,Write" --permission-mode acceptEdits
  --append-system-prompt "<지시문 + 디자인 규칙>"
  --output-format stream-json --verbose --include-partial-messages [--model <id>]
```

| 플래그 | 이유 |
|---|---|
| `--safe-mode`, `--strict-mcp-config` | 사용자 설정·hook·MCP 를 섞지 않는다. 2배 빠르고 10배 싸다 |
| `--restricted`, `--tools` | 명령 실행 없이 디자인 폴더 안 파일만 다룬다 |
| `--resume` | `system/init` 의 `session_id` 로 대화를 잇는다 |
| `--include-partial-messages` | 큰 파일을 쓰는 동안(opus 80초) 진행을 보여준다 |

- 프롬프트는 표준 입력으로 넘긴다. `-` 로 시작하는 요청이 플래그로 해석되지 않게.
- `--bare` 는 OAuth 를 읽지 않아 쓰지 않는다.

**진행 표시**: 단계, 회전 표시, 경과 시간. 쓰는 중에는 1KB 마다 크기를 알린다. 이벤트가 없는 구간(opus 17초)에도 시간이 올라간다.

**실행 규칙**: 프로젝트당 하나만 실행한다. 실행 중에는 입력·에디터·되돌리기를 잠근다. 보내기 전에 에디터를 저장한다. 취소하면 1초 안에 프로세스를 끝낸다.

**프롬프트**

```
[현재 페이지] landing.html
[알림] 마지막 응답 이후 사용자가 직접 고친 파일: design.css. 고치기 전에 다시 읽어라.
[디자인 시스템]
<design.css>…</design.css>
[에셋] assets/logo.png, assets/hero.jpg
[선택한 요소] main > section.hero > button.cta
<button class="cta">시작하기</button>
[요청] 더 크게
```

- `[알림]`: 응답 직후 파일 해시를 기억하고 달라진 파일을 알린다. 되돌리기도 포함. 없으면 예전 내용으로 Edit 하다 실패해 6초가 17초가 된다.
- `[에셋]`: 올린 에셋이 있으면 요청마다 경로 목록을 붙인다.
- 선택한 요소의 HTML 은 2000자까지.
- **고정된 것은 앞, 바뀌는 것은 뒤.** 프롬프트 캐시는 앞부분이 같아야 재사용된다. system prompt 는 지시문(`internal/chat/instructions.md`)과 디자인 규칙(`internal/project/rules.md`)만 두고 바뀌지 않는다.
- `[디자인 시스템]`: 대화의 첫 요청(새 대화·서버 재시작 포함)에 전체를, 이후에는 사용자가 바꾼 파일만 붙인다. LLM 이 고친 내용은 다시 보내지 않는다.

## 7. 외부 에이전트 (MCP)

에이전트가 hunsu 에 접속해 브라우저 상태를 읽고, 파일은 자기 도구로 고친다. hunsu 는 변경을 감지해 다시 그린다.

| 항목 | 결정 | 이유 |
|---|---|---|
| 도구 | `get_context`, `checkpoint`, `undo`, `open_page`, `screenshot`(Chrome 이 있을 때) | 브라우저 상태, 되돌리기, 렌더 결과만 hunsu 의 일 |
| 전송 | 기존 HTTP 서버의 `/mcp` (Streamable HTTP, 응답 하나) | 브라우저 상태를 같은 프로세스에서 공유. stdio 는 별도 프로세스 |
| 구현 | 표준 라이브러리 JSON-RPC (`initialize`, `ping`, `tools/list`, `tools/call`) | 도구 호출만 필요 |
| 포트·인증 | 7777 고정, `Bearer` 토큰(사용자 설정 폴더, 0600, 재사용) | 등록한 주소·헤더가 바뀌지 않게. `-port` 로 바꾸면 다시 등록해야 한다 |
| 화면 상태 | 브라우저가 `POST /api/ui` 로 알린다. 같은 출처만 | 가짜 선택 요소로 에이전트에 지시를 끼워 넣는 것 차단 |
| 브라우저 명령 | `GET /api/ui/commands` (SSE) | `open_page` |
| 안내문 | 디자인 규칙은 브라우저 채팅과 공유. 클라이언트 도구 이름을 쓰지 않는다 | 클라이언트 중립 |

**렌더러**: 설치된 Chrome 계열 브라우저를 `--remote-debugging-pipe` 로 띄워 DevTools 프로토콜로 찍는다(WebSocket 불필요). 요청마다 새 프로세스, 너비 320~2560px(기본 1280), 위에서 최대 4000px, 30초 제한. 결과는 MCP 이미지로 돌려준다.

**남은 것**: `open_page` 없이 만든 페이지의 목록 갱신.

## 8. 화면

- 패널: 파일 / 렌더 / 편집기·채팅. 접기(⌘\ 는 렌더만), 경계 끌어 크기 조절(렌더 320px·편집기 120px 은 남긴다). 상태는 localStorage.
- 렌더 도구: 선택(V, 클릭 선택·링크 무시), 손(H, 끌어서 이동·3px 이하는 클릭). Esc 는 선택 해제.
- 확대/축소 25~300%. 모바일은 390px 을 그대로 확대, 데스크톱은 브라우저 확대처럼 논리 너비를 바꾼다.
- 선택자: `body` 아래 태그·클래스 2개·형제 순서. 조상에 id 가 있으면 id 부터. 새로고침 후 다시 찾아 유지, 없으면 해제.
- 외곽선·커서는 렌더된 문서에만 넣는다. 파일은 건드리지 않는다.
- 색감: "훈수" → 바둑. 먹색 바탕, 강조색은 바둑판 나무색 하나(`#e0a64b`). 디자인의 색 판단을 방해하지 않게. 토큰은 `web/app.css` 의 `:root`.

## 9. 되돌리기·보안

- 요청·checkpoint·복원 직전에 편집 대상 파일 전체를 `.hunsu/history/<순번>/` 에 복사한다. 되돌리면 그 뒤에 생긴 파일은 지운다. 최근 50개만 남긴다. 버전은 마일스톤 10.
- 파일 API 는 편집 대상 이름 규칙 + 일반 파일만 + `os.Root` 로 폴더 밖을 막는다.
- 서버는 `127.0.0.1` 만 받는다. `/mcp` 는 토큰, `/api/ui` 는 출처 검사.
- 계획(마일스톤 4): 나머지 API 에 토큰·Host·Origin 검사 (CSRF, DNS rebinding).
- 한계: iframe 이 같은 출처라 디자인 JS 가 API 를 부를 수 있다. 외부 HTML 을 들일 때 포트를 분리한다.

## 10. 마일스톤

**완료**

- [x] 0. 스파이크 (11)
- [x] 1. 서버·렌더·소스 편집
- [x] 2. LLM 채팅: 진행 표시, 대화 이어가기, 디자인 시스템, 수동 편집 알림, 모델 선택, 취소, 되돌리기
- [x] 3. 요소 선택: 선택·손 도구, 새로고침 후 유지
- [x] LLM provider 분리 (5), 외부 에이전트 MCP (7), 프롬프트 캐시 배치 (6)

**v1 기능** (먼저)

- [x] 6. 에셋: `assets/`(유일한 하위 폴더)에 이미지(png·jpg·gif·webp·avif·svg)·폰트(woff·woff2·ttf·otf)를 끌어다 놓거나 골라서 올린다. 파일당 10MB, 같은 이름은 거부. 목록을 요청마다 프롬프트 끝과 `get_context` 에. LLM 은 에셋을 참조만 하고 만들지 않으므로 되돌리기 스냅샷에 넣지 않는다. 지우기는 Finder 에서
- [x] 7. 텍스트 직접 수정: 선택 도구로 텍스트만 가진 요소를 더블클릭해 고치고, Enter·바깥 클릭으로 반영, Esc 로 취소. 원본에서 같은 문구 중 몇 번째인지로 위치를 찾고, 찾지 못하면(페이지 JS 가 만든 문구 등) 반영하지 않고 알린다. 기존 파일 저장 API 를 쓴다. 편집기 저장처럼 되돌리기 스냅샷은 없다
- [x] 8. 렌더러: headless Chrome 스크린샷. `screenshot` MCP 도구를 내장 provider 와 외부 에이전트가 함께 쓴다. 확인 시점은 LLM 이 판단(새 페이지·큰 변경 뒤). Chrome 이 없으면 기능을 끈다
- [x] 9. 내보내기: 폴더 ZIP(디자인 시스템·페이지·에셋, `.hunsu/` 제외), 페이지별 단일 HTML(`design.css` 링크를 `<style>` 로, 에셋 경로를 data URI 로), PNG(렌더러, 높이 16,000px 까지). 화면 막대의 [내보내기] 메뉴에서 받는다. Chrome 이 없으면 PNG 는 이유를 알린다
- [x] 10. 버저닝: 버전은 디자인 파일 전체의 저장본(시각·이름·대표 페이지·썸네일). 요청이 끝나면 요청 내용을 이름으로, 요청 전에 사용자가 고친 내용이 있으면 '요청 전' 으로, [버전 저장] 으로 남긴다. 직전 버전과 내용이 같으면 남기지 않는다. 아무 버전으로 복원하고, 복원 전 상태와 복원 결과도 버전으로 남는다. [되돌리기] 는 지금처럼 요청·checkpoint·복원 직전으로 돌아간다. 썸네일은 대표 페이지(새로 만든 페이지 → 보고 있던 페이지 → 첫 페이지)를 1280×800 으로 찍고, Chrome 이 없으면 없다. 에셋은 넣지 않는다. 버전은 최근 100개, 되돌리기 스냅샷은 최근 50개만 남긴다. MCP `undo` 설명을 맞춘다
- [x] 11. Codex provider: `codex exec --json`(이어가기는 `exec resume`). 지시문은 `developer_instructions`, 도구는 `mcp_servers.hunsu`(자동 승인), 샌드박스는 `workspace-write`, 사용자 config·rules·프로젝트 AGENTS.md 는 끈다. 모델은 gpt-5.6-sol(기본)·terra·luna. provider 는 실행 인자 `-provider claude|codex` 로 고르고, 생략하면 설치된 것을 claude → codex 순으로 찾는다. 대화 ID 는 provider 별 파일(`.hunsu/session-<이름>`)
- [x] 12. 비교안(옵션): 입력창에서 [안 2개·3개] 를 고르면 디자인 파일을 안마다 `.hunsu/variants/<n>/` 에 복사하고 provider 를 병렬로 실행한다(새 대화, 안마다 '충실하게·과감하게·절제해서' 방향). 비교안 실행에는 도구를 주지 않는다(`screenshot` 은 원본 폴더를 찍는다). 그래서 도구는 provider 생성이 아니라 요청(`llm.Request.Tools`)으로 건넨다. 나란히 보고 고르면 그 안이 지금 상태가 되고, 나머지 안도 썸네일과 함께 버전에 남는다. [되돌리기] 로 고르기 전으로 돌아간다

**v1 마무리** (기능 뒤)

- [x] 5. 영어 UI: 화면 문구는 `web/i18n.mjs` 사전(영어 기본, 한국어). 파일 패널 위 언어 선택으로 고르고 브라우저에 기억한다. 서버의 오류·터미널 출력·프롬프트 라벨·지시문·MCP 설명은 영어. 화면에 보이는 서버 오류는 `{"code","message"}` 로 받아 화면이 코드로 번역한다. LLM 은 사용자가 쓴 언어로 답한다. 모델은 provider 가 이름과 빠름 여부(`Model.Fast`)만 알리고 문구는 화면이 붙인다. 버전 이름은 종류(`kind`)·요청 내용·번호로 남기고 화면이 문구로 바꾼다
- [ ] 4. 보안·설치: 모든 요청에 Host 검사(호스트 이름이 127.0.0.1·localhost 가 아니면 거부, DNS rebinding)와 Origin 검사(있는데 자기 출처가 아니면 거부, CSRF. 없으면 브라우저가 아닌 로컬 프로그램으로 보고 받는다). `/p/*`·`/variant/*` 는 디자인 파일(페이지·디자인 시스템·에셋)만 주고 `.hunsu/`·디렉터리 목록은 주지 않는다. provider 계약에 `Status()`(설치·로그인, 고칠 명령), 화면과 터미널에 알린다. MCP `initialize` 는 아는 버전이면 그대로, 모르면 최신 버전으로 답한다. 배포는 태그를 push 하면 GitHub Actions 가 universal 바이너리를 Release 에 올리고 Homebrew tap(`zzzinho/homebrew-hunsu`) formula 를 갱신한다. tap 저장소와 토큰 준비는 사용자가 한다

## 11. 측정값 (2026-10-09, claude 2.1.295)

**스파이크**

| 질문 | 결과 |
|---|---|
| 진행을 실시간으로 받나 | 된다. 첫 이벤트 0.7초, 첫 도구 2.0초. 답 글자는 마지막에 온다 |
| 손으로 고친 파일을 이어 편집하나 | 된다. 수동 편집 보존. 다만 Edit 실패 후 재읽기로 17초 |
| 폴더 밖 접근이 막히나 | 막힌다. 상대·절대 경로, 심볼릭 링크 모두 |
| 디자인 규칙을 따르나 | 따른다. 새 요소에 `var(--...)` 12회, `design.css` 링크 자동 추가 |
| 규칙 변경이 이어진 대화에 반영되나 | system prompt 에 두면 기본값으로는 안 된다. 메시지 끝에 붙이면 된다 |

**작은 수정** ("버튼 배경을 초록색으로")

| 설정 | 완료 | 비용 |
|---|---|---|
| 격리 + opus | 7.3초 | $0.022 |
| 격리 + sonnet | 7.6초 | $0.015 |
| 격리 + haiku | 3.8초 | $0.0009 |
| 격리 안 함 + opus | 15.8초 | $0.210 |

`--effort low` 는 효과 없음. 프로세스 시작은 1초 미만.

**새 페이지** ("카페 소개 랜딩 페이지", 빈 프로젝트)

| 모델 | 파일 쓰기 | 전체 | 비용 |
|---|---|---|---|
| opus | 80초 | 102초 | $0.29 |
| haiku | 18초 | 25~40초 | - |

빈 프로젝트에서는 없는 파일을 읽으려다 4초를 쓴다. 프롬프트에 파일 목록을 넣으면 줄일 수 있다.

**프롬프트 캐시** (haiku, 대화 기록 약 1만 토큰, `design.css` 한 글자 변경)

| 디자인 시스템 위치 | 캐시 재사용 | 새로 저장 |
|---|---|---|
| system prompt 안 (이전) | 2,149 | 8,770 |
| 요청 메시지 끝 (현재) | 10,861 | 1,353 |

캐시 저장 1.25배, 재사용 0.1배 단가 기준 약 4배 차이. 대화가 길수록 커진다. 바뀐 값도 정확히 반영한다.

**렌더러** (2026-10-12, claude 2.1.296)

| 질문 | 결과 |
|---|---|
| Chrome 시작부터 전체 페이지 PNG 까지 | 0.5~0.9초 |
| `--safe-mode` 에서 `--mcp-config` 가 되나 | 안 된다. 넘긴 서버까지 꺼진다(`mcp_servers: []`) |
| `--safe-mode` 없이 `--restricted --strict-mcp-config` 로 격리되나 | 된다. 사용자·부모 폴더 CLAUDE.md, 설정, 사용자 플러그인이 섞이지 않는다. 입력 4,476 토큰(safe-mode 4,520) |
| 도구 설명의 비용 | `screenshot` 하나 +254 토큰, `/mcp` 전체(도구 4개·안내문) +1,750 토큰 |
| LLM 이 스스로 확인하나 | haiku 가 "모바일에서 확인하고 고쳐" 에 `screenshot` 을 불렀다. 10초 |

**Codex provider** (2026-10-12, codex-cli 0.157.1, ChatGPT 로그인)

| 질문 | 결과 |
|---|---|
| 작은 수정 (h1 문구, gpt-5.6-luna) | 14초. 이어서 묻기 4초 |
| 기본 입력 | 약 18,700 토큰(claude 격리 4,500). 기능(apps·browser_use 등)을 꺼도 2,300 토큰만 준다. 기능 이름이 버전마다 바뀌어 끄지 않는다 |
| 사용자 지침이 섞이나 | `--ignore-user-config` 로도 전역 `~/.codex/AGENTS.md` 는 들어간다. 끌 설정이 없다. 프로젝트 AGENTS.md 는 `project_doc_max_bytes=0` 으로 끈다 |
| MCP 도구 | 기본은 승인이 필요해 exec 에서 거부된다. `default_tools_approval_mode="approve"` 로 된다. 이미지 결과를 읽는다 |
| 실패 | `turn.failed` 를 보내고 종료 코드는 0 이다 |
| 모델 | gpt-5.6-sol·terra·luna, gpt-5.5 모두 된다. 짧은 답 5~7초 |

**답변 언어** (2026-10-12): 한국어 디자인 파일에 영어로 요청하면 opus 는 영어로, haiku 는 한국어로 답할 때가 있다. 빈 폴더에서는 둘 다 요청의 언어로 답한다.

**MCP** (클라이언트: Claude Code, haiku): 선택 요소 색 변경 + 새 페이지 + 열기 14.7초. 도구 순서 `get_context` → `checkpoint` → 편집 → `open_page`. 브라우저 [되돌리기] 한 번으로 모두 복원. 토큰 없는 `/mcp` 401, 다른 출처 `/api/ui` 403.

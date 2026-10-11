# hunsu 기획

HTML 로 디자인하는 로컬 디자인 도구. 원본을 직접 고치거나 LLM 에게 말로 고치게 한다.

- 상태: 마일스톤 0~3, LLM provider 분리, 외부 에이전트(MCP) 완료. 남은 것은 마일스톤 4.

## 1. 목표

- 디자인 한 장 = HTML 파일 하나. 고치면 렌더 화면에 바로 반영.
- 채팅으로 요청하면 LLM 이 파일을 직접 고친다. 대화는 이어진다.
- 렌더 화면에서 요소를 클릭해 "이거" 를 지정한다.
- 모든 페이지가 디자인 시스템을 공유하고, LLM 도 따른다.
- macOS 단일 실행파일 `./hunsu`.

**비목표**: 무한 캔버스·다중 프레임, 컴포넌트·레이어, 디자인 시스템 전용 편집 화면, 협업·클라우드, vision, 내보내기, 서명·공증·Homebrew.

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
| LLM provider | `llm.Provider` 뒤에 둔다. PoC 는 `claude -p`. 고르는 곳은 `main.go` 하나 | API·Codex 등으로 교체·추가 (5) |
| LLM 편집 범위 | 일부 수정, 페이지 재디자인, 새 페이지, 여러 페이지, 빈 프로젝트 전체 생성 | 일부 수정만으로는 디자인 도구가 안 된다 |
| LLM 편집 방식 | LLM 이 자기 편집 도구로 파일을 직접 수정 | 바뀐 부분만 쓴다. 전체 재생성은 느리다 |
| 대화 단위 | 프로젝트 하나에 대화 하나. 요청마다 현재 페이지를 알린다 | 여러 페이지 요청이 맥락을 공유 |
| 모델 | provider 가 목록과 기본값(첫 번째)을 정한다. 화면은 `/api/models` | claude provider: opus(기본)·sonnet·haiku. haiku 는 2배 빠르고 20배 싸다 (11) |
| 캔버스 | 파일 1개 미리보기 + 모바일/데스크톱 너비 | 다중 프레임은 MVP 핵심과 무관 |
| 외부 에이전트 | MCP 서버 `/mcp` | 클라이언트(Claude Code, Codex 등)에 묶이지 않는 표준 (7) |
| 배포 | 본인·소수 테스트. 서명 없음 | 받은 사람은 `xattr -d com.apple.quarantine hunsu` |

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
 └─ llm.Provider   PoC: claude -p (작업 폴더 = 디자인 폴더)
```

```
designs/
  DESIGN.md, design.css   # 디자인 시스템
  *.html                  # 페이지
  .hunsu/session          # 대화 ID
  .hunsu/history/000001/  # 요청 직전 스냅샷
```

- 파일 변경은 0.5초 폴링으로 감지한다. LLM·에디터·외부 에디터 수정을 한 방법으로 잡는다.
- 저장은 임시 파일에 쓰고 이름을 바꾼다. 반쯤 쓴 파일을 읽지 않게.

## 5. LLM provider

hunsu 는 특정 LLM·에이전트를 전제하지 않는다. `claude -p` 는 PoC 구현일 뿐이다.

| 방향 | 통로 | 현재 | 교체·추가 |
|---|---|---|---|
| hunsu → LLM (브라우저 채팅) | `llm.Provider` | `internal/claude` | API, Codex 등. `main.go` 의 `provider()` |
| 에이전트 → hunsu | MCP | 모든 MCP 클라이언트 | 등록만 하면 된다 |

**계약** (`internal/llm`)

- `Run(ctx, Request, emit)`: `Request.Dir` 의 파일을 직접 고치고, 공통 이벤트(start·think·read·write·answer·retry·reply·fail)로 진행을 알린다. 대화 ID 는 `Started` 의 `SessionID` 로 알리고 다음 `Request.SessionID` 로 받는다. ctx 취소 시 멈춘다.
- `Models()`: 고를 수 있는 모델. 첫 번째가 기본. 비면 화면이 선택을 숨긴다.
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
  --append-system-prompt "<지시문 + 디자인 규칙 + DESIGN.md + design.css>"
  --system-prompt-snapshot off
  --output-format stream-json --verbose --include-partial-messages [--model <id>]
```

| 플래그 | 이유 |
|---|---|
| `--safe-mode`, `--strict-mcp-config` | 사용자 설정·hook·MCP 를 섞지 않는다. 2배 빠르고 10배 싸다 |
| `--restricted`, `--tools` | 명령 실행 없이 디자인 폴더 안 파일만 다룬다 |
| `--resume` | `system/init` 의 `session_id` 로 대화를 잇는다 |
| `--system-prompt-snapshot off` | 기본값이면 resume 때 첫 디자인 규칙이 계속 쓰인다 |
| `--include-partial-messages` | 큰 파일을 쓰는 동안(opus 80초) 진행을 보여준다 |

- 프롬프트는 표준 입력으로 넘긴다. `-` 로 시작하는 요청이 플래그로 해석되지 않게.
- `--bare` 는 OAuth 를 읽지 않아 쓰지 않는다.

**진행 표시**: 단계, 회전 표시, 경과 시간. 쓰는 중에는 1KB 마다 크기를 알린다. 이벤트가 없는 구간(opus 17초)에도 시간이 올라간다.

**실행 규칙**: 프로젝트당 하나만 실행한다. 실행 중에는 입력·에디터·되돌리기를 잠근다. 보내기 전에 에디터를 저장한다. 취소하면 1초 안에 프로세스를 끝낸다.

**프롬프트**

```
[현재 페이지] landing.html
[알림] 마지막 응답 이후 사용자가 직접 고친 파일: landing.html. 고치기 전에 다시 읽어라.
[선택한 요소] main > section.hero > button.cta
<button class="cta">시작하기</button>
[요청] 더 크게
```

- `[알림]`: 응답 직후 파일 해시를 기억하고 달라진 파일을 알린다. 되돌리기도 포함. 없으면 예전 내용으로 Edit 하다 실패해 6초가 17초가 된다.
- 선택한 요소의 HTML 은 2000자까지.
- 디자인 시스템은 매 요청 system prompt 로 넣는다. 지시문은 `internal/chat/instructions.md`, 디자인 규칙은 `internal/project/rules.md`.

## 7. 외부 에이전트 (MCP)

에이전트가 hunsu 에 접속해 브라우저 상태를 읽고, 파일은 자기 도구로 고친다. hunsu 는 변경을 감지해 다시 그린다.

| 항목 | 결정 | 이유 |
|---|---|---|
| 도구 | `get_context`, `checkpoint`, `undo`, `open_page` | 브라우저 상태와 되돌리기 연결만 hunsu 의 일 |
| 전송 | 기존 HTTP 서버의 `/mcp` (Streamable HTTP, 응답 하나) | 브라우저 상태를 같은 프로세스에서 공유. stdio 는 별도 프로세스 |
| 구현 | 표준 라이브러리 JSON-RPC (`initialize`, `ping`, `tools/list`, `tools/call`) | 도구 호출만 필요 |
| 포트·인증 | 7777 고정, `Bearer` 토큰(사용자 설정 폴더, 0600, 재사용) | 등록한 주소·헤더가 바뀌지 않게 |
| 화면 상태 | 브라우저가 `POST /api/ui` 로 알린다. 같은 출처만 | 가짜 선택 요소로 에이전트에 지시를 끼워 넣는 것 차단 |
| 브라우저 명령 | `GET /api/ui/commands` (SSE) | `open_page` |
| 안내문 | 디자인 규칙은 브라우저 채팅과 공유. 클라이언트 도구 이름을 쓰지 않는다 | 클라이언트 중립 |

**남은 것**: `screenshot` 도구(headless Chrome 필요), `open_page` 없이 만든 페이지의 목록 갱신.

## 8. 화면

- 패널: 파일 / 렌더 / 편집기·채팅. 접기(⌘\ 는 렌더만), 경계 끌어 크기 조절(렌더 320px·편집기 120px 은 남긴다). 상태는 localStorage.
- 렌더 도구: 선택(V, 클릭 선택·링크 무시), 손(H, 끌어서 이동·3px 이하는 클릭). Esc 는 선택 해제.
- 확대/축소 25~300%. 모바일은 390px 을 그대로 확대, 데스크톱은 브라우저 확대처럼 논리 너비를 바꾼다.
- 선택자: `body` 아래 태그·클래스 2개·형제 순서. 조상에 id 가 있으면 id 부터. 새로고침 후 다시 찾아 유지, 없으면 해제.
- 외곽선·커서는 렌더된 문서에만 넣는다. 파일은 건드리지 않는다.
- 색감: "훈수" → 바둑. 먹색 바탕, 강조색은 바둑판 나무색 하나(`#e0a64b`). 디자인의 색 판단을 방해하지 않게. 토큰은 `web/app.css` 의 `:root`.

## 9. 되돌리기·보안

- 요청 직전에 편집 대상 파일 전체를 `.hunsu/history/<순번>/` 에 복사한다. 되돌리면 그 뒤에 생긴 파일은 지운다. 정리는 MVP 이후.
- 파일 API 는 편집 대상 이름 규칙 + 일반 파일만 + `os.Root` 로 폴더 밖을 막는다.
- 서버는 `127.0.0.1` 만 받는다. `/mcp` 는 토큰, `/api/ui` 는 출처 검사.
- 계획(마일스톤 4): 나머지 API 에 토큰·Host·Origin 검사 (CSRF, DNS rebinding).
- 한계: iframe 이 같은 출처라 디자인 JS 가 API 를 부를 수 있다. 외부 HTML 을 들일 때 포트를 분리한다.

## 10. 마일스톤

- [x] 0. 스파이크 (11)
- [x] 1. 서버·렌더·소스 편집
- [x] 2. LLM 채팅: 진행 표시, 대화 이어가기, 디자인 시스템 주입, 수동 편집 알림, 모델 선택, 취소, 되돌리기
- [x] 3. 요소 선택: 선택·손 도구, 새로고침 후 유지
- [x] LLM provider 분리 (5)
- [x] 외부 에이전트 MCP (7)
- [ ] 4. 보안·패키징: 나머지 API 보호, `claude` 설치 확인, universal 바이너리(`lipo`)

## 11. 측정값 (2026-10-09, claude 2.1.295)

**스파이크**

| 질문 | 결과 |
|---|---|
| 진행을 실시간으로 받나 | 된다. 첫 이벤트 0.7초, 첫 도구 2.0초. 답 글자는 마지막에 온다 |
| 손으로 고친 파일을 이어 편집하나 | 된다. 수동 편집 보존. 다만 Edit 실패 후 재읽기로 17초 |
| 폴더 밖 접근이 막히나 | 막힌다. 상대·절대 경로, 심볼릭 링크 모두 |
| 디자인 규칙을 따르나 | 따른다. 새 요소에 `var(--...)` 12회, `design.css` 링크 자동 추가 |
| 규칙 변경이 이어진 대화에 반영되나 | 기본값은 안 된다. `--system-prompt-snapshot off` 면 된다 |

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

**MCP** (클라이언트: Claude Code, haiku): 선택 요소 색 변경 + 새 페이지 + 열기 14.7초. 도구 순서 `get_context` → `checkpoint` → 편집 → `open_page`. 브라우저 [되돌리기] 한 번으로 모두 복원. 토큰 없는 `/mcp` 401, 다른 출처 `/api/ui` 403.

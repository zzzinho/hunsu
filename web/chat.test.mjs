import { test } from 'node:test';
import assert from 'node:assert/strict';
import { elapsedText, parseSSE, stepText } from './chat.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

test('parseSSE', (t) => table(t, [
  ['완성된 이벤트를 JSON 으로 읽는다', 'data: {"kind":"start"}\n\n', [{ kind: 'start' }], ''],
  ['여러 이벤트를 한 번에 읽는다', 'data: {"kind":"start"}\n\ndata: {"kind":"end"}\n\n', [{ kind: 'start' }, { kind: 'end' }], ''],
  ['끝나지 않은 이벤트는 다음 조각을 위해 남긴다', 'data: {"kind":"start"}\n\ndata: {"ki', [{ kind: 'start' }], 'data: {"ki'],
], (text, events, rest) => assert.deepEqual(parseSSE(text), { events, rest })));

test('stepText', (t) => table(t, [
  ['시작하면 시작했다고 보인다', { kind: 'start' }, '시작했습니다'],
  ['생각을 시작하면 생각하는 중으로 보인다', { kind: 'think' }, '생각하는 중'],
  ['파일 이름을 모르면 파일을 연다고만 보인다', { kind: 'read' }, '파일 여는 중'],
  ['파일 읽기는 파일 이름과 함께 보인다', { kind: 'read', file: 'a.html' }, 'a.html 읽는 중'],
  ['파일 이름을 모르면 쓰기 시작했다고만 보인다', { kind: 'write' }, '파일 쓰기 시작'],
  ['파일 쓰기는 파일 이름과 함께 보인다', { kind: 'write', file: 'a.html' }, 'a.html 쓰는 중'],
  ['쓴 양을 알면 KB 로 함께 보인다', { kind: 'write', file: 'a.html', size: 12698 }, 'a.html 쓰는 중 · 12.4KB'],
  ['답변을 쓰기 시작하면 답변 쓰는 중으로 보인다', { kind: 'answer' }, '답변 쓰는 중'],
  ['도구 실패는 재시도로 보인다', { kind: 'retry' }, '다시 시도하는 중'],
  ['모델의 실패는 이유와 함께 보인다', { kind: 'fail', text: 'error_max_turns' }, '실패: error_max_turns'],
  ['서버 오류는 이유와 함께 보인다', { kind: 'error', text: 'boom' }, '오류: boom'],
  ['끝나면 완료로 보인다', { kind: 'end' }, '완료'],
  ['답변은 진행 표시가 아니라 대화에 보인다', { kind: 'reply', text: '고쳤습니다' }, null],
], (event, want) => assert.equal(stepText(event), want)));

test('elapsedText', (t) => table(t, [
  ['1분 미만은 초로 보인다', 42_300, '42초'],
  ['1분 이상은 분과 초로 보인다', 102_000, '1분 42초'],
], (ms, want) => assert.equal(elapsedText(ms), want)));

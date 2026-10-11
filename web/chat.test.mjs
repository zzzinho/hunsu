import { test } from 'node:test';
import assert from 'node:assert/strict';
import { elapsedText, parseSSE, stepText, variantsText } from './chat.mjs';
import { translator } from './i18n.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

const en = translator('en');

test('parseSSE', (t) => table(t, [
  ['완성된 이벤트를 JSON 으로 읽는다', 'data: {"kind":"start"}\n\n', [{ kind: 'start' }], ''],
  ['여러 이벤트를 한 번에 읽는다', 'data: {"kind":"start"}\n\ndata: {"kind":"end"}\n\n', [{ kind: 'start' }, { kind: 'end' }], ''],
  ['끝나지 않은 이벤트는 다음 조각을 위해 남긴다', 'data: {"kind":"start"}\n\ndata: {"ki', [{ kind: 'start' }], 'data: {"ki'],
], (text, events, rest) => assert.deepEqual(parseSSE(text), { events, rest })));

test('stepText', (t) => table(t, [
  ['시작하면 시작했다고 보인다', { kind: 'start' }, 'Started'],
  ['생각을 시작하면 생각하는 중으로 보인다', { kind: 'think' }, 'Thinking'],
  ['파일 이름을 모르면 파일을 연다고만 보인다', { kind: 'read' }, 'Opening files'],
  ['파일 읽기는 파일 이름과 함께 보인다', { kind: 'read', file: 'a.html' }, 'Reading a.html'],
  ['파일 이름을 모르면 쓰기 시작했다고만 보인다', { kind: 'write' }, 'Starting to write'],
  ['파일 쓰기는 파일 이름과 함께 보인다', { kind: 'write', file: 'a.html' }, 'Writing a.html'],
  ['쓴 양을 알면 KB 로 함께 보인다', { kind: 'write', file: 'a.html', size: 12698 }, 'Writing a.html · 12.4KB'],
  ['답변을 쓰기 시작하면 답변 쓰는 중으로 보인다', { kind: 'answer' }, 'Writing the reply'],
  ['화면 확인 도구는 화면을 확인하는 중으로 보인다', { kind: 'tool', text: 'screenshot' }, 'Checking the render'],
  ['그 밖의 도구는 도구 이름과 함께 보인다', { kind: 'tool', text: 'lookup' }, 'Using lookup'],
  ['도구 실패는 재시도로 보인다', { kind: 'retry' }, 'Retrying'],
  ['모델의 실패는 이유와 함께 보인다', { kind: 'fail', text: 'error_max_turns' }, 'Failed: error_max_turns'],
  ['서버 오류는 이유와 함께 보인다', { kind: 'error', text: 'boom' }, 'Error: boom'],
  ['끝나면 완료로 보인다', { kind: 'end' }, 'Done'],
  ['답변은 진행 표시가 아니라 대화에 보인다', { kind: 'reply', text: '고쳤습니다' }, null],
], (event, want) => assert.equal(stepText(event, en), want)));

test('variantsText', (t) => table(t, [
  ['안마다 마지막 진행을 안 번호순으로 보인다', { 2: 'Writing a.html', 1: 'Thinking' }, 'Take 1 · Thinking / Take 2 · Writing a.html'],
  ['아직 소식이 없는 안은 보이지 않는다', {}, ''],
], (steps, want) => assert.equal(variantsText(steps, en), want)));

test('elapsedText', (t) => table(t, [
  ['1분 미만은 초로 보인다', 42_300, '42s'],
  ['1분 이상은 분과 초로 보인다', 102_000, '1m 42s'],
], (ms, want) => assert.equal(elapsedText(ms, en), want)));

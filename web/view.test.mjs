import { test } from 'node:test';
import assert from 'node:assert/strict';
import { modeOf, openFile, pageFileName, pageFromPath, reactionTo, watchList } from './view.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

const pageOpen = { open: 'a.html', preview: 'a.html' };
const designOpen = { open: 'DESIGN.md', preview: 'a.html' };

test('modeOf', (t) => table(t, [
  ['html 은 htmlmixed 모드로 연다', 'a.html', 'htmlmixed'],
  ['css 는 css 모드로 연다', 'design.css', 'css'],
  ['md 는 markdown 모드로 연다', 'DESIGN.md', 'markdown'],
  ['모르는 확장자는 일반 텍스트로 연다', 'notes.txt', null],
], (path, want) => assert.equal(modeOf(path), want)));

test('pageFromPath', (t) => table(t, [
  ['렌더 주소에서 페이지 파일 이름을 뽑는다', '/p/about.html', 'about.html'],
  ['인코딩된 이름은 풀어서 돌려준다', '/p/my%20page.html', 'my page.html'],
  ['페이지가 아닌 렌더 주소는 페이지로 보지 않는다', '/p/design.css', null],
  ['렌더 주소가 아니면 페이지로 보지 않는다', 'blank', null],
], (pathname, want) => assert.equal(pageFromPath(pathname), want)));

test('pageFileName', (t) => table(t, [
  ['이름에 .html 을 붙여 페이지 파일 이름을 만든다', 'about', 'about.html'],
  ['이미 .html 로 끝나면 그대로 둔다', 'about.html', 'about.html'],
  ['앞뒤 공백은 지운다', '  about ', 'about.html'],
], (name, want) => assert.equal(pageFileName(name), want)));

test('openFile', (t) => table(t, [
  ['페이지를 열면 렌더 화면도 그 페이지를 보여준다', { open: null, preview: null }, 'a.html', pageOpen],
  ['디자인 시스템 파일을 열면 렌더 화면은 보던 페이지를 유지한다', pageOpen, 'DESIGN.md', designOpen],
], (state, path, want) => assert.deepEqual(openFile(state, path), want)));

test('watchList', (t) => table(t, [
  ['열린 페이지와 design.css 를 감시한다', pageOpen, ['a.html', 'design.css']],
  ['디자인 시스템 파일을 열면 렌더 중인 페이지도 함께 감시한다', designOpen, ['DESIGN.md', 'a.html', 'design.css']],
  ['아무것도 열지 않았으면 design.css 만 감시한다', { open: null, preview: null }, ['design.css']],
], (state, want) => assert.deepEqual(watchList(state), want)));

test('reactionTo', (t) => table(t, [
  ['열린 페이지가 바뀌면 렌더 화면과 에디터를 새로고침한다', 'a.html', pageOpen, false, { preview: true, editor: true }],
  ['저장하지 않은 편집이 있으면 에디터는 덮어쓰지 않는다', 'a.html', pageOpen, true, { preview: true, editor: false }],
  ['design.css 가 바뀌면 렌더 화면을 새로고침한다', 'design.css', pageOpen, false, { preview: true, editor: false }],
  ['렌더 중이 아닌 파일이 바뀌면 렌더 화면은 그대로 둔다', 'DESIGN.md', designOpen, false, { preview: false, editor: true }],
], (changed, state, dirty, want) => assert.deepEqual(reactionTo(changed, state, dirty), want)));

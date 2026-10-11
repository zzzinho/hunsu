import { test } from 'node:test';
import assert from 'node:assert/strict';
import { replaceText } from './textedit.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

const entities = { '&amp;': '&', '&lt;': '<', '&gt;': '>', '&nbsp;': ' ' };
const decode = (raw) => raw.replace(/&\w+;/g, (entity) => entities[entity] ?? entity);

test('replaceText', (t) => table(t, [
  ['문구를 새 문구로 바꾼다',
    '<h1>안녕</h1>', { text: '안녕', occurrence: 0, value: '반가워' }, '<h1>반가워</h1>'],
  ['문구 앞뒤의 줄바꿈과 들여쓰기는 그대로 둔다',
    '<p>\n  안녕\n</p>', { text: '\n  안녕\n', occurrence: 0, value: '반가워' }, '<p>\n  반가워\n</p>'],
  ['같은 문구가 여럿이면 몇 번째인지로 찾는다',
    '<a>더 보기</a><a>더 보기</a>', { text: '더 보기', occurrence: 1, value: '자세히' }, '<a>더 보기</a><a>자세히</a>'],
  ['원본에 엔티티로 쓴 문구도 찾는다',
    '<p>A &amp; B</p>', { text: 'A & B', occurrence: 0, value: 'A & C' }, '<p>A &amp; C</p>'],
  ['새 문구의 HTML 특수 문자는 글자로 보이게 바꾼다',
    '<p>안녕</p>', { text: '안녕', occurrence: 0, value: '<b>굵게</b> & 끝' }, '<p>&lt;b&gt;굵게&lt;/b&gt; &amp; 끝</p>'],
  ['줄바꿈 없는 공백은 &nbsp; 로 쓴다',
    '<p>안녕</p>', { text: '안녕', occurrence: 0, value: '10 원' }, '<p>10&nbsp;원</p>'],
  ['태그 속성 안의 같은 문구는 건드리지 않는다',
    '<img alt="로고"><p>로고</p>', { text: '로고', occurrence: 0, value: '회사' }, '<img alt="로고"><p>회사</p>'],
  ['속성 값의 따옴표 안에 있는 > 는 태그의 끝으로 보지 않는다',
    '<a title="a > b">링크</a>', { text: '링크', occurrence: 0, value: '이동' }, '<a title="a > b">이동</a>'],
  ['화면에 보이지 않는 스크립트·스타일·제목·주석 안의 같은 문구는 세지 않는다',
    '<title>메뉴</title><script>"메뉴"</script><style>/*메뉴*/</style><!-- 메뉴 --><p>메뉴</p>',
    { text: '메뉴', occurrence: 0, value: '목록' },
    '<title>메뉴</title><script>"메뉴"</script><style>/*메뉴*/</style><!-- 메뉴 --><p>목록</p>'],
  ['따옴표 없는 속성값 속의 \' 는 따옴표로 보지 않는다',
    "<div data-x=it's>안녕</div><p>'</p>", { text: '안녕', occurrence: 0, value: '반가워' }, "<div data-x=it's>반가워</div><p>'</p>"],
  ['빈 주석(<!-->) 뒤의 문구도 센다',
    '<!--><p>안녕</p><!-- 끝 -->', { text: '안녕', occurrence: 0, value: '반가워' }, '<!--><p>반가워</p><!-- 끝 -->'],
  ['태그를 시작하지 않는 < 는 문구로 본다',
    '<p>1 < 2</p>', { text: '1 < 2', occurrence: 0, value: '1 < 3' }, '<p>1 &lt; 3</p>'],
  ['원본에 없는 문구는 고치지 않는다',
    '<p>안녕</p>', { text: '잘 가', occurrence: 0, value: '반가워' }, null],
  ['같은 문구가 원본에 그만큼 많지 않으면 고치지 않는다',
    '<p>안녕</p>', { text: '안녕', occurrence: 1, value: '반가워' }, null],
], (source, edit, want) => assert.equal(replaceText(source, edit, decode), want)));

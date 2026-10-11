export function parseSSE(text) {
  const blocks = text.split('\n\n');
  const rest = blocks.pop();
  const events = blocks.flatMap((block) => block.split('\n')
    .filter((line) => line.startsWith('data: '))
    .map((line) => JSON.parse(line.slice('data: '.length))));
  return { events, rest };
}

const kilobytes = (bytes) => `${(bytes / 1024).toFixed(1)}KB`;

const toolSteps = { screenshot: '화면 확인하는 중' };

const steps = {
  start: () => '시작했습니다',
  think: () => '생각하는 중',
  read: (e) => (e.file ? `${e.file} 읽는 중` : '파일 여는 중'),
  write: (e) => (e.file ? `${e.file} 쓰는 중${e.size ? ` · ${kilobytes(e.size)}` : ''}` : '파일 쓰기 시작'),
  answer: () => '답변 쓰는 중',
  tool: (e) => toolSteps[e.text] ?? `${e.text} 쓰는 중`,
  retry: () => '다시 시도하는 중',
  fail: (e) => `실패: ${e.text}`,
  error: (e) => `오류: ${e.text}`,
  end: () => '완료',
};

export const stepText = (event) => steps[event.kind]?.(event) ?? null;

export function elapsedText(ms) {
  const seconds = Math.floor(ms / 1000);
  return seconds < 60 ? `${seconds}초` : `${Math.floor(seconds / 60)}분 ${seconds % 60}초`;
}

export const variantsText = (steps) => Object.keys(steps)
  .sort((a, b) => a - b)
  .map((id) => `안 ${id} · ${steps[id]}`)
  .join(' / ');

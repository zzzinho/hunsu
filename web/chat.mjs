export function parseSSE(text) {
  const blocks = text.split('\n\n');
  const rest = blocks.pop();
  const events = blocks.flatMap((block) => block.split('\n')
    .filter((line) => line.startsWith('data: '))
    .map((line) => JSON.parse(line.slice('data: '.length))));
  return { events, rest };
}

const kilobytes = (bytes) => `${(bytes / 1024).toFixed(1)}KB`;

const toolSteps = { screenshot: 'step.toolScreenshot' };

const steps = {
  start: (e, t) => t('step.start'),
  think: (e, t) => t('step.think'),
  read: (e, t) => (e.file ? t('step.read', { file: e.file }) : t('step.readAny')),
  write: (e, t) => (e.file ? `${t('step.write', { file: e.file })}${e.size ? ` · ${kilobytes(e.size)}` : ''}` : t('step.writeAny')),
  answer: (e, t) => t('step.answer'),
  tool: (e, t) => (toolSteps[e.text] ? t(toolSteps[e.text]) : t('step.tool', { tool: e.text })),
  retry: (e, t) => t('step.retry'),
  fail: (e, t) => t('step.fail', { text: e.text }),
  error: (e, t) => t('step.error', { text: e.text }),
  end: (e, t) => t('step.end'),
};

export const stepText = (event, t) => steps[event.kind]?.(event, t) ?? null;

export function elapsedText(ms, t) {
  const seconds = Math.floor(ms / 1000);
  return seconds < 60 ? t('elapsed.seconds', { s: seconds }) : t('elapsed.minutes', { m: Math.floor(seconds / 60), s: seconds % 60 });
}

export const variantsText = (steps, t) => Object.keys(steps)
  .sort((a, b) => a - b)
  .map((id) => t('variants.step', { id, step: steps[id] }))
  .join(' / ');

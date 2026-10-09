const LIMITS = { files: [160, 480], source: [320, 900], chat: [120, 800] };
const DIRECTION = { files: 1, source: -1, chat: -1 };
const MIN_PREVIEW = 320;
const MIN_EDITOR = 120;
const SOURCE_BAR = 40;
const KEY_STEP = 16;
const KEYS = { x: { ArrowLeft: -KEY_STEP, ArrowRight: KEY_STEP }, y: { ArrowUp: -KEY_STEP, ArrowDown: KEY_STEP } };

export const DEFAULT_SIZES = { files: 220, source: 520, chat: 340 };

export function togglePanel(panels, which) {
  if (which !== 'focus') return { ...panels, [which]: !panels[which] };
  const anyOpen = panels.files || panels.source;
  return { files: !anyOpen, source: !anyOpen };
}

function room({ sizes, panels, viewport }, kind) {
  if (kind === 'chat') return viewport.height - SOURCE_BAR - MIN_EDITOR;
  const other = kind === 'files' ? 'source' : 'files';
  return viewport.width - (panels[other] ? sizes[other] : 0) - MIN_PREVIEW;
}

export function resize(layout, kind, start, delta) {
  const [min, max] = LIMITS[kind];
  const size = Math.max(min, Math.min(max, room(layout, kind), start + DIRECTION[kind] * delta));
  return { ...layout.sizes, [kind]: size };
}

export const keyDelta = (key, axis) => KEYS[axis][key] ?? 0;

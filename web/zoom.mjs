const LEVELS = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2, 3];
const MIN = LEVELS[0];
const MAX = LEVELS.at(-1);
const PINCH_SPEED = 100;
const KEYS = { '=': 1, '+': 1, '-': -1 };

const clamp = (zoom) => Math.min(MAX, Math.max(MIN, zoom));

export function zoomStep(zoom, direction) {
  const next = direction > 0 ? LEVELS.find((level) => level > zoom + 1e-9) : LEVELS.findLast((level) => level < zoom - 1e-9);
  return next ?? clamp(zoom);
}

export const zoomBy = (zoom, deltaY) => Math.round(clamp(zoom * Math.exp(-deltaY / PINCH_SPEED)) * 100) / 100;

export function zoomForKey(key, zoom) {
  if (key === '0') return 1;
  return key in KEYS ? zoomStep(zoom, KEYS[key]) : null;
}

// 모바일처럼 너비가 정해진 화면은 그 너비를 확대·축소하고, 데스크톱은 렌더 영역을 채우도록 논리 너비를 바꾼다.
export function frameBox({ width, zoom, stage }) {
  const logicalWidth = width ?? stage.width / zoom;
  return { width: logicalWidth, height: stage.height / zoom, boxWidth: logicalWidth * zoom, boxHeight: stage.height };
}

export const zoomText = (zoom) => `${Math.round(zoom * 100)}%`;

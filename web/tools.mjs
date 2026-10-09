const keys = { v: 'select', h: 'hand' };

const DRAG_THRESHOLD = 3;

export const toolForKey = (key) => keys[key.toLowerCase()] ?? null;

export const isDrag = (start, point) =>
  Math.abs(point.x - start.x) + Math.abs(point.y - start.y) > DRAG_THRESHOLD;

export const panPosition = (start, point) => ({
  left: start.scrollX - (point.x - start.x),
  top: start.scrollY - (point.y - start.y),
});

const pad = (n) => String(n).padStart(2, '0');

export function timeText(date, now) {
  const clock = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  return date.toDateString() === now.toDateString() ? `오늘 ${clock}` : `${date.getMonth() + 1}월 ${date.getDate()}일 ${clock}`;
}

export function formatElapsed(milliseconds: number): string {
  const seconds = Math.floor(Math.max(0, milliseconds) / 1000);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor(seconds / 60) % 60;
  const parts = [minutes, seconds % 60].map(value => String(value).padStart(2, '0'));
  if (hours > 0) parts.unshift(String(hours).padStart(2, '0'));
  return parts.join(':');
}

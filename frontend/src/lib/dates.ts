// localToday returns the user's local calendar date as YYYY-MM-DD. Practice
// days are whatever day it is for the musician, not the server.
export function localToday(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

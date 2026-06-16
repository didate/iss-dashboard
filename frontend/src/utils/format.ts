export function fmtNum(v: number): string {
  return v.toLocaleString('fr-FR');
}

export function fmtPct(v: number, decimals = 1): string {
  return `${v.toFixed(decimals)}%`;
}

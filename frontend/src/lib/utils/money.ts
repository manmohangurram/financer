// Float-safe money rounding. The naive Math.round(x*100)/100 mis-rounds values
// like 1.005 (1.005*100 === 100.49999…); adding Number.EPSILON fixes the
// classic floating-point edge cases.
export function roundMoney(value: number): number {
  return Math.round((value + Number.EPSILON) * 100) / 100;
}

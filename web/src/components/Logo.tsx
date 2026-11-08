/** Max's hand-drawn wordmark (white letters, orange M + cat tail) — built for dark backgrounds. */
export function Logo({ className = "" }: { className?: string }) {
  return <img src="/brand/MeowanceLogo.svg" alt="meowance" className={`block w-auto ${className}`} />;
}

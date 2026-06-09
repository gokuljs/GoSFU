const SEGMENT_COUNT = 20

// Placeholder until rate-limit data is wired from the backend.
const DUMMY_USAGE_PCT = 15

interface RateLimitMeterProps {
  /** Usage percentage 0–100. Omit to show the dummy placeholder. */
  usagePct?: number
}

export function RateLimitMeter({ usagePct }: RateLimitMeterProps) {
  const pct = usagePct ?? DUMMY_USAGE_PCT
  const filled = Math.round((pct / 100) * SEGMENT_COUNT)
  const isDummy = usagePct === undefined

  return (
    <div className="flex min-h-0 flex-1 flex-col border border-[#1a1a1a] bg-[#050505] p-2">
      <div className="mb-2 flex shrink-0 items-center justify-between gap-2 text-[9px] tracking-wider uppercase">
        <span className="text-white/30">Rate Limit</span>
        <span className="font-mono text-white/20">
          {isDummy ? "—" : `${Math.round(pct)}%`}
        </span>
      </div>
      <div
        className="flex min-h-0 flex-1 items-stretch gap-[2px]"
        role="meter"
        aria-label="Rate limit usage"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        {Array.from({ length: SEGMENT_COUNT }, (_, i) => (
          <div
            key={i}
            className={`min-h-0 min-w-0 flex-1 rounded-[2px] transition-colors duration-200 ${
              i < filled
                ? "bg-[#00d4aa] shadow-[0_0_8px_rgba(0,212,170,0.45)]"
                : "bg-[#0d1f1c]"
            }`}
          />
        ))}
      </div>
    </div>
  )
}

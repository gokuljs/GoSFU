import type { QuotaState } from "@/hooks/use-room-stream"

const SEGMENT_COUNT = 15
const DEFAULT_LIMIT = 15

interface SessionQuotaMeterProps {
  quota: QuotaState | null
}

export function SessionQuotaMeter({ quota }: SessionQuotaMeterProps) {
  const limit = quota?.limit && quota.limit > 0 ? quota.limit : DEFAULT_LIMIT
  const used = Math.min(Math.max(quota?.used ?? 0, 0), limit)
  const exhausted = quota?.exhausted ?? used >= limit
  const filled = exhausted
    ? SEGMENT_COUNT
    : Math.min(SEGMENT_COUNT, Math.floor((used / limit) * SEGMENT_COUNT))
  const activeIndex = exhausted ? -1 : Math.min(filled, SEGMENT_COUNT - 1)
  const remaining = Math.max(limit - used, 0)
  const valueText = exhausted
    ? "Session quota exhausted"
    : `${remaining} of ${limit} turns remaining`

  return (
    <div className="flex min-h-0 flex-1 flex-col border border-[#1a1a1a] bg-[#050505] p-2">
      <div className="mb-2 flex shrink-0 items-center justify-between gap-2 text-[9px] tracking-wider uppercase">
        <span className={exhausted ? "text-[#ff4444]/80" : "text-white/30"}>
          Session Quota
        </span>
        <span className="font-mono text-white/25">
          {used}/{limit}
        </span>
      </div>
      <div
        className="grid min-h-0 flex-1 items-stretch gap-[2px]"
        style={{ gridTemplateColumns: `repeat(${SEGMENT_COUNT}, minmax(0, 1fr))` }}
        role="meter"
        aria-label="Session quota usage"
        aria-valuenow={used}
        aria-valuemin={0}
        aria-valuemax={limit}
        aria-valuetext={valueText}
      >
        {Array.from({ length: SEGMENT_COUNT }, (_, i) => {
          const filledClass = exhausted
            ? "bg-[#ff4444]/80 shadow-[0_0_8px_rgba(255,68,68,0.4)]"
            : "bg-[#00d4aa] shadow-[0_0_8px_rgba(0,212,170,0.45)]"
          const emptyClass = "bg-[#0d1f1c]"
          const activeClass =
            i === activeIndex
              ? "quota-bar-active bg-[#00d4aa]/80 shadow-[0_0_10px_rgba(0,212,170,0.55)]"
              : ""

          return (
            <div
              key={i}
              className={`min-h-0 min-w-0 rounded-[2px] transition-colors duration-200 ${
                i < filled ? filledClass : activeClass || emptyClass
              }`}
            />
          )
        })}
      </div>
      <div className="mt-2 min-h-3 font-mono text-[8px] tracking-wider text-white/20 uppercase">
        {quota ? valueText : "Waiting for quota state"}
      </div>
    </div>
  )
}

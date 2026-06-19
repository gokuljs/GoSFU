import { useEffect, useMemo, useRef, useState } from "react"

const DEFAULT_MAX_MS = 30 * 60 * 1000
const WARNING_REMAINING_MS = 5 * 60 * 1000

function parseMaxDurationMs(raw: string | undefined): number {
  if (!raw?.trim()) return DEFAULT_MAX_MS
  const match = /^(\d+(?:\.\d+)?)(ms|s|m|h)$/.exec(raw.trim())
  if (!match) return DEFAULT_MAX_MS
  const value = Number(match[1])
  switch (match[2]) {
    case "ms":
      return value
    case "s":
      return value * 1000
    case "m":
      return value * 60 * 1000
    case "h":
      return value * 60 * 60 * 1000
    default:
      return DEFAULT_MAX_MS
  }
}

function formatDuration(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
}

export function useSessionTimer(active: boolean) {
  const maxDurationMs = useMemo(
    () => parseMaxDurationMs(import.meta.env.VITE_SESSION_MAX_DURATION),
    []
  )
  const [elapsedMs, setElapsedMs] = useState(0)
  const startedAtRef = useRef<number | null>(null)
  const prevActiveRef = useRef(active)

  if (prevActiveRef.current !== active) {
    prevActiveRef.current = active
    setElapsedMs(0)
  }

  useEffect(() => {
    if (!active) {
      startedAtRef.current = null
      return
    }

    startedAtRef.current = Date.now()

    const interval = window.setInterval(() => {
      if (startedAtRef.current === null) return
      setElapsedMs(Date.now() - startedAtRef.current)
    }, 1000)

    return () => window.clearInterval(interval)
  }, [active])

  const remainingMs = Math.max(0, maxDurationMs - elapsedMs)
  const maxLabel = formatDuration(maxDurationMs)
  const label = `${formatDuration(elapsedMs)} / ${maxLabel}`

  return {
    label,
    remainingMs,
    isWarning: active && remainingMs > 0 && remainingMs < WARNING_REMAINING_MS,
  }
}

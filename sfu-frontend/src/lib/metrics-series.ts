import type { ChartSeries } from "@/components/metrics-chart"
import { parseUtcTimestamp } from "@/lib/format-time"

interface MetricLike {
  id?: string
  ts: string
  turn?: number
  stage: string
  name: string
  value: number
  unit?: string
}

interface MetricFilter {
  stage: string
  name: string
  id: string
  label: string
  color: string
}

export function buildSeries(
  metrics: MetricLike[],
  filters: MetricFilter[]
): ChartSeries[] {
  return filters.map((filter) => ({
    id: filter.id,
    label: filter.label,
    color: filter.color,
    points: metrics
      .filter((m) => m.stage === filter.stage && m.name === filter.name)
      .map((m) => ({
        id: m.id ?? `${filter.id}-${m.ts}-${m.turn ?? "na"}`,
        ts: parseUtcTimestamp(m.ts).getTime(),
        isoTs: m.ts,
        value: m.value,
        label: filter.label,
        stage: filter.stage,
        name: filter.name,
        unit: m.unit ?? "ms",
        turn: m.turn,
      }))
      .filter((point) => Number.isFinite(point.ts))
      .sort((a, b) => a.ts - b.ts),
  }))
}

export function avgValue(points: Array<{ value: number }>) {
  if (points.length === 0) return 0
  return points.reduce((sum, p) => sum + p.value, 0) / points.length
}

export function latestValue(points: Array<{ value: number }>) {
  if (points.length === 0) return 0
  return points[points.length - 1].value
}

import type { ChartSeries } from "@/components/metrics-chart"

export function buildSeries(
  metrics: Array<{ ts: string; stage: string; name: string; value: number }>,
  filters: Array<{ stage: string; name: string; id: string; label: string; color: string }>
): ChartSeries[] {
  return filters.map((filter) => ({
    id: filter.id,
    label: filter.label,
    color: filter.color,
    points: metrics
      .filter((m) => m.stage === filter.stage && m.name === filter.name)
      .map((m) => ({ ts: new Date(m.ts).getTime(), value: m.value })),
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

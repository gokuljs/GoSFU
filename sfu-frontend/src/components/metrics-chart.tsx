import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent,
} from "react"
import { formatLocalTime } from "@/lib/format-time"

export interface ChartPoint {
  id: string
  ts: number
  isoTs: string
  value: number
  label: string
  stage: string
  name: string
  unit: string
  turn?: number
}

export interface ChartSeries {
  id: string
  label: string
  color: string
  points: ChartPoint[]
}

interface MetricsChartProps {
  series: ChartSeries[]
  height?: number
  showGrid?: boolean
  showLabels?: boolean
  className?: string
}

const GRID_COLOR = "rgba(255,255,255,0.075)"
const AXIS_COLOR = "rgba(255,255,255,0.15)"
const LINE_COLOR = "rgba(255,255,255,0.92)"
const ANIMATION_MS = 220
const POINT_GAP = 42
const AXIS_HEIGHT = 18

interface HoverState {
  x: number
  y: number
  pointY: number
  pageX: number
  pageY: number
  time: string
  items: Array<{ id: string; label: string; value: string; turn?: number }>
}

type DrawableSeries = ChartSeries[]

interface ChartBounds {
  width: number
  height: number
  pad: { top: number; right: number; bottom: number; left: number }
  plotW: number
  plotH: number
  minX: number
  xRange: number
  minY: number
  yRange: number
  timestamps: number[]
}

export function MetricsChart({
  series,
  height = 120,
  showGrid = true,
  showLabels = false,
  className = "",
}: MetricsChartProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const animationRef = useRef<number | null>(null)
  const drawnSeriesRef = useRef<DrawableSeries>([])
  const pinnedToRightRef = useRef(true)
  const [viewportWidth, setViewportWidth] = useState(0)
  const [hover, setHover] = useState<HoverState | null>(null)

  const hasData = useMemo(
    () => series.some((s) => s.points.length > 0),
    [series]
  )

  useEffect(() => {
    if (import.meta.env.DEV && series.some((s) => s.points.length > 0)) {
      console.debug(
        "[metrics] rendered",
        series.map((s) => `${s.id}:${s.points.length}`).join(", ")
      )
    }
  }, [series])

  useEffect(() => {
    const scrollEl = scrollRef.current
    if (!scrollEl) return

    const resizeObserver = new ResizeObserver(([entry]) => {
      setViewportWidth(entry.contentRect.width)
    })
    resizeObserver.observe(scrollEl)
    return () => resizeObserver.disconnect()
  }, [])

  const chartWidth = useMemo(() => {
    const timestamps = uniqueTimestamps(series)
    const pointWidth = Math.max(0, timestamps.length - 1) * POINT_GAP + 16
    return Math.max(viewportWidth, pointWidth, 1)
  }, [series, viewportWidth])

  useEffect(() => {
    const scrollEl = scrollRef.current
    if (!scrollEl || !pinnedToRightRef.current) return
    scrollEl.scrollLeft = scrollEl.scrollWidth - scrollEl.clientWidth
  }, [chartWidth, series])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const dpr = window.devicePixelRatio || 1
    const width = chartWidth
    canvas.width = width * dpr
    canvas.height = height * dpr

    const ctx = canvas.getContext("2d")
    if (!ctx) return

    ctx.scale(dpr, dpr)

    const from = drawnSeriesRef.current
    const start = performance.now()

    const animate = (now: number) => {
      const progress = Math.min(1, (now - start) / ANIMATION_MS)
      const eased = 1 - Math.pow(1 - progress, 3)
      const frameSeries = interpolateSeries(from, series, eased)
      drawnSeriesRef.current = frameSeries
      drawChart(ctx, frameSeries, {
        width,
        height,
        showGrid,
        showLabels,
      })

      if (progress < 1) {
        animationRef.current = requestAnimationFrame(animate)
      } else {
        drawnSeriesRef.current = series
        animationRef.current = null
      }
    }

    if (animationRef.current !== null) {
      cancelAnimationFrame(animationRef.current)
    }
    animationRef.current = requestAnimationFrame(animate)

    return () => {
      if (animationRef.current !== null) {
        cancelAnimationFrame(animationRef.current)
        animationRef.current = null
      }
    }
  }, [series, height, showGrid, showLabels, chartWidth])

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const canvas = canvasRef.current
    if (!canvas || !hasData) return

    const rect = canvas.getBoundingClientRect()
    const bounds = chartBounds(series, chartWidth, height, showLabels)
    if (!bounds) return

    const x = event.clientX - rect.left
    const y = event.clientY - rect.top
    const ts = timestampAtX(bounds, x)

    const items = nearestPoints(series, ts)
    const firstPoint = items[0]?.point
    if (!firstPoint) return
    const pointY = yForValue(bounds, firstPoint.value)

    setHover({
      x: Math.min(Math.max(x, 8), rect.width - 8),
      y: Math.min(Math.max(y, 8), rect.height - 8),
      pointY,
      pageX: event.clientX,
      pageY: event.clientY,
      time: formatLocalTime(firstPoint.isoTs),
      items: items.map(({ series: itemSeries, point }) => ({
        id: `${itemSeries.id}-${point.id}`,
        label: itemSeries.label,
        value: formatPointValue(point),
        turn: point.turn,
      })),
    })
  }

  return (
    <div
      ref={scrollRef}
      className={`relative w-full overflow-x-auto overflow-y-hidden border border-[#1a1a1a] bg-[#050505] ${className}`}
      style={{ height }}
      onScroll={(event) => {
        const target = event.currentTarget
        pinnedToRightRef.current =
          target.scrollLeft + target.clientWidth >= target.scrollWidth - 8
      }}
    >
      <div
        className="relative"
        style={{ width: chartWidth, height }}
        onPointerMove={handlePointerMove}
        onPointerLeave={() => setHover(null)}
      >
        <canvas
          ref={canvasRef}
          className="block"
          style={{ width: chartWidth, height }}
          aria-hidden
        />
        {hover && (
          <>
            <div
              className="pointer-events-none absolute top-0 bottom-0 w-px bg-white/20"
              style={{ left: hover.x }}
            />
            <div
              className="pointer-events-none absolute h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full border border-white bg-[#00d4aa] shadow-[0_0_10px_rgba(0,212,170,0.8)]"
              style={{ left: hover.x, top: hover.pointY }}
            />
            <div
              className="pointer-events-none fixed z-50 min-w-40 border border-[#00d4aa]/25 bg-[#050505]/95 px-2 py-1.5 font-mono text-[9px] shadow-[0_0_18px_rgba(0,212,170,0.18)]"
              style={{
                left:
                  hover.pageX > window.innerWidth - 190
                    ? hover.pageX - 172
                    : hover.pageX + 12,
                top:
                  hover.pageY > window.innerHeight - 110
                    ? hover.pageY - 96
                    : hover.pageY + 12,
              }}
            >
              <div className="mb-1 text-white/40">{hover.time}</div>
              <div className="space-y-0.5">
                {hover.items.map((item) => (
                  <div key={item.id} className="flex items-center justify-between gap-3">
                    <span className="truncate text-white/35">
                      {item.label}
                      {item.turn !== undefined ? ` T${item.turn}` : ""}
                    </span>
                    <span className="text-white/90">{item.value}</span>
                  </div>
                ))}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

function drawChart(
  ctx: CanvasRenderingContext2D,
  series: ChartSeries[],
  opts: { width: number; height: number; showGrid: boolean; showLabels: boolean }
) {
  const { width, height, showGrid, showLabels } = opts
  ctx.clearRect(0, 0, width, height)

  const bounds = chartBounds(series, width, height, showLabels)
  if (!bounds) {
    ctx.fillStyle = "rgba(255,255,255,0.15)"
    ctx.font = "10px 'JetBrains Mono Variable', monospace"
    ctx.textAlign = "center"
    ctx.fillText("WAITING FOR DATA", width / 2, height / 2)
    return
  }

  const { pad, plotH, minY, yRange } = bounds
  const xScale = (ts: number) => xForTimestamp(bounds, ts)
  const yScale = (v: number) =>
    pad.top + plotH - ((v - minY) / yRange) * plotH

  if (showGrid) {
    ctx.strokeStyle = GRID_COLOR
    ctx.lineWidth = 1
    for (let i = 0; i <= 4; i++) {
      const y = pad.top + (plotH / 4) * i
      ctx.beginPath()
      ctx.moveTo(pad.left, y)
      ctx.lineTo(pad.left + bounds.plotW, y)
      ctx.stroke()
    }
    bounds.timestamps.forEach((ts) => {
      const x = xForTimestamp(bounds, ts)
      ctx.beginPath()
      ctx.moveTo(x, pad.top)
      ctx.lineTo(x, pad.top + plotH)
      ctx.stroke()
    })
  }

  series.forEach((s, seriesIndex) => {
    if (s.points.length === 0) return
    const visiblePoints =
      s.points.length === 1
        ? [
            { ...s.points[0], ts: bounds.timestamps[0] },
            { ...s.points[0], ts: bounds.timestamps[0] },
          ]
        : s.points

    const gradient = ctx.createLinearGradient(0, pad.top, 0, pad.top + plotH)
    gradient.addColorStop(0, "rgba(0, 212, 170, 0.18)")
    gradient.addColorStop(1, "rgba(0, 212, 170, 0)")

    ctx.beginPath()
    visiblePoints.forEach((p, i) => {
      const x = xScale(p.ts)
      const y = yScale(p.value)
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    })
    ctx.lineTo(xScale(visiblePoints[visiblePoints.length - 1].ts), pad.top + plotH)
    ctx.lineTo(xScale(visiblePoints[0].ts), pad.top + plotH)
    ctx.closePath()
    ctx.fillStyle = gradient
    ctx.fill()

    ctx.beginPath()
    visiblePoints.forEach((p, i) => {
      const x = xScale(p.ts)
      const y = yScale(p.value)
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    })
    ctx.strokeStyle = LINE_COLOR
    ctx.lineWidth = 1.6
    ctx.globalAlpha = Math.max(0.45, 1 - seriesIndex * 0.18)
    ctx.setLineDash(seriesIndex === 0 ? [] : [4 + seriesIndex, 3])
    ctx.shadowColor = "rgba(0, 212, 170, 0.65)"
    ctx.shadowBlur = 8
    ctx.stroke()
    ctx.setLineDash([])
    ctx.shadowBlur = 0
    ctx.globalAlpha = 1
  })

  if (showLabels) {
    const axisY = pad.top + plotH + 1
    ctx.strokeStyle = "rgba(255,255,255,0.08)"
    ctx.beginPath()
    ctx.moveTo(pad.left, axisY)
    ctx.lineTo(pad.left + bounds.plotW, axisY)
    ctx.stroke()

    ctx.fillStyle = AXIS_COLOR
    ctx.font = "9px 'JetBrains Mono Variable', monospace"
    ctx.textAlign = "center"
    visibleAxisTimestamps(bounds).forEach((ts) => {
      ctx.fillText(
        formatLocalTime(new Date(ts).toISOString()),
        xForTimestamp(bounds, ts),
        height - 5
      )
    })
  }
}

function chartBounds(
  series: ChartSeries[],
  width: number,
  height: number,
  showLabels: boolean
): ChartBounds | null {
  const allPoints = series.flatMap((s) => s.points)
  if (allPoints.length === 0 || width <= 0 || height <= 0) return null

  const pad = { top: 0, right: 0, bottom: showLabels ? AXIS_HEIGHT : 0, left: 0 }
  const plotW = Math.max(1, width - pad.left - pad.right)
  const plotH = Math.max(1, height - pad.top - pad.bottom)
  const timestamps = uniqueTimestamps(series)
  const minX = Math.min(...allPoints.map((p) => p.ts))
  const maxX = Math.max(...allPoints.map((p) => p.ts))
  const rawMinY = Math.min(...allPoints.map((p) => p.value))
  const rawMaxY = Math.max(...allPoints.map((p) => p.value))
  const yPadding = Math.max((rawMaxY - rawMinY) * 0.18, rawMaxY * 0.08, 1)
  const minY = Math.max(0, rawMinY - yPadding)
  const maxY = rawMaxY + yPadding

  return {
    width,
    height,
    pad,
    plotW,
    plotH,
    minX,
    xRange: Math.max(maxX - minX, 1000),
    minY,
    yRange: Math.max(maxY - minY, 1),
    timestamps,
  }
}

function uniqueTimestamps(series: ChartSeries[]) {
  return Array.from(
    new Set(series.flatMap((item) => item.points.map((point) => point.ts)))
  ).sort((a, b) => a - b)
}

function xForTimestamp(bounds: ChartBounds, ts: number) {
  const { timestamps, pad, plotW } = bounds
  if (timestamps.length <= 1) return pad.left + Math.min(plotW / 2, POINT_GAP)

  const exactIndex = timestamps.indexOf(ts)
  if (exactIndex >= 0) {
    return Math.min(pad.left + exactIndex * POINT_GAP, pad.left + plotW)
  }

  let upperIndex = timestamps.findIndex((item) => item > ts)
  if (upperIndex === -1) upperIndex = timestamps.length - 1
  const lowerIndex = Math.max(0, upperIndex - 1)
  const lower = timestamps[lowerIndex]
  const upper = timestamps[upperIndex]
  const localProgress = upper === lower ? 0 : (ts - lower) / (upper - lower)
  const index = lowerIndex + localProgress
  return Math.min(pad.left + index * POINT_GAP, pad.left + plotW)
}

function timestampAtX(bounds: ChartBounds, x: number) {
  const { timestamps, pad } = bounds
  if (timestamps.length === 0) return bounds.minX
  if (timestamps.length === 1) return timestamps[0]
  const index = Math.round((Math.max(x, pad.left) - pad.left) / POINT_GAP)
  return timestamps[Math.min(Math.max(index, 0), timestamps.length - 1)]
}

function yForValue(bounds: ChartBounds, value: number) {
  return (
    bounds.pad.top +
    bounds.plotH -
    ((value - bounds.minY) / bounds.yRange) * bounds.plotH
  )
}

function visibleAxisTimestamps(bounds: ChartBounds) {
  if (bounds.timestamps.length <= 6) return bounds.timestamps
  const step = Math.ceil(bounds.timestamps.length / 6)
  return bounds.timestamps.filter(
    (_, index) => index % step === 0 || index === bounds.timestamps.length - 1
  )
}

function interpolateSeries(
  from: ChartSeries[],
  to: ChartSeries[],
  progress: number
): ChartSeries[] {
  return to.map((targetSeries) => {
    const sourceSeries = from.find((item) => item.id === targetSeries.id)
    const sourcePoints = sourceSeries?.points ?? []
    const fallback = sourcePoints[sourcePoints.length - 1]

    return {
      ...targetSeries,
      points: targetSeries.points.map((targetPoint, index) => {
        const sourcePoint = sourcePoints[index] ?? fallback ?? targetPoint
        return {
          ...targetPoint,
          ts: lerp(sourcePoint.ts, targetPoint.ts, progress),
          value: lerp(sourcePoint.value, targetPoint.value, progress),
        }
      }),
    }
  })
}

function nearestPoints(series: ChartSeries[], ts: number) {
  return series
    .map((item) => {
      const point = item.points.reduce<ChartPoint | null>((nearest, current) => {
        if (!nearest) return current
        return Math.abs(current.ts - ts) < Math.abs(nearest.ts - ts)
          ? current
          : nearest
      }, null)
      return point ? { series: item, point } : null
    })
    .filter((item): item is { series: ChartSeries; point: ChartPoint } => !!item)
}

function formatPointValue(point: ChartPoint) {
  const rounded = Math.round(point.value)
  if (point.unit === "tokens") return `${rounded} tok`
  if (point.unit === "count") return String(rounded)
  return `${rounded} ${point.unit || "ms"}`
}

function lerp(from: number, to: number, progress: number) {
  return from + (to - from) * progress
}

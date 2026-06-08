import { useEffect, useRef } from "react"

export interface ChartSeries {
  id: string
  label: string
  color: string
  points: Array<{ ts: number; value: number }>
}

interface MetricsChartProps {
  series: ChartSeries[]
  height?: number
  showGrid?: boolean
  showLabels?: boolean
  className?: string
}

const GRID_COLOR = "rgba(255,255,255,0.04)"
const AXIS_COLOR = "rgba(255,255,255,0.15)"

export function MetricsChart({
  series,
  height = 120,
  showGrid = true,
  showLabels = false,
  className = "",
}: MetricsChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    if (import.meta.env.DEV && series.some((s) => s.points.length > 0)) {
      console.debug(
        "[metrics] rendered",
        series.map((s) => `${s.id}:${s.points.length}`).join(", ")
      )
    }
  }, [series])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const dpr = window.devicePixelRatio || 1
    const width = canvas.clientWidth
    canvas.width = width * dpr
    canvas.height = height * dpr

    const ctx = canvas.getContext("2d")
    if (!ctx) return

    ctx.scale(dpr, dpr)
    ctx.clearRect(0, 0, width, height)

    const allPoints = series.flatMap((s) => s.points)
    if (allPoints.length === 0) {
      ctx.fillStyle = "rgba(255,255,255,0.15)"
      ctx.font = "10px 'JetBrains Mono Variable', monospace"
      ctx.textAlign = "center"
      ctx.fillText("WAITING FOR DATA", width / 2, height / 2)
      return
    }

    const pad = { top: 8, right: 8, bottom: showLabels ? 18 : 8, left: 8 }
    const plotW = width - pad.left - pad.right
    const plotH = height - pad.top - pad.bottom

    const minX = Math.min(...allPoints.map((p) => p.ts))
    const maxX = Math.max(...allPoints.map((p) => p.ts))
    const minY = Math.min(...allPoints.map((p) => p.value))
    const maxY = Math.max(...allPoints.map((p) => p.value))
    const xRange = Math.max(maxX - minX, 1000)
    const yRange = Math.max(maxY - minY, 1)

    const xScale = (ts: number) => pad.left + ((ts - minX) / xRange) * plotW
    const yScale = (v: number) =>
      pad.top + plotH - ((v - minY) / yRange) * plotH

    if (showGrid) {
      ctx.strokeStyle = GRID_COLOR
      ctx.lineWidth = 1
      for (let i = 0; i <= 4; i++) {
        const y = pad.top + (plotH / 4) * i
        ctx.beginPath()
        ctx.moveTo(pad.left, y)
        ctx.lineTo(pad.left + plotW, y)
        ctx.stroke()
      }
      for (let i = 0; i <= 5; i++) {
        const x = pad.left + (plotW / 5) * i
        ctx.beginPath()
        ctx.moveTo(x, pad.top)
        ctx.lineTo(x, pad.top + plotH)
        ctx.stroke()
      }
    }

    for (const s of series) {
      if (s.points.length < 2) continue

      const gradient = ctx.createLinearGradient(0, pad.top, 0, pad.top + plotH)
      gradient.addColorStop(0, s.color.replace(")", ", 0.35)").replace("rgb", "rgba"))
      gradient.addColorStop(1, "rgba(0, 212, 170, 0)")

      ctx.beginPath()
      s.points.forEach((p, i) => {
        const x = xScale(p.ts)
        const y = yScale(p.value)
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      })
      ctx.lineTo(xScale(s.points[s.points.length - 1].ts), pad.top + plotH)
      ctx.lineTo(xScale(s.points[0].ts), pad.top + plotH)
      ctx.closePath()
      ctx.fillStyle = gradient
      ctx.fill()

      ctx.beginPath()
      s.points.forEach((p, i) => {
        const x = xScale(p.ts)
        const y = yScale(p.value)
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      })
      ctx.strokeStyle = s.color
      ctx.lineWidth = 1.5
      ctx.shadowColor = s.color
      ctx.shadowBlur = 8
      ctx.stroke()
      ctx.shadowBlur = 0
    }

    if (showLabels) {
      ctx.fillStyle = AXIS_COLOR
      ctx.font = "9px 'JetBrains Mono Variable', monospace"
      ctx.textAlign = "center"
      for (let i = 0; i <= 5; i++) {
        const ts = minX + (xRange / 5) * i
        ctx.fillText(
          new Date(ts).toLocaleTimeString(undefined, {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
          }),
          pad.left + (plotW / 5) * i,
          height - 4
        )
      }
    }
  }, [series, height, showGrid, showLabels])

  return (
    <canvas
      ref={canvasRef}
      className={`w-full ${className}`}
      style={{ height }}
      aria-hidden
    />
  )
}

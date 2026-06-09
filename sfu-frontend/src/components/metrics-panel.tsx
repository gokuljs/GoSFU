import { ArrowsOutSimple, X } from "@phosphor-icons/react"
import { useMemo, useState, type ReactNode } from "react"
import {
  MetricsChart,
  type ChartSeries,
} from "@/components/metrics-chart"
import { avgValue, buildSeries, latestValue, minValue } from "@/lib/metrics-series"
import type { MetricPoint } from "@/hooks/use-room-stream"
import { RateLimitMeter } from "@/components/rate-limit-meter"
import { formatLocalTime } from "@/lib/format-time"

const TEAL = "rgb(0, 212, 170)"
const TEAL_DIM = "rgb(0, 184, 148)"
const TEAL_MID = "rgb(0, 155, 125)"

const STT_METRICS = [
  { key: "first_transcript_ms", label: "FIRST TX" },
  { key: "final_transcript_ms", label: "FINAL TX" },
  { key: "turn_latency_ms", label: "TURN LAT" },
]

const LLM_METRICS = [
  { key: "ttft_ms", label: "TTFT" },
  { key: "duration_ms", label: "DURATION" },
  { key: "total_tokens", label: "TOKENS" },
]

const TTS_METRICS = [
  { key: "first_byte_ms", label: "1ST BYTE" },
  { key: "first_playable_ms", label: "PLAYABLE" },
  { key: "synthesis_ms", label: "SYNTH" },
]

const LATENCY_FILTERS = [
  { stage: "stt", name: "first_transcript_ms", id: "stt-first", label: "STT First", color: TEAL },
  { stage: "stt", name: "final_transcript_ms", id: "stt-final", label: "STT Final", color: TEAL_DIM },
  { stage: "stt", name: "turn_latency_ms", id: "stt-turn", label: "STT Turn", color: TEAL_MID },
  { stage: "llm", name: "ttft_ms", id: "llm-ttft", label: "LLM TTFT", color: TEAL_MID },
  { stage: "llm", name: "duration_ms", id: "llm-duration", label: "LLM Duration", color: TEAL },
  { stage: "tts", name: "first_byte_ms", id: "tts-byte", label: "TTS TTFB", color: TEAL },
  { stage: "tts", name: "first_playable_ms", id: "tts-play", label: "TTS Playable", color: TEAL_DIM },
  { stage: "tts", name: "synthesis_ms", id: "tts-synth", label: "TTS Synth", color: TEAL_MID },
]

const PRIMARY_SPARK_BY_STAGE = {
  stt: "stt-first",
  llm: "llm-ttft",
  tts: "tts-byte",
}

interface MetricsPanelProps {
  metrics: MetricPoint[]
  latestByStage: Record<string, Record<string, MetricPoint>>
}

export function MetricsPanel({ metrics, latestByStage }: MetricsPanelProps) {
  const [expanded, setExpanded] = useState(false)

  const latencySeries = useMemo(
    () => buildSeries(metrics, LATENCY_FILTERS),
    [metrics]
  )

  return (
    <>
      <PanelShell title="METRICS" onExpand={() => setExpanded(true)}>
        <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto">
          <StageCard
            title="STT"
            defs={STT_METRICS}
            latest={latestByStage.stt ?? {}}
            sparkSeries={latencySeries.filter(
              (s) => s.id === PRIMARY_SPARK_BY_STAGE.stt
            )}
          />
          <StageCard
            title="LLM"
            defs={LLM_METRICS}
            latest={latestByStage.llm ?? {}}
            sparkSeries={latencySeries.filter(
              (s) => s.id === PRIMARY_SPARK_BY_STAGE.llm
            )}
          />
          <StageCard
            title="TTS"
            defs={TTS_METRICS}
            latest={latestByStage.tts ?? {}}
            sparkSeries={latencySeries.filter(
              (s) => s.id === PRIMARY_SPARK_BY_STAGE.tts
            )}
          />
          <RateLimitMeter />
        </div>
      </PanelShell>

      {expanded && (
        <ExpandOverlay
          onClose={() => setExpanded(false)}
          latencySeries={latencySeries}
        />
      )}
    </>
  )
}

function PanelShell({
  title,
  onExpand,
  children,
}: {
  title: string
  onExpand: () => void
  children: ReactNode
}) {
  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden border border-[#1a1a1a] bg-[#0a0a0a]">
      <div className="flex items-center justify-between border-b border-[#1a1a1a] px-3 py-2">
        <h2 className="text-[10px] font-medium tracking-wider text-[#00d4aa] uppercase">
          {title}
        </h2>
        <button
          type="button"
          onClick={onExpand}
          className="flex h-5 w-5 items-center justify-center border border-[#1a1a1a] text-white/30 hover:border-[#00d4aa]/40 hover:text-[#00d4aa]"
          aria-label="Expand metrics"
        >
          <ArrowsOutSimple size={12} />
        </button>
      </div>
      <div className="flex min-h-0 flex-1 flex-col p-2">{children}</div>
    </section>
  )
}

function StageCard({
  title,
  defs,
  latest,
  sparkSeries,
}: {
  title: string
  defs: Array<{ key: string; label: string }>
  latest: Record<string, MetricPoint>
  sparkSeries: ChartSeries[]
}) {
  const latestSparkPoint = sparkSeries
    .flatMap((series) => series.points)
    .sort((a, b) => b.ts - a.ts)[0]

  return (
    <div className="border border-[#1a1a1a] bg-[#050505] p-2">
      <div className="mb-1.5 flex items-center justify-between gap-2 text-[9px] tracking-wider uppercase">
        <span className="text-white/30">{title}</span>
        <span className="font-mono text-white/20">
          {latestSparkPoint ? formatLocalTime(latestSparkPoint.isoTs) : "NO TIME"}
        </span>
      </div>
      <MetricsChart series={sparkSeries} height={56} showGrid={true} />
      <div className="mt-2 grid grid-cols-3 gap-1">
        {defs.map((def) => {
          const point = latest[def.key]
          const unit = point?.unit === "tokens" ? "tok" : point?.unit === "count" ? "" : "ms"
          return (
            <div key={def.key}>
              <div className="text-[8px] tracking-wider text-white/25 uppercase">
                {def.label}
              </div>
              <div className="font-mono text-[11px] text-[#00d4aa]/90">
                {point ? formatMetricValue(point.value, unit) : "—"}
              </div>
              <div className="mt-0.5 font-mono text-[8px] text-white/15">
                {point ? formatLocalTime(point.ts) : ""}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function ExpandOverlay({
  onClose,
  latencySeries,
}: {
  onClose: () => void
  latencySeries: ChartSeries[]
}) {
  const sttSeries = latencySeries.filter((s) => s.id === "stt-first")
  const llmSeries = latencySeries.filter((s) => s.id === "llm-ttft")
  const ttsSeries = latencySeries.filter((s) => s.id === "tts-byte")
  const totalPoints = sttSeries.flatMap((s) => s.points).length +
    llmSeries.flatMap((s) => s.points).length +
    ttsSeries.flatMap((s) => s.points).length

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm">
      <div className="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden border border-[#1a1a1a] bg-[#0a0a0a]">
        <div className="flex items-center justify-between border-b border-[#1a1a1a] px-4 py-3">
          <h2 className="text-[11px] font-medium tracking-wider text-[#00d4aa] uppercase">
            Pipeline Metrics
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="flex h-7 w-7 items-center justify-center border border-[#1a1a1a] text-white/40 hover:border-[#00d4aa]/40 hover:text-[#00d4aa]"
            aria-label="Close metrics"
          >
            <X size={14} />
          </button>
        </div>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
          <ChartBlock
            title="STT First Transcript"
            series={sttSeries}
            footer={[
              { label: "Latest", value: latestValue(sttSeries[0]?.points ?? []) },
              { label: "Average", value: avgValue(sttSeries[0]?.points ?? []) },
              { label: "Min", value: minValue(sttSeries[0]?.points ?? []) },
            ]}
          />
          <ChartBlock
            title="LLM TTFT"
            series={llmSeries}
            footer={[
              { label: "Latest", value: latestValue(llmSeries[0]?.points ?? []) },
              { label: "Average", value: avgValue(llmSeries[0]?.points ?? []) },
              { label: "Min", value: minValue(llmSeries[0]?.points ?? []) },
            ]}
          />
          <ChartBlock
            title="TTS TTFB"
            series={ttsSeries}
            footer={[
              { label: "Latest", value: latestValue(ttsSeries[0]?.points ?? []) },
              { label: "Average", value: avgValue(ttsSeries[0]?.points ?? []) },
              { label: "Min", value: minValue(ttsSeries[0]?.points ?? []) },
            ]}
          />
        </div>

        <div className="border-t border-[#1a1a1a] px-4 py-2 text-[9px] tracking-wider text-white/20 uppercase">
          {totalPoints} data points captured
        </div>
      </div>
    </div>
  )
}

function ChartBlock({
  title,
  series,
  footer,
  formatAsInt = false,
}: {
  title: string
  series: ChartSeries[]
  footer: Array<{ label: string; value: number }>
  formatAsInt?: boolean
}) {
  const fmt = (v: number) =>
    formatAsInt ? Math.round(v).toString() : `${Math.round(v)} ms`

  return (
    <div className="border border-[#1a1a1a] bg-[#050505] p-3">
      <div className="mb-2 text-[10px] tracking-wider text-white/40 uppercase">
        {title}
      </div>
      <MetricsChart series={series} height={180} showGrid showLabels />
      <div className="mt-3 grid grid-cols-3 gap-3 border-t border-[#1a1a1a] pt-3">
        {footer.map((item) => (
          <div key={item.label}>
            <div className="text-[8px] tracking-wider text-white/25 uppercase">
              {item.label}
            </div>
            <div className="font-mono text-[14px] font-light text-white/90">
              {item.value > 0 ? fmt(item.value) : "—"}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function formatMetricValue(value: number, unit: string) {
  const rounded = Math.round(value)
  if (unit === "tok") return `${rounded} tok`
  if (unit === "ms") return `${rounded} ms`
  if (unit === "") return String(rounded)
  return `${rounded} ${unit}`
}

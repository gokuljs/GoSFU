import { useRef, useEffect, useState } from "react"
import { User } from "@phosphor-icons/react"
import { Orb, type AgentState } from "@/components/ui/orb"
import { ShimmeringText } from "@/components/ui/shimmering-text"

type TileType = "human" | "agent"

interface VideoTileProps {
  stream: MediaStream | null
  muted?: boolean
  label: string
  type?: TileType
}

export function VideoTile({
  stream,
  muted = false,
  label,
  type = "human",
}: VideoTileProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [agentState, setAgentState] = useState<AgentState>("listening")

  useEffect(() => {
    if (videoRef.current && stream) {
      videoRef.current.srcObject = stream
    }
  }, [stream])

  useEffect(() => {
    if (type !== "agent") return
    const states: AgentState[] = ["listening", "thinking", "talking", null]
    let idx = 0
    const interval = setInterval(() => {
      idx = (idx + 1) % states.length
      setAgentState(states[idx])
    }, 3000)
    return () => clearInterval(interval)
  }, [type])

  const stateLabel =
    agentState === "listening"
      ? "Listening..."
      : agentState === "thinking"
        ? "Processing..."
        : agentState === "talking"
          ? "Speaking..."
          : "Idle"

  const hasVideo = stream && stream.getVideoTracks().length > 0

  return (
    <div className="relative flex aspect-video w-full items-center justify-center overflow-hidden rounded-xl bg-black">
      {type === "human" && hasVideo ? (
        <video
          ref={videoRef}
          autoPlay
          playsInline
          muted={muted}
          className="h-full w-full object-cover"
        />
      ) : type === "human" ? (
        <div className="flex flex-col items-center gap-3">
          <div className="flex h-20 w-20 items-center justify-center rounded-full bg-white/5 sm:h-24 sm:w-24">
            <User className="h-10 w-10 text-white/25 sm:h-12 sm:w-12" />
          </div>
          <span className="text-xs text-white/25">You</span>
        </div>
      ) : (
        <div className="flex flex-col items-center gap-5">
          <div className="h-36 w-36 overflow-hidden rounded-full sm:h-40 sm:w-40 md:h-44 md:w-44">
            <Orb
              colors={["#6366f1", "#a78bfa"]}
              agentState={agentState}
              seed={42}
            />
          </div>
          <ShimmeringText
            text={stateLabel}
            className="text-sm font-medium"
            color="rgba(255, 255, 255, 0.4)"
            shimmerColor="rgba(255, 255, 255, 0.9)"
            duration={2}
            repeat={true}
            repeatDelay={0.5}
            spread={2}
          />
        </div>
      )}

      <div className="absolute bottom-3 left-3 rounded-md bg-black/50 px-2 py-1 text-[11px] font-medium text-white/50 backdrop-blur-sm">
        {label}
      </div>
    </div>
  )
}

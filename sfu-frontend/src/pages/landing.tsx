import { DitherText } from "@/components/dither-text"

interface LandingPageProps {
  onConnect: () => void
}

export function LandingPage({ onConnect }: LandingPageProps) {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center bg-[#050505] px-4">
      <div className="relative h-[200px] w-full max-w-[800px] sm:h-[260px] md:h-[320px]">
        <DitherText text="GO SFU" className="cursor-crosshair" />
      </div>

      <div className="mt-10 flex flex-col items-center gap-4 sm:mt-14">
        <button
          onClick={onConnect}
          className="relative h-10 cursor-pointer border border-[#00d4aa]/30 bg-[#00d4aa]/10 px-8 text-xs font-medium tracking-wider text-[#00d4aa] uppercase transition-all duration-300 hover:border-[#00d4aa]/60 hover:bg-[#00d4aa]/20 hover:shadow-[0_0_20px_rgba(0,212,170,0.15)] active:scale-[0.98]"
        >
          Connect
        </button>
        <p className="text-[10px] tracking-wide text-white/30 uppercase">
          Each connect creates a new room at /room/your-id
        </p>
      </div>
    </div>
  )
}

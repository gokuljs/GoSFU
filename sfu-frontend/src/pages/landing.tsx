import { DitherText } from "@/components/dither-text"

interface LandingPageProps {
  onConnect: () => void
}

export function LandingPage({ onConnect }: LandingPageProps) {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center bg-black px-4">
      <div className="relative h-[200px] w-full max-w-[800px] sm:h-[260px] md:h-[320px]">
        <DitherText text="GO SFU" className="cursor-crosshair" />
      </div>

      <div className="mt-10 flex flex-col items-center gap-4 sm:mt-14">
        <button
          onClick={onConnect}
          className="relative h-11 cursor-pointer rounded-full bg-linear-to-b from-white/90 to-white/70 px-10 text-sm font-semibold text-black shadow-[0_0_0_1px_rgba(255,255,255,0.1),0_2px_8px_rgba(0,0,0,0.4)] transition-all duration-300 hover:from-white hover:to-white/80 hover:shadow-[0_0_24px_rgba(255,255,255,0.12),0_0_0_1px_rgba(255,255,255,0.2)] active:scale-[0.97]"
        >
          Connect
        </button>
        <p className="text-xs text-white/40">
          Join the SFU room to start streaming
        </p>
      </div>
    </div>
  )
}

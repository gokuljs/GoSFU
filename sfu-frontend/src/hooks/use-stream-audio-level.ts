import { useEffect, useRef, useState } from "react"

const SPEAK_THRESHOLD = 0.06

export function useStreamAudioLevel(stream: MediaStream | null) {
  const levelRef = useRef(0)
  const [isSpeaking, setIsSpeaking] = useState(false)
  const [hasAudio, setHasAudio] = useState(false)

  useEffect(() => {
    if (!stream) {
      levelRef.current = 0
      return
    }

    let cancelled = false
    let raf = 0
    let audioContext: AudioContext | null = null
    let source: MediaStreamAudioSourceNode | null = null

    const updateHasAudio = () => {
      setHasAudio(stream.getAudioTracks().some((t) => t.readyState === "live"))
    }

    const setup = () => {
      source?.disconnect()
      source = null
      if (audioContext && audioContext.state !== "closed") {
        void audioContext.close()
      }
      audioContext = null
      cancelAnimationFrame(raf)

      if (stream.getAudioTracks().length === 0) {
        levelRef.current = 0
        setIsSpeaking(false)
        updateHasAudio()
        return
      }

      updateHasAudio()

      const AudioContextConstructor =
        window.AudioContext ||
        (window as unknown as { webkitAudioContext: typeof AudioContext })
          .webkitAudioContext
      audioContext = new AudioContextConstructor()
      void audioContext.resume()
      const analyser = audioContext.createAnalyser()
      analyser.fftSize = 256
      analyser.smoothingTimeConstant = 0.75

      source = audioContext.createMediaStreamSource(stream)
      source.connect(analyser)

      const data = new Uint8Array(analyser.frequencyBinCount)

      const tick = () => {
        if (cancelled) return
        analyser.getByteFrequencyData(data)
        let sum = 0
        for (let i = 0; i < data.length; i++) sum += data[i]
        const level = Math.min(1, (sum / data.length / 255) * 3)
        levelRef.current = level
        setIsSpeaking(level > SPEAK_THRESHOLD)
        raf = requestAnimationFrame(tick)
      }
      tick()
    }

    setup()
    stream.addEventListener("addtrack", setup)
    stream.addEventListener("removetrack", setup)

    return () => {
      cancelled = true
      cancelAnimationFrame(raf)
      stream.removeEventListener("addtrack", setup)
      stream.removeEventListener("removetrack", setup)
      source?.disconnect()
      if (audioContext && audioContext.state !== "closed") {
        void audioContext.close()
      }
    }
  }, [stream])

  return {
    levelRef,
    isSpeaking: stream ? isSpeaking : false,
    hasAudio: stream ? hasAudio : false,
  }
}

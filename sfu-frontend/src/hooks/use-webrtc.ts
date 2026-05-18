import { useRef, useState, useCallback } from "react"

const SFU_URL = "http://localhost:8080"
const ICE_SERVERS = [{ urls: "stun:stun.l.google.com:19302" }]

export type ConnectionState = "idle" | "connecting" | "connected" | "failed"

interface UseWebRTCReturn {
  localStream: MediaStream | null
  remoteStream: MediaStream | null
  connectionState: ConnectionState
  connect: () => Promise<void>
  disconnect: () => void
  toggleMic: () => void
  toggleCamera: () => void
  isMicOn: boolean
  isCameraOn: boolean
}

function waitForIceGathering(pc: RTCPeerConnection): Promise<void> {
  return new Promise((resolve) => {
    if (pc.iceGatheringState === "complete") {
      resolve()
      return
    }
    pc.addEventListener("icegatheringstatechange", () => {
      if (pc.iceGatheringState === "complete") {
        resolve()
      }
    })
  })
}

export function useWebRTC(): UseWebRTCReturn {
  const [localStream, setLocalStream] = useState<MediaStream | null>(null)
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null)
  const [connectionState, setConnectionState] =
    useState<ConnectionState>("idle")
  const [isMicOn, setIsMicOn] = useState(true)
  const [isCameraOn, setIsCameraOn] = useState(true)

  const pcRef = useRef<RTCPeerConnection | null>(null)
  const localStreamRef = useRef<MediaStream | null>(null)

  const connect = useCallback(async () => {
    try {
      setConnectionState("connecting")

      const stream = await navigator.mediaDevices.getUserMedia({
        video: true,
        audio: true,
      })
      setLocalStream(stream)
      localStreamRef.current = stream

      const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS })
      pcRef.current = pc

      const remote = new MediaStream()
      setRemoteStream(remote)

      pc.ontrack = (event) => {
        event.streams[0]?.getTracks().forEach((track) => {
          remote.addTrack(track)
        })
      }

      stream.getTracks().forEach((track) => {
        pc.addTrack(track, stream)
      })

      const offer = await pc.createOffer()
      await pc.setLocalDescription(offer)

      await waitForIceGathering(pc)

      const encoded = btoa(JSON.stringify(pc.localDescription))
      const res = await fetch(SFU_URL, {
        method: "POST",
        body: encoded,
      })

      if (!res.ok) {
        throw new Error(`Server responded with ${res.status}`)
      }

      const answerB64 = await res.text()
      if (answerB64 && answerB64 !== "done") {
        const answer = JSON.parse(atob(answerB64)) as RTCSessionDescriptionInit
        await pc.setRemoteDescription(answer)
      }

      setConnectionState("connected")
    } catch (err) {
      console.error("WebRTC connection failed:", err)
      setConnectionState("failed")
    }
  }, [])

  const disconnect = useCallback(() => {
    pcRef.current?.close()
    pcRef.current = null

    localStreamRef.current?.getTracks().forEach((track) => track.stop())
    localStreamRef.current = null

    setLocalStream(null)
    setRemoteStream(null)
    setConnectionState("idle")
    setIsMicOn(true)
    setIsCameraOn(true)
  }, [])

  const toggleMic = useCallback(() => {
    const stream = localStreamRef.current
    if (!stream) return
    const audioTrack = stream.getAudioTracks()[0]
    if (audioTrack) {
      audioTrack.enabled = !audioTrack.enabled
      setIsMicOn(audioTrack.enabled)
    }
  }, [])

  const toggleCamera = useCallback(() => {
    const stream = localStreamRef.current
    if (!stream) return
    const videoTrack = stream.getVideoTracks()[0]
    if (videoTrack) {
      videoTrack.enabled = !videoTrack.enabled
      setIsCameraOn(videoTrack.enabled)
    }
  }, [])

  return {
    localStream,
    remoteStream,
    connectionState,
    connect,
    disconnect,
    toggleMic,
    toggleCamera,
    isMicOn,
    isCameraOn,
  }
}

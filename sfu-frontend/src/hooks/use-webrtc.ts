import { useRef, useState, useCallback } from "react"

export const SFU_URL = "http://localhost:8080"
const ICE_SERVERS = [{ urls: "stun:stun.l.google.com:19302" }]
const AUDIO_CONSTRAINTS: MediaTrackConstraints = {
  echoCancellation: true,
  noiseSuppression: true,
  autoGainControl: true,
}

export type ConnectionState = "idle" | "connecting" | "connected" | "failed"
export type PeerConnectionStateValue = RTCPeerConnectionState | "idle"
export type IceConnectionStateValue = RTCIceConnectionState | "idle"

export interface SelectedDevices {
  audioInput: string
  videoInput: string
  audioOutput: string
}

interface UseWebRTCReturn {
  localStream: MediaStream | null
  remoteStream: MediaStream | null
  roomId: string | null
  participantId: string | null
  connectionState: ConnectionState
  peerConnectionState: PeerConnectionStateValue
  iceConnectionState: IceConnectionStateValue
  devices: MediaDeviceInfo[]
  selectedDevices: SelectedDevices
  createRoom: () => Promise<string>
  connect: (roomId: string, systemPrompt?: string) => Promise<void>
  disconnect: () => void
  stopSession: (roomId?: string) => Promise<void>
  deleteRoom: (roomId?: string) => Promise<void>
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
  const [roomId, setRoomId] = useState<string | null>(null)
  const [participantId, setParticipantId] = useState<string | null>(null)
  const [connectionState, setConnectionState] =
    useState<ConnectionState>("idle")
  const [peerConnectionState, setPeerConnectionState] =
    useState<PeerConnectionStateValue>("idle")
  const [iceConnectionState, setIceConnectionState] =
    useState<IceConnectionStateValue>("idle")
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [selectedDevices, setSelectedDevices] = useState<SelectedDevices>({
    audioInput: "",
    videoInput: "",
    audioOutput: "",
  })
  const [isMicOn, setIsMicOn] = useState(true)
  const [isCameraOn, setIsCameraOn] = useState(false)

  const pcRef = useRef<RTCPeerConnection | null>(null)
  const localStreamRef = useRef<MediaStream | null>(null)
  const connectGenerationRef = useRef(0)

  const refreshDevices = useCallback(async (stream?: MediaStream) => {
    const nextDevices = await navigator.mediaDevices.enumerateDevices()
    setDevices(nextDevices)

    const audioTrack = stream?.getAudioTracks()[0]
    const videoTrack = stream?.getVideoTracks()[0]
    const audioInputId = audioTrack?.getSettings().deviceId ?? ""
    const videoInputId = videoTrack?.getSettings().deviceId ?? ""
    const audioOutputId =
      nextDevices.find((device) => device.kind === "audiooutput")?.deviceId ?? ""

    setSelectedDevices({
      audioInput:
        nextDevices.find((device) => device.deviceId === audioInputId)?.label ||
        audioTrack?.label ||
        "Default microphone",
      videoInput:
        nextDevices.find((device) => device.deviceId === videoInputId)?.label ||
        videoTrack?.label ||
        "Default camera",
      audioOutput:
        nextDevices.find((device) => device.deviceId === audioOutputId)?.label ||
        "Default audio output",
    })
  }, [])

  const createRoom = useCallback(async () => {
    const createRes = await fetch(`${SFU_URL}/room/create`, {
      method: "POST",
    })
    if (!createRes.ok) {
      throw new Error(`Create failed: ${createRes.status}`)
    }
    const { roomId: newRoomId } = await createRes.json()
    return newRoomId as string
  }, [])

  const disconnect = useCallback(() => {
    connectGenerationRef.current += 1

    pcRef.current?.close()
    pcRef.current = null

    localStreamRef.current?.getTracks().forEach((track) => track.stop())
    localStreamRef.current = null

    setLocalStream(null)
    setRemoteStream(null)
    setRoomId(null)
    setParticipantId(null)
    setConnectionState("idle")
    setPeerConnectionState("idle")
    setIceConnectionState("idle")
    setIsMicOn(true)
    setIsCameraOn(false)
  }, [])

  const connect = useCallback(
    async (targetRoomId: string, systemPrompt?: string) => {
      const generation = ++connectGenerationRef.current
      disconnect()
      connectGenerationRef.current = generation

      try {
        setConnectionState("connecting")
        setRoomId(targetRoomId)

        const stream = await navigator.mediaDevices.getUserMedia({
          video: false,
          audio: AUDIO_CONSTRAINTS,
        })
        if (connectGenerationRef.current !== generation) {
          stream.getTracks().forEach((track) => track.stop())
          return
        }

        setLocalStream(stream)
        localStreamRef.current = stream
        await refreshDevices(stream)

        const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS })
        pcRef.current = pc
        setPeerConnectionState(pc.connectionState)
        setIceConnectionState(pc.iceConnectionState)

        const remote = new MediaStream()
        setRemoteStream(remote)

        pc.ontrack = (event) => {
          remote.addTrack(event.track)
          setRemoteStream(new MediaStream(remote.getTracks()))
        }

        pc.onconnectionstatechange = () => {
          if (connectGenerationRef.current !== generation) return
          setPeerConnectionState(pc.connectionState)
          if (pc.connectionState === "connected") {
            setConnectionState("connected")
          }
          if (
            pc.connectionState === "failed" ||
            pc.connectionState === "closed"
          ) {
            setConnectionState("failed")
          }
        }

        pc.oniceconnectionstatechange = () => {
          if (connectGenerationRef.current !== generation) return
          setIceConnectionState(pc.iceConnectionState)
        }

        stream.getTracks().forEach((track) => {
          pc.addTrack(track, stream)
        })

        const offer = await pc.createOffer()
        await pc.setLocalDescription(offer)
        await waitForIceGathering(pc)

        if (connectGenerationRef.current !== generation) {
          return
        }

        const joinRes = await fetch(`${SFU_URL}/room/${targetRoomId}/join`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            sdp: pc.localDescription,
            systemPrompt: systemPrompt?.trim() || undefined,
          }),
        })
        if (!joinRes.ok) {
          throw new Error(`Join failed: ${joinRes.status}`)
        }

        const { sdp: answer, participantId: pid } = await joinRes.json()
        if (connectGenerationRef.current !== generation) {
          return
        }

        setParticipantId(pid)
        await pc.setRemoteDescription(answer)

        setConnectionState(
          pc.connectionState === "connected" ? "connected" : "connecting"
        )
      } catch (err) {
        console.error("WebRTC connection failed:", err)
        if (connectGenerationRef.current === generation) {
          setConnectionState("failed")
          setPeerConnectionState("failed")
        }
      }
    },
    [disconnect, refreshDevices]
  )

  const stopSession = useCallback(
    async (targetRoomId = roomId ?? undefined) => {
      disconnect()
      if (!targetRoomId) return

      const stopRes = await fetch(`${SFU_URL}/room/${targetRoomId}/session/stop`, {
        method: "POST",
      })
      if (!stopRes.ok && stopRes.status !== 404) {
        throw new Error(`Stop session failed: ${stopRes.status}`)
      }
    },
    [disconnect, roomId]
  )

  const deleteRoom = useCallback(
    async (targetRoomId = roomId ?? undefined) => {
      disconnect()
      if (!targetRoomId) return

      const deleteRes = await fetch(`${SFU_URL}/room/${targetRoomId}`, {
        method: "DELETE",
      })
      if (!deleteRes.ok && deleteRes.status !== 404) {
        throw new Error(`Delete room failed: ${deleteRes.status}`)
      }
    },
    [disconnect, roomId]
  )

  const toggleMic = useCallback(() => {
    const stream = localStreamRef.current
    if (!stream) return
    const audioTrack = stream.getAudioTracks()[0]
    if (audioTrack) {
      audioTrack.enabled = !audioTrack.enabled
      setIsMicOn(audioTrack.enabled)
    }
  }, [])

  const toggleCamera = useCallback(async () => {
    const stream = localStreamRef.current
    const pc = pcRef.current
    if (!stream) return

    let videoTrack = stream.getVideoTracks()[0]
    if (!videoTrack) {
      try {
        const videoStream = await navigator.mediaDevices.getUserMedia({
          video: true,
        })
        videoTrack = videoStream.getVideoTracks()[0]
        stream.addTrack(videoTrack)
        pc?.addTrack(videoTrack, stream)
        setIsCameraOn(true)
        await refreshDevices(stream)
      } catch (err) {
        console.error("Camera failed:", err)
      }
      return
    }

    videoTrack.enabled = !videoTrack.enabled
    setIsCameraOn(videoTrack.enabled)
  }, [refreshDevices])

  return {
    localStream,
    remoteStream,
    roomId,
    participantId,
    connectionState,
    peerConnectionState,
    iceConnectionState,
    devices,
    selectedDevices,
    createRoom,
    connect,
    disconnect,
    stopSession,
    deleteRoom,
    toggleMic,
    toggleCamera,
    isMicOn,
    isCameraOn,
  }
}

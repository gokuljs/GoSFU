import { useRef, useEffect, useCallback } from "react"

interface DitherTextProps {
  text?: string
  className?: string
}

interface Particle {
  x: number
  y: number
  baseX: number
  baseY: number
  size: number
  opacity: number
}

export function DitherText({ text = "GO SFU", className }: DitherTextProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const particlesRef = useRef<Particle[]>([])
  const mouseRef = useRef({ x: -9999, y: -9999 })
  const animFrameRef = useRef<number>(0)
  const dimensionsRef = useRef({ width: 0, height: 0 })

  const createParticles = useCallback(
    (canvas: HTMLCanvasElement) => {
      const ctx = canvas.getContext("2d")
      if (!ctx) return

      const dpr = window.devicePixelRatio || 1
      const width = canvas.clientWidth
      const height = canvas.clientHeight

      canvas.width = width * dpr
      canvas.height = height * dpr
      ctx.scale(dpr, dpr)

      dimensionsRef.current = { width, height }

      const fontSize = Math.min(width * 0.2, height * 0.4, 180)
      ctx.fillStyle = "#00d4aa"
      ctx.font = `900 ${fontSize}px "Inter Variable", sans-serif`
      ctx.textAlign = "center"
      ctx.textBaseline = "middle"
      ctx.letterSpacing = `${fontSize * 0.08}px`
      ctx.fillText(text, width / 2, height / 2)

      const imageData = ctx.getImageData(0, 0, width * dpr, height * dpr)
      const pixels = imageData.data

      const particles: Particle[] = []
      const gap = Math.max(3, Math.floor(fontSize / 28))

      for (let y = 0; y < height * dpr; y += gap * dpr) {
        for (let x = 0; x < width * dpr; x += gap * dpr) {
          const i = (y * width * dpr + x) * 4
          const alpha = pixels[i + 3]
          if (alpha > 128) {
            const px = x / dpr
            const py = y / dpr
            particles.push({
              x: px,
              y: py,
              baseX: px,
              baseY: py,
              size: Math.random() * 1.5 + 0.8,
              opacity: 0.4 + Math.random() * 0.6,
            })
          }
        }
      }

      particlesRef.current = particles
      ctx.clearRect(0, 0, width, height)
    },
    [text]
  )

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    createParticles(canvas)

    function render() {
      const ctx = canvas!.getContext("2d")
      if (!ctx) return

      const { width, height } = dimensionsRef.current
      const dpr = window.devicePixelRatio || 1

      ctx.setTransform(1, 0, 0, 1, 0, 0)
      ctx.clearRect(0, 0, width * dpr, height * dpr)
      ctx.scale(dpr, dpr)

      const mouse = mouseRef.current
      const particles = particlesRef.current
      const radius = 100
      const forceMultiplier = 8

      for (let i = 0; i < particles.length; i++) {
        const p = particles[i]

        const dx = mouse.x - p.baseX
        const dy = mouse.y - p.baseY
        const dist = Math.sqrt(dx * dx + dy * dy)

        if (dist < radius) {
          const force = (radius - dist) / radius
          const angle = Math.atan2(dy, dx)
          const pushX = Math.cos(angle) * force * forceMultiplier
          const pushY = Math.sin(angle) * force * forceMultiplier

          p.x += (-pushX - (p.x - p.baseX)) * 0.1
          p.y += (-pushY - (p.y - p.baseY)) * 0.1
        } else {
          p.x += (p.baseX - p.x) * 0.08
          p.y += (p.baseY - p.y) * 0.08
        }

        ctx.beginPath()
        ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
        ctx.fillStyle = `rgba(0, 212, 170, ${p.opacity})`
        ctx.fill()
      }

      animFrameRef.current = requestAnimationFrame(render)
    }

    animFrameRef.current = requestAnimationFrame(render)

    const resizeObserver = new ResizeObserver(() => {
      createParticles(canvas)
    })
    resizeObserver.observe(canvas)

    return () => {
      cancelAnimationFrame(animFrameRef.current)
      resizeObserver.disconnect()
    }
  }, [createParticles])

  const handleMouseMove = useCallback(
    (e: React.MouseEvent<HTMLCanvasElement>) => {
      const rect = e.currentTarget.getBoundingClientRect()
      mouseRef.current = {
        x: e.clientX - rect.left,
        y: e.clientY - rect.top,
      }
    },
    []
  )

  const handleMouseLeave = useCallback(() => {
    mouseRef.current = { x: -9999, y: -9999 }
  }, [])

  const handleTouchMove = useCallback(
    (e: React.TouchEvent<HTMLCanvasElement>) => {
      const rect = e.currentTarget.getBoundingClientRect()
      const touch = e.touches[0]
      mouseRef.current = {
        x: touch.clientX - rect.left,
        y: touch.clientY - rect.top,
      }
    },
    []
  )

  return (
    <canvas
      ref={canvasRef}
      className={className}
      onMouseMove={handleMouseMove}
      onMouseLeave={handleMouseLeave}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleMouseLeave}
      style={{ width: "100%", height: "100%" }}
    />
  )
}

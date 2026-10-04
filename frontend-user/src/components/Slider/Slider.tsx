import { useRef, useCallback, useState, useEffect } from 'react'
import './Slider.css'

interface Props {
  min: number
  max: number
  step?: number
  value: number
  onChange: (v: number) => void
  unit?: string
  marks?: number[]
  tiers?: number[]
}

export function Slider({ min, max, step = 1, value, onChange, unit = 'GB', marks, tiers }: Props) {
  const trackRef = useRef<HTMLDivElement>(null)
  const draggingRef = useRef(false)
  const [dragging, setDragging] = useState(false)

  const percent = ((value - min) / (max - min)) * 100

  const snapToTiers = useCallback((v: number): number => {
    if (!tiers || tiers.length === 0) return v
    let closest = tiers[0]
    let minDiff = Math.abs(v - closest)
    for (const t of tiers) {
      const diff = Math.abs(v - t)
      if (diff < minDiff) {
        minDiff = diff
        closest = t
      }
    }
    return closest
  }, [tiers])

  const updateFromEvent = useCallback((clientX: number) => {
    const track = trackRef.current
    if (!track) return
    const rect = track.getBoundingClientRect()
    let p = (clientX - rect.left) / rect.width
    p = Math.max(0, Math.min(1, p))
    let v = min + p * (max - min)
    if (tiers && tiers.length > 0) {
      v = snapToTiers(v)
    } else {
      v = Math.round(v / step) * step
      v = Math.max(min, Math.min(max, v))
    }
    onChange(v)
  }, [min, max, step, onChange, tiers, snapToTiers])

  useEffect(() => {
    if (!dragging) return

    const onMove = (e: MouseEvent) => {
      e.preventDefault()
      updateFromEvent(e.clientX)
    }
    const onUp = () => {
      draggingRef.current = false
      setDragging(false)
    }
    const onTouchMove = (e: TouchEvent) => {
      e.preventDefault()
      updateFromEvent(e.touches[0].clientX)
    }
    const onTouchEnd = () => {
      draggingRef.current = false
      setDragging(false)
    }

    document.addEventListener('mousemove', onMove)
    document.addEventListener('mouseup', onUp)
    document.addEventListener('touchmove', onTouchMove, { passive: false })
    document.addEventListener('touchend', onTouchEnd)

    return () => {
      document.removeEventListener('mousemove', onMove)
      document.removeEventListener('mouseup', onUp)
      document.removeEventListener('touchmove', onTouchMove)
      document.removeEventListener('touchend', onTouchEnd)
    }
  }, [dragging, updateFromEvent])

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault()
    draggingRef.current = true
    setDragging(true)
    updateFromEvent(e.clientX)
  }

  const handleTouchStart = (e: React.TouchEvent) => {
    draggingRef.current = true
    setDragging(true)
    updateFromEvent(e.touches[0].clientX)
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    let v = Number(e.target.value)
    if (isNaN(v)) v = min
    v = Math.max(min, Math.min(max, v))
    if (tiers && tiers.length > 0) {
      v = snapToTiers(v)
    }
    onChange(v)
  }

  const handleInputBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    let v = Number(e.target.value)
    if (isNaN(v)) v = min
    v = Math.max(min, Math.min(max, v))
    if (tiers && tiers.length > 0) {
      v = snapToTiers(v)
    } else {
      v = Math.round(v / step) * step
    }
    onChange(v)
  }

  const stepDown = () => {
    if (tiers && tiers.length > 0) {
      const idx = tiers.findIndex(t => t >= value)
      const target = idx > 0 ? tiers[idx - 1] : tiers[0]
      onChange(target)
    } else {
      onChange(Math.max(min, Math.round((value - step) / step) * step))
    }
  }

  const stepUp = () => {
    if (tiers && tiers.length > 0) {
      const idx = tiers.findIndex(t => t > value)
      const target = idx >= 0 && idx < tiers.length ? tiers[idx] : tiers[tiers.length - 1]
      onChange(target)
    } else {
      onChange(Math.min(max, Math.round((value + step) / step) * step))
    }
  }

  return (
    <div className="sky-config-slider-container">
      <div className="progress-content">
        <div
          ref={trackRef}
          className="sky-config-progress-bar"
          onMouseDown={handleMouseDown}
          onTouchStart={handleTouchStart}
        >
          <div className="sky-config-progress" style={{ width: `${percent}%` }} />
          <div
            className={`sky-config-slider ${dragging ? 'dragging' : ''}`}
            style={{ left: `${percent}%` }}
            onMouseDown={handleMouseDown}
            onTouchStart={handleTouchStart}
          >
            <div className="sky-config-tooltip">{value}{unit}</div>
          </div>
        </div>
        <div className="sky-config-value-info">
          <span className="min-value">{min}{unit}</span>
          <span className="max-value">{max}{unit}</span>
        </div>
      </div>
      <div className="progress-toolbar">
        <div className="sky-config-decrease" onClick={stepDown}>-</div>
        <input
          type="number"
          className="input-value"
          value={value}
          onChange={handleInputChange}
          onBlur={handleInputBlur}
        />
        <div className="sky-config-increase" onClick={stepUp}>+</div>
      </div>
    </div>
  )
}

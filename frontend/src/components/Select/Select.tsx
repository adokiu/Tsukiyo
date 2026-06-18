import { useState, useRef, useEffect } from 'react'
import { ChevronDown, Check } from 'lucide-react'
import './Select.css'

export interface SelectOption {
  label: string
  value: string | number
}

interface SelectProps {
  value: string | number | undefined
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  onChange: (value: string | number) => void
}

export function Select({ value, options, placeholder = '请选择', disabled = false, onChange }: SelectProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const selected = options.find((o) => o.value === value)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  return (
    <div ref={ref} className={`select ${open ? 'select--open' : ''} ${disabled ? 'select--disabled' : ''}`}>
      <button
        type="button"
        className="select__trigger"
        onClick={() => !disabled && setOpen(!open)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className={selected ? 'select__value' : 'select__placeholder'}>
          {selected ? selected.label : placeholder}
        </span>
        <ChevronDown size={16} className={`select__arrow ${open ? 'select__arrow--up' : ''}`} />
      </button>

      {open && (
        <div className="select__dropdown" role="listbox">
          <div className="select__list">
            {options.map((opt) => (
              <div
                key={opt.value}
                role="option"
                aria-selected={value === opt.value}
                className={`select__option ${value === opt.value ? 'select__option--active' : ''}`}
                onClick={() => {
                  onChange(opt.value)
                  setOpen(false)
                }}
              >
                <span>{opt.label}</span>
                {value === opt.value && <Check size={16} />}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

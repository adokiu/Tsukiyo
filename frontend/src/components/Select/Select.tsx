import { useState, useRef, useEffect, useLayoutEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
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
  editable?: boolean
  emptyText?: string
  onChange: (value: string | number) => void
}

export function Select({ value, options, placeholder = '请选择', disabled = false, editable = false, emptyText, onChange }: SelectProps) {
  const [open, setOpen] = useState(false)
  const [inputValue, setInputValue] = useState<string>('')
  const ref = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const [dropdownStyle, setDropdownStyle] = useState<React.CSSProperties>({})

  const selected = options.find((o) => o.value === value)

  useEffect(() => {
    const sel = options.find((o) => o.value === value)
    setInputValue(sel ? sel.label : (value != null ? String(value) : ''))
  }, [value, options])

  const updatePosition = useCallback(() => {
    if (!ref.current) return
    const rect = ref.current.getBoundingClientRect()
    setDropdownStyle({
      position: 'fixed',
      top: rect.bottom + 4,
      left: rect.left,
      width: rect.width,
      zIndex: 99999,
    })
  }, [])

  useLayoutEffect(() => {
    if (!open) return
    const raf = requestAnimationFrame(updatePosition)
    return () => cancelAnimationFrame(raf)
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      const target = e.target as Node
      if (ref.current && ref.current.contains(target)) return
      if (dropdownRef.current && dropdownRef.current.contains(target)) return
      setOpen(false)
    }
    document.addEventListener('mousedown', handler, true)
    return () => document.removeEventListener('mousedown', handler, true)
  }, [open])

  useEffect(() => {
    if (!open) return
    const onScroll = () => updatePosition()
    const onResize = () => updatePosition()
    window.addEventListener('scroll', onScroll, true)
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('scroll', onScroll, true)
      window.removeEventListener('resize', onResize)
    }
  }, [open, updatePosition])

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputValue(e.target.value)
    onChange(e.target.value)
  }

  const handleInputFocus = () => {
    if (editable) setOpen(true)
  }

  return (
    <div ref={ref} className={`select ${open ? 'select--open' : ''} ${disabled ? 'select--disabled' : ''} ${editable ? 'select--editable' : ''}`}>
      {editable ? (
        <div className="select__trigger">
          <input
            ref={inputRef}
            type="text"
            className="select__input"
            value={inputValue}
            placeholder={placeholder}
            disabled={disabled}
            onChange={handleInputChange}
            onFocus={handleInputFocus}
          />
          <ChevronDown size={16} className={`select__arrow ${open ? 'select__arrow--up' : ''}`} onClick={() => setOpen(!open)} />
        </div>
      ) : (
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
      )}

      {open && createPortal(
        <div ref={dropdownRef} className="select__dropdown" role="listbox" style={dropdownStyle}>
          <div className="select__list">
            {options.length === 0 ? (
              <div className="select__empty">{emptyText || '无选项'}</div>
            ) : (
              options.map((opt) => (
                <div
                  key={opt.value}
                  role="option"
                  aria-selected={value === opt.value}
                  className={`select__option ${value === opt.value ? 'select__option--active' : ''}`}
                  onClick={() => {
                    onChange(opt.value)
                    setInputValue(String(opt.label))
                    setOpen(false)
                  }}
                >
                  <span>{opt.label}</span>
                  {value === opt.value && <Check size={16} />}
                </div>
              ))
            )}
          </div>
        </div>,
        document.body
      )}
    </div>
  )
}

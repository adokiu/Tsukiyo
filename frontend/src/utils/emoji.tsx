import React from 'react'

const REGIONAL_INDICATOR_A = 0x1F1E6
const REGIONAL_INDICATOR_Z = 0x1F1FF

function isRegionalIndicator(code: number): boolean {
  return code >= REGIONAL_INDICATOR_A && code <= REGIONAL_INDICATOR_Z
}

function regionalIndicatorToLetter(code: number): string {
  return String.fromCharCode('A'.charCodeAt(0) + (code - REGIONAL_INDICATOR_A))
}

export function renderTextWithFlags(text: string): React.ReactNode[] {
  if (!text) return [text]

  const codePoints: number[] = []
  for (const ch of text) {
    codePoints.push(ch.codePointAt(0)!)
  }

  const result: React.ReactNode[] = []
  let buffer = ''
  let i = 0

  const flushBuffer = () => {
    if (buffer) {
      result.push(buffer)
      buffer = ''
    }
  }

  while (i < codePoints.length) {
    const cp = codePoints[i]

    if (isRegionalIndicator(cp) && i + 1 < codePoints.length && isRegionalIndicator(codePoints[i + 1])) {
      flushBuffer()
      const code = regionalIndicatorToLetter(cp) + regionalIndicatorToLetter(codePoints[i + 1])
      result.push(
        <img
          key={`flag-${i}-${code}`}
          src={`https://flagcdn.com/${code.toLowerCase()}.svg`}
          alt={code}
          style={{ width: '22px', height: '16px', display: 'inline-block', verticalAlign: '-3px', objectFit: 'cover', borderRadius: '3px', flexShrink: 0, marginRight: '4px' }}
        />
      )
      i += 2
    } else {
      buffer += String.fromCodePoint(cp)
      i++
    }
  }

  flushBuffer()
  return result
}

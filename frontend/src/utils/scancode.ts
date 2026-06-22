export interface ScancodeMapping {
  make: number
  shift: boolean
}

const scancodeMap: Record<string, ScancodeMapping> = {
  'a': { make: 30, shift: false }, 'b': { make: 48, shift: false }, 'c': { make: 46, shift: false },
  'd': { make: 32, shift: false }, 'e': { make: 18, shift: false }, 'f': { make: 33, shift: false },
  'g': { make: 34, shift: false }, 'h': { make: 35, shift: false }, 'i': { make: 23, shift: false },
  'j': { make: 36, shift: false }, 'k': { make: 37, shift: false }, 'l': { make: 38, shift: false },
  'm': { make: 50, shift: false }, 'n': { make: 49, shift: false }, 'o': { make: 24, shift: false },
  'p': { make: 25, shift: false }, 'q': { make: 16, shift: false }, 'r': { make: 19, shift: false },
  's': { make: 31, shift: false }, 't': { make: 20, shift: false }, 'u': { make: 22, shift: false },
  'v': { make: 47, shift: false }, 'w': { make: 17, shift: false }, 'x': { make: 45, shift: false },
  'y': { make: 21, shift: false }, 'z': { make: 44, shift: false },
  'A': { make: 30, shift: true }, 'B': { make: 48, shift: true }, 'C': { make: 46, shift: true },
  'D': { make: 32, shift: true }, 'E': { make: 18, shift: true }, 'F': { make: 33, shift: true },
  'G': { make: 34, shift: true }, 'H': { make: 35, shift: true }, 'I': { make: 23, shift: true },
  'J': { make: 36, shift: true }, 'K': { make: 37, shift: true }, 'L': { make: 38, shift: true },
  'M': { make: 50, shift: true }, 'N': { make: 49, shift: true }, 'O': { make: 24, shift: true },
  'P': { make: 25, shift: true }, 'Q': { make: 16, shift: true }, 'R': { make: 19, shift: true },
  'S': { make: 31, shift: true }, 'T': { make: 20, shift: true }, 'U': { make: 22, shift: true },
  'V': { make: 47, shift: true }, 'W': { make: 17, shift: true }, 'X': { make: 45, shift: true },
  'Y': { make: 21, shift: true }, 'Z': { make: 44, shift: true },
  '0': { make: 11, shift: false }, '1': { make: 2, shift: false }, '2': { make: 3, shift: false },
  '3': { make: 4, shift: false }, '4': { make: 5, shift: false }, '5': { make: 6, shift: false },
  '6': { make: 7, shift: false }, '7': { make: 8, shift: false }, '8': { make: 9, shift: false },
  '9': { make: 10, shift: false },
  ')': { make: 11, shift: true }, '!': { make: 2, shift: true }, '@': { make: 3, shift: true },
  '#': { make: 4, shift: true }, '$': { make: 5, shift: true }, '%': { make: 6, shift: true },
  '^': { make: 7, shift: true }, '&': { make: 8, shift: true }, '*': { make: 9, shift: true },
  '(': { make: 10, shift: true },
  '-': { make: 12, shift: false }, '_': { make: 12, shift: true },
  '=': { make: 13, shift: false }, '+': { make: 13, shift: true },
  '[': { make: 26, shift: false }, '{': { make: 26, shift: true },
  ']': { make: 27, shift: false }, '}': { make: 27, shift: true },
  ';': { make: 39, shift: false }, ':': { make: 39, shift: true },
  "'": { make: 40, shift: false }, '"': { make: 40, shift: true },
  '`': { make: 41, shift: false }, '~': { make: 41, shift: true },
  '\\': { make: 43, shift: false }, '|': { make: 43, shift: true },
  ',': { make: 51, shift: false }, '<': { make: 51, shift: true },
  '.': { make: 52, shift: false }, '>': { make: 52, shift: true },
  '/': { make: 53, shift: false }, '?': { make: 53, shift: true },
  ' ': { make: 57, shift: false },
  '\n': { make: 28, shift: false },
  '\t': { make: 15, shift: false },
}

export function charToScancode(ch: string): ScancodeMapping | null {
  return scancodeMap[ch] ?? null
}

const numpadCodeMap: Record<number, string> = {
  96: 'Numpad0', 97: 'Numpad1', 98: 'Numpad2', 99: 'Numpad3',
  100: 'Numpad4', 101: 'Numpad5', 102: 'Numpad6', 103: 'Numpad7',
  104: 'Numpad8', 105: 'Numpad9', 106: 'NumpadMultiply',
  107: 'NumpadAdd', 109: 'NumpadSubtract', 110: 'NumpadDecimal',
  111: 'NumpadDivide', 13: 'NumpadEnter',
}

export function numpadCodeFix(e: KeyboardEvent): string {
  if (e.location === 3 || (e.keyCode >= 96 && e.keyCode <= 111)) {
    const fixed = numpadCodeMap[e.keyCode]
    if (fixed) return fixed
  }
  return e.code
}

import type { Plugin } from 'vite'
import { createWriteStream, readFileSync, existsSync } from 'fs'
import { writeFile, mkdir } from 'fs/promises'
import * as path from 'path'
import * as zlib from 'zlib'

// Vite 插件：构建后将 dist 目录和 tsukiyo-theme.json 打包为 tsukiyo-theme.zip
export function archiverPlugin(): Plugin {
  return {
    name: 'theme-pack',
    apply: 'build',
    closeBundle: async () => {
      const distDir = path.resolve(process.cwd(), 'dist')
      const themeConfigPath = path.resolve(process.cwd(), 'tsukiyo-theme.json')
      const outputPath = path.resolve(process.cwd(), 'tsukiyo-theme.zip')

      if (!existsSync(distDir)) {
        console.warn('[theme-pack] dist 目录不存在，跳过打包')
        return
      }

      // 简单 ZIP 打包实现（不依赖外部库）
      const entries: ZipEntry[] = []

      // 添加 dist 目录下所有文件
      await addDirectoryEntries(entries, distDir, 'dist')

      // 添加 tsukiyo-theme.json
      if (existsSync(themeConfigPath)) {
        const content = readFileSync(themeConfigPath)
        entries.push({
          filename: 'tsukiyo-theme.json',
          content,
        })
      }

      // 写入 ZIP 文件
      await writeZipFile(outputPath, entries)
      console.log(`[theme-pack] 主题包已生成: ${outputPath}`)
    },
  }
}

interface ZipEntry {
  filename: string
  content: Buffer
}

async function addDirectoryEntries(entries: ZipEntry[], dir: string, prefix: string) {
  const { readdirSync, statSync } = await import('fs')
  const items = readdirSync(dir)
  for (const item of items) {
    const fullPath = path.join(dir, item)
    const relativePath = path.join(prefix, item)
    const stat = statSync(fullPath)
    if (stat.isDirectory()) {
      await addDirectoryEntries(entries, fullPath, relativePath)
    } else {
      const content = readFileSync(fullPath)
      entries.push({
        filename: relativePath.replace(/\\/g, '/'),
        content,
      })
    }
  }
}

async function writeZipFile(outputPath: string, entries: ZipEntry[]) {
  const dir = path.dirname(outputPath)
  if (!existsSync(dir)) {
    await mkdir(dir, { recursive: true })
  }

  const stream = createWriteStream(outputPath)
  const chunks: Buffer[] = []

  for (const entry of entries) {
    const header = createLocalFileHeader(entry.filename, entry.content)
    chunks.push(header)
    chunks.push(entry.content)

    // CRC32
    const crc = crc32(entry.content)
    chunks.push(createDataDescriptor(crc, entry.content.length, entry.content.length))
  }

  // Central directory
  const centralDirChunks: Buffer[] = []
  let offset = 0
  for (const entry of entries) {
    const contentLength = entry.content.length
    const crc = crc32(entry.content)

    centralDirChunks.push(createCentralDirHeader(entry.filename, crc, contentLength, contentLength, offset))
    offset += createLocalFileHeader(entry.filename, entry.content).length + contentLength + 16 // data descriptor size
  }

  const centralDir = Buffer.concat(centralDirChunks)
  chunks.push(centralDir)

  // End of central directory
  const eocd = createEndOfCentralDir(entries.length, centralDir.length, offset)
  chunks.push(eocd)

  const result = Buffer.concat(chunks)
  stream.write(result)
  stream.end()
}

function createLocalFileHeader(filename: string, content: Buffer): Buffer {
  const filenameBuf = Buffer.from(filename, 'utf8')
  const header = Buffer.alloc(30)
  header.writeUInt32LE(0x04034b50, 0) // signature
  header.writeUInt16LE(20, 4) // version needed
  header.writeUInt16LE(0, 6) // flags
  header.writeUInt16LE(0, 8) // compression method (0 = stored)
  header.writeUInt16LE(0, 10) // mod time
  header.writeUInt16LE(0, 12) // mod date
  header.writeUInt32LE(0, 14) // crc32 (in data descriptor)
  header.writeUInt32LE(0, 18) // compressed size (in data descriptor)
  header.writeUInt32LE(0, 22) // uncompressed size (in data descriptor)
  header.writeUInt16LE(filenameBuf.length, 26) // filename length
  header.writeUInt16LE(0, 28) // extra field length
  return Buffer.concat([header, filenameBuf])
}

function createDataDescriptor(crc: number, compressedSize: number, uncompressedSize: number): Buffer {
  const desc = Buffer.alloc(16)
  desc.writeUInt32LE(0x08074b50, 0) // signature
  desc.writeUInt32LE(crc, 4) // crc32
  desc.writeUInt32LE(compressedSize, 8) // compressed size
  desc.writeUInt32LE(uncompressedSize, 12) // uncompressed size
  return desc
}

function createCentralDirHeader(filename: string, crc: number, compressedSize: number, uncompressedSize: number, offset: number): Buffer {
  const filenameBuf = Buffer.from(filename, 'utf8')
  const header = Buffer.alloc(46)
  header.writeUInt32LE(0x02014b50, 0) // signature
  header.writeUInt16LE(20, 4) // version made by
  header.writeUInt16LE(20, 6) // version needed
  header.writeUInt16LE(0, 8) // flags
  header.writeUInt16LE(0, 10) // compression method
  header.writeUInt16LE(0, 12) // mod time
  header.writeUInt16LE(0, 14) // mod date
  header.writeUInt32LE(crc, 16) // crc32
  header.writeUInt32LE(compressedSize, 20) // compressed size
  header.writeUInt32LE(uncompressedSize, 24) // uncompressed size
  header.writeUInt16LE(filenameBuf.length, 28) // filename length
  header.writeUInt16LE(0, 30) // extra field length
  header.writeUInt16LE(0, 32) // comment length
  header.writeUInt16LE(0, 34) // disk number start
  header.writeUInt16LE(0, 36) // internal attributes
  header.writeUInt32LE(0, 38) // external attributes
  header.writeUInt32LE(offset, 42) // relative offset of local header
  return Buffer.concat([header, filenameBuf])
}

function createEndOfCentralDir(entryCount: number, centralDirSize: number, centralDirOffset: number): Buffer {
  const eocd = Buffer.alloc(22)
  eocd.writeUInt32LE(0x06054b50, 0) // signature
  eocd.writeUInt16LE(0, 4) // disk number
  eocd.writeUInt16LE(0, 6) // disk with central dir
  eocd.writeUInt16LE(entryCount, 8) // entries on this disk
  eocd.writeUInt16LE(entryCount, 10) // total entries
  eocd.writeUInt32LE(centralDirSize, 12) // central dir size
  eocd.writeUInt32LE(centralDirOffset, 16) // central dir offset
  eocd.writeUInt16LE(0, 20) // comment length
  return eocd
}

// CRC32 计算
const crcTable = (() => {
  const table = new Uint32Array(256)
  for (let i = 0; i < 256; i++) {
    let crc = i
    for (let j = 0; j < 8; j++) {
      if (crc & 1) {
        crc = (crc >>> 1) ^ 0xedb88320
      } else {
        crc = crc >>> 1
      }
    }
    table[i] = crc
  }
  return table
})()

function crc32(buf: Buffer): number {
  let crc = 0xffffffff
  for (let i = 0; i < buf.length; i++) {
    crc = (crc >>> 8) ^ crcTable[(crc ^ buf[i]) & 0xff]
  }
  return (crc ^ 0xffffffff) >>> 0
}

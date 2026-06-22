import { InvalidInputError } from '../errors.js';
import { crc32ieee } from './crc32.js';
import { murmur3x64_128 } from './murmur3.js';

export const HEADER_SIZE = 64;
export const SECTION_ENTRY_SIZE = 17;
// CURRENT_MAJOR is 2: the tag-index key is type-qualified (t:{tag}:{type}:{id}).
// The `major > CURRENT_MAJOR` gate in decodeHeader deliberately keeps major 1
// readable (read-compat) — its only job is to make an old reader reject a new
// major-2 file. Writers always emit major 2, so files self-upgrade on compaction.
export const CURRENT_MAJOR = 2;
export const CURRENT_MINOR = 0;
export const HEADER_CHECKSUM_OFF = 55;
export const CHECKSUM_CRC32 = 0x01;
export const SECTION_DATA = 0x01;
export const SECTION_BLOOM = 0x02;
export const SECTION_WAL = 0x03;
export const BLOOM_BITS_PER_KEY = 10;
export const BLOOM_HASH_COUNT = 7;
export const BLOOM_HASH_SEED = 0;

const MAGIC = new Uint8Array([0x41, 0x4b, 0x47, 0x00]); // AKG\0

export interface Section {
  type: number;
  offset: bigint;
  length: bigint;
}

export interface Container {
  // major is the binary major from the file header. Read-side validation needs
  // it to re-derive a major-1 file's legacy 3-part tag keys (read-compat).
  major: number;
  data: Uint8Array;
  bloom: Uint8Array | null;
  wal: Uint8Array | null;
}

export interface DataEntry {
  key: Uint8Array;
  value: Uint8Array;
}

// ---- Header ----------------------------------------------------------------

function headerChecksum(buf: Uint8Array): number {
  const copy = new Uint8Array(buf);
  copy[HEADER_CHECKSUM_OFF] = 0;
  copy[HEADER_CHECKSUM_OFF + 1] = 0;
  copy[HEADER_CHECKSUM_OFF + 2] = 0;
  copy[HEADER_CHECKSUM_OFF + 3] = 0;
  return crc32ieee(copy.slice(0, HEADER_SIZE));
}

function encodeHeader(sectionCount: number): Uint8Array {
  const buf = new Uint8Array(HEADER_SIZE);
  buf.set(MAGIC, 0);
  buf[4] = CURRENT_MAJOR;
  buf[5] = CURRENT_MINOR;
  buf[6] = CHECKSUM_CRC32;
  writeUint32LE(buf, 7, sectionCount);
  const ck = headerChecksum(buf);
  writeUint32LE(buf, HEADER_CHECKSUM_OFF, ck);
  return buf;
}

function decodeHeader(file: Uint8Array): number {
  if (file.length < HEADER_SIZE) throw new InvalidInputError('invalid header: too short');
  for (let i = 0; i < 4; i++) {
    if (file[i] !== MAGIC[i]) throw new InvalidInputError('invalid header: bad magic');
  }
  for (let i = 11; i < 55; i++) {
    if (file[i] !== 0) throw new InvalidInputError('invalid header: reserved bytes non-zero');
  }
  for (let i = 59; i < 64; i++) {
    if (file[i] !== 0) throw new InvalidInputError('invalid header: reserved bytes non-zero');
  }
  const storedCk = readUint32LE(file, HEADER_CHECKSUM_OFF);
  const computedCk = headerChecksum(file);
  if (storedCk !== computedCk) throw new InvalidInputError('checksum mismatch');
  const major = file[4];
  const checksumAlgo = file[6];
  if (major > CURRENT_MAJOR || checksumAlgo !== CHECKSUM_CRC32) {
    throw new InvalidInputError('invalid header: unsupported version or checksum algorithm');
  }
  return readUint32LE(file, 7);
}

// ---- Section table ---------------------------------------------------------

function encodeSectionTable(sections: Section[]): Uint8Array {
  const buf = new Uint8Array(sections.length * SECTION_ENTRY_SIZE);
  for (let i = 0; i < sections.length; i++) {
    const off = i * SECTION_ENTRY_SIZE;
    buf[off] = sections[i].type;
    writeUint64LE(buf, off + 1, sections[i].offset);
    writeUint64LE(buf, off + 9, sections[i].length);
  }
  return buf;
}

function decodeSectionTable(buf: Uint8Array, count: number): Section[] {
  const need = count * SECTION_ENTRY_SIZE;
  if (buf.length !== need) throw new InvalidInputError('invalid section table');
  const sections: Section[] = [];
  for (let i = 0; i < count; i++) {
    const off = i * SECTION_ENTRY_SIZE;
    sections.push({
      type: buf[off],
      offset: readUint64LE_big(buf, off + 1),
      length: readUint64LE_big(buf, off + 9),
    });
  }
  return sections;
}

function validateSections(sections: Section[], fileSize: bigint): void {
  let dataCount = 0, bloomCount = 0, walCount = 0;
  for (const s of sections) {
    switch (s.type) {
      case SECTION_DATA:
        dataCount++;
        if (s.length < 4n) throw new InvalidInputError('invalid section table: data section too short');
        break;
      case SECTION_BLOOM:
        bloomCount++;
        if (s.length <= 4n) throw new InvalidInputError('invalid section table: bloom section too short');
        break;
      case SECTION_WAL:
        walCount++;
        if (s.length !== 0n && s.length < 4n) throw new InvalidInputError('invalid section table: wal section too short');
        break;
    }
    if (s.offset > fileSize || s.length > fileSize - s.offset) {
      throw new InvalidInputError('invalid section ranges: out of bounds');
    }
  }
  if (dataCount !== 1 || bloomCount > 1 || walCount > 1) {
    throw new InvalidInputError('invalid section table: wrong section counts');
  }
  const sorted = [...sections].sort((a, b) => (a.offset < b.offset ? -1 : a.offset > b.offset ? 1 : 0));
  for (let i = 1; i < sorted.length; i++) {
    if (sorted[i - 1].offset + sorted[i - 1].length > sorted[i].offset) {
      throw new InvalidInputError('invalid section ranges: sections overlap');
    }
  }
}

// ---- Section encode/decode -------------------------------------------------

function encodeSection(payload: Uint8Array): Uint8Array {
  const ck = crc32ieee(payload);
  const out = new Uint8Array(payload.length + 4);
  out.set(payload);
  writeUint32LE(out, payload.length, ck);
  return out;
}

function decodeSection(data: Uint8Array): Uint8Array {
  if (data.length < 4) throw new InvalidInputError('invalid section table: section too short');
  const payload = data.slice(0, data.length - 4);
  const storedCk = readUint32LE(data, data.length - 4);
  const computedCk = crc32ieee(payload);
  if (storedCk !== computedCk) throw new InvalidInputError('checksum mismatch');
  return payload;
}

// ---- Data section ----------------------------------------------------------

export function encodeDataEntries(entries: DataEntry[]): Uint8Array {
  const sorted = [...entries].sort((a, b) => compareBytes(a.key, b.key));
  for (let i = 1; i < sorted.length; i++) {
    if (equalBytes(sorted[i - 1].key, sorted[i].key)) throw new InvalidInputError('duplicate data key');
  }
  let total = 0;
  for (const e of sorted) total += 8 + e.key.length + e.value.length;
  const buf = new Uint8Array(total);
  let pos = 0;
  for (const e of sorted) {
    writeUint32LE(buf, pos, e.key.length);
    writeUint32LE(buf, pos + 4, e.value.length);
    pos += 8;
    buf.set(e.key, pos);
    pos += e.key.length;
    buf.set(e.value, pos);
    pos += e.value.length;
  }
  return buf;
}

export function decodeDataEntries(payload: Uint8Array): DataEntry[] {
  const entries: DataEntry[] = [];
  let pos = 0;
  while (pos < payload.length) {
    if (payload.length - pos < 8) throw new InvalidInputError('invalid data section');
    const keyLen = readUint32LE(payload, pos);
    const valueLen = readUint32LE(payload, pos + 4);
    pos += 8;
    if (pos + keyLen + valueLen > payload.length) throw new InvalidInputError('invalid data section');
    const key = payload.slice(pos, pos + keyLen);
    const value = payload.slice(pos + keyLen, pos + keyLen + valueLen);
    pos += keyLen + valueLen;
    if (entries.length > 0) {
      const cmp = compareBytes(entries[entries.length - 1].key, key);
      if (cmp === 0) throw new InvalidInputError('duplicate data key');
      if (cmp > 0) throw new InvalidInputError('invalid data section: keys not sorted');
    }
    entries.push({ key: new Uint8Array(key), value: new Uint8Array(value) });
  }
  return entries;
}

// ---- Bloom filter ----------------------------------------------------------

export function encodeBloom(keys: Uint8Array[]): Uint8Array {
  const keyCount = BigInt(keys.length);
  const bitCount = keyCount * BigInt(BLOOM_BITS_PER_KEY);
  const bitsLen = Number((bitCount + 7n) / 8n);
  const bitsArr = new Uint8Array(bitsLen);

  for (const key of keys) {
    if (bitCount === 0n) break;
    const [h1, h2] = murmur3x64_128(key, BLOOM_HASH_SEED);
    for (let i = 0n; i < BigInt(BLOOM_HASH_COUNT); i++) {
      const idx = ((h1 + i * h2) & 0xffffffffffffffffn) % bitCount;
      const byteIdx = Number(idx / 8n);
      const bitIdx = Number(idx % 8n);
      bitsArr[byteIdx] |= 1 << bitIdx;
    }
  }

  const buf = new Uint8Array(8 + 8 + 1 + 4 + bitsLen);
  writeUint64LE(buf, 0, keyCount);
  writeUint64LE(buf, 8, bitCount);
  buf[16] = BLOOM_HASH_COUNT;
  writeUint32LE(buf, 17, BLOOM_HASH_SEED);
  buf.set(bitsArr, 21);
  return buf;
}

export function decodeBloom(payload: Uint8Array): void {
  if (payload.length < 21) throw new InvalidInputError('invalid bloom section');
  const keyCount = readUint64LE_big(payload, 0);
  const bitCount = readUint64LE_big(payload, 8);
  const hashCount = payload[16];
  const hashSeed = readUint32LE(payload, 17);
  const bitsArr = payload.slice(21);

  if (hashCount !== BLOOM_HASH_COUNT || hashSeed !== BLOOM_HASH_SEED || bitCount !== keyCount * BigInt(BLOOM_BITS_PER_KEY)) {
    throw new InvalidInputError('invalid bloom section: bad parameters');
  }
  if (BigInt(bitsArr.length) !== (bitCount + 7n) / 8n) {
    throw new InvalidInputError('invalid bloom section: bad bit array length');
  }
  if (bitCount % 8n !== 0n && bitsArr.length > 0) {
    const extra = Number(bitCount % 8n);
    const mask = (0xff << extra) & 0xff;
    if (bitsArr[bitsArr.length - 1] & mask) {
      throw new InvalidInputError('invalid bloom section: trailing bits set');
    }
  }
}

// ---- Container -------------------------------------------------------------

// encodeContainer always writes CURRENT_MAJOR (the decoded `major` is read-only),
// so callers need not supply it.
export function encodeContainer(c: Omit<Container, 'major'>): Uint8Array {
  const sections: Section[] = [];
  const payloads: Uint8Array[] = [];

  const addSection = (type: number, payload: Uint8Array) => {
    const encoded = encodeSection(payload);
    sections.push({ type, offset: 0n, length: BigInt(encoded.length) });
    payloads.push(encoded);
  };

  addSection(SECTION_DATA, c.data);
  if (c.bloom !== null) {
    addSection(SECTION_BLOOM, c.bloom);
  }
  if (c.wal !== null) {
    if (c.wal.length === 0) {
      sections.push({ type: SECTION_WAL, offset: 0n, length: 0n });
      payloads.push(new Uint8Array(0));
    } else {
      addSection(SECTION_WAL, c.wal);
    }
  }

  const tableSize = sections.length * SECTION_ENTRY_SIZE;
  let off = BigInt(HEADER_SIZE + tableSize);
  for (let i = 0; i < sections.length; i++) {
    sections[i].offset = off;
    off += sections[i].length;
  }

  const header = encodeHeader(sections.length);
  const table = encodeSectionTable(sections);
  const out = new Uint8Array(Number(off));
  out.set(header, 0);
  out.set(table, HEADER_SIZE);
  let pos = HEADER_SIZE + tableSize;
  for (const p of payloads) {
    out.set(p, pos);
    pos += p.length;
  }
  return out;
}

export function decodeContainer(file: Uint8Array): Container {
  const sectionCount = decodeHeader(file);
  const tableStart = HEADER_SIZE;
  const tableEnd = tableStart + sectionCount * SECTION_ENTRY_SIZE;
  if (file.length < tableEnd) throw new InvalidInputError('invalid section table: file too short');
  const sections = decodeSectionTable(file.slice(tableStart, tableEnd), sectionCount);
  validateSections(sections, BigInt(file.length));

  const c: Container = { major: file[4], data: new Uint8Array(0), bloom: null, wal: null };
  for (const s of sections) {
    if (s.type === SECTION_WAL && s.length === 0n) {
      c.wal = new Uint8Array(0);
      continue;
    }
    const start = Number(s.offset);
    const end = Number(s.offset + s.length);
    const payload = decodeSection(file.slice(start, end));
    switch (s.type) {
      case SECTION_DATA: c.data = payload; break;
      case SECTION_BLOOM: c.bloom = payload; break;
      case SECTION_WAL: c.wal = payload; break;
    }
  }
  return c;
}

// ---- Utilities -------------------------------------------------------------

export function compareBytes(a: Uint8Array, b: Uint8Array): number {
  const len = Math.min(a.length, b.length);
  for (let i = 0; i < len; i++) {
    if (a[i] < b[i]) return -1;
    if (a[i] > b[i]) return 1;
  }
  return a.length - b.length;
}

export function equalBytes(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}

export function writeUint32LE(buf: Uint8Array, off: number, v: number): void {
  buf[off] = v & 0xff;
  buf[off + 1] = (v >>> 8) & 0xff;
  buf[off + 2] = (v >>> 16) & 0xff;
  buf[off + 3] = (v >>> 24) & 0xff;
}

export function readUint32LE(buf: Uint8Array, off: number): number {
  return (buf[off] | (buf[off + 1] << 8) | (buf[off + 2] << 16) | (buf[off + 3] << 24)) >>> 0;
}

export function writeUint64LE(buf: Uint8Array, off: number, v: bigint): void {
  const lo = Number(v & 0xffffffffn);
  const hi = Number((v >> 32n) & 0xffffffffn);
  writeUint32LE(buf, off, lo);
  writeUint32LE(buf, off + 4, hi);
}

export function readUint64LE_big(buf: Uint8Array, off: number): bigint {
  const lo = BigInt(readUint32LE(buf, off));
  const hi = BigInt(readUint32LE(buf, off + 4));
  return lo | (hi << 32n);
}

import { InvalidInputError } from '../errors.js';

export type MsgpackValue =
  | null
  | boolean
  | string
  | number
  | bigint
  | MsgpackValue[]
  | { [key: string]: MsgpackValue };

function validateUtf8(s: string): void {
  for (let i = 0; i < s.length; ) {
    const cp = s.codePointAt(i)!;
    if (cp >= 0xd800 && cp <= 0xdfff) {
      throw new InvalidInputError('invalid UTF-8 string');
    }
    i += cp > 0xffff ? 2 : 1;
  }
}

function encodeStr(buf: number[], s: string): void {
  validateUtf8(s);
  const encoded = new TextEncoder().encode(s);
  const len = encoded.length;
  if (len < 32) {
    buf.push(0xa0 | len);
  } else {
    buf.push(0xdb);
    buf.push((len >>> 24) & 0xff, (len >>> 16) & 0xff, (len >>> 8) & 0xff, len & 0xff);
  }
  for (let i = 0; i < encoded.length; i++) buf.push(encoded[i]);
}

function encodeUint64(buf: number[], v: bigint): void {
  buf.push(0xcf);
  const hi = Number((v >> 32n) & 0xffffffffn);
  const lo = Number(v & 0xffffffffn);
  buf.push(
    (hi >>> 24) & 0xff, (hi >>> 16) & 0xff, (hi >>> 8) & 0xff, hi & 0xff,
    (lo >>> 24) & 0xff, (lo >>> 16) & 0xff, (lo >>> 8) & 0xff, lo & 0xff,
  );
}

function appendValue(buf: number[], v: MsgpackValue): void {
  if (v === null) {
    buf.push(0xc0);
    return;
  }
  if (typeof v === 'boolean') {
    buf.push(v ? 0xc3 : 0xc2);
    return;
  }
  if (typeof v === 'string') {
    encodeStr(buf, v);
    return;
  }
  if (typeof v === 'bigint') {
    if (v < 0n) throw new InvalidInputError('negative bigint not supported');
    encodeUint64(buf, v);
    return;
  }
  if (typeof v === 'number') {
    if (Number.isInteger(v) && v >= 0 && v <= Number.MAX_SAFE_INTEGER) {
      encodeUint64(buf, BigInt(v));
    } else {
      buf.push(0xcb);
      const dv = new DataView(new ArrayBuffer(8));
      dv.setFloat64(0, v, false);
      for (let i = 0; i < 8; i++) buf.push(dv.getUint8(i));
    }
    return;
  }
  if (Array.isArray(v)) {
    const len = v.length;
    if (len < 16) {
      buf.push(0x90 | len);
    } else {
      buf.push(0xdc, (len >>> 8) & 0xff, len & 0xff);
    }
    for (const item of v) appendValue(buf, item);
    return;
  }
  if (typeof v === 'object') {
    const keys = Object.keys(v).sort();
    const len = keys.length;
    if (len < 16) {
      buf.push(0x80 | len);
    } else {
      buf.push(0xde, (len >>> 8) & 0xff, len & 0xff);
    }
    for (const k of keys) {
      encodeStr(buf, k);
      appendValue(buf, (v as Record<string, MsgpackValue>)[k]);
    }
    return;
  }
  throw new InvalidInputError('unsupported msgpack value type');
}

export function encodeMsgpack(v: MsgpackValue): Uint8Array {
  const buf: number[] = [];
  appendValue(buf, v);
  return new Uint8Array(buf);
}

type DecodeResult = [MsgpackValue, number];

function decodeStr(data: Uint8Array, off: number, len: number): DecodeResult {
  if (off + len > data.length) throw new InvalidInputError('invalid msgpack: short string');
  const bytes = data.slice(off, off + len);
  const s = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  return [s, off + len];
}

function decodeArray(data: Uint8Array, off: number, len: number): DecodeResult {
  const arr: MsgpackValue[] = [];
  for (let i = 0; i < len; i++) {
    const [v, next] = decodeValue(data, off);
    arr.push(v);
    off = next;
  }
  return [arr, off];
}

function decodeMap(data: Uint8Array, off: number, len: number): DecodeResult {
  const m: Record<string, MsgpackValue> = {};
  for (let i = 0; i < len; i++) {
    const [k, off2] = decodeValue(data, off);
    if (typeof k !== 'string') throw new InvalidInputError('invalid msgpack: non-string map key');
    const [v, off3] = decodeValue(data, off2);
    m[k] = v;
    off = off3;
  }
  return [m, off];
}

function decodeValue(data: Uint8Array, off: number): DecodeResult {
  if (off >= data.length) throw new InvalidInputError('invalid msgpack: unexpected end');
  const c = data[off];
  off++;

  if (c <= 0x7f) return [c, off];
  if (c >= 0xa0 && c <= 0xbf) return decodeStr(data, off, c & 0x1f);
  if (c >= 0x90 && c <= 0x9f) return decodeArray(data, off, c & 0x0f);
  if (c >= 0x80 && c <= 0x8f) return decodeMap(data, off, c & 0x0f);

  switch (c) {
    case 0xc0: return [null, off];
    case 0xc2: return [false, off];
    case 0xc3: return [true, off];
    case 0xcc: {
      if (off >= data.length) throw new InvalidInputError('invalid msgpack');
      return [data[off], off + 1];
    }
    case 0xcd: {
      if (off + 2 > data.length) throw new InvalidInputError('invalid msgpack');
      return [(data[off] << 8) | data[off + 1], off + 2];
    }
    case 0xce: {
      if (off + 4 > data.length) throw new InvalidInputError('invalid msgpack');
      const v = ((data[off] << 24) | (data[off+1] << 16) | (data[off+2] << 8) | data[off+3]) >>> 0;
      return [v, off + 4];
    }
    case 0xcf: {
      if (off + 8 > data.length) throw new InvalidInputError('invalid msgpack');
      const hi = ((data[off] << 24) | (data[off+1] << 16) | (data[off+2] << 8) | data[off+3]) >>> 0;
      const lo = ((data[off+4] << 24) | (data[off+5] << 16) | (data[off+6] << 8) | data[off+7]) >>> 0;
      const big = (BigInt(hi) << 32n) | BigInt(lo);
      if (big <= BigInt(Number.MAX_SAFE_INTEGER)) {
        return [Number(big), off + 8];
      }
      return [big, off + 8];
    }
    case 0xcb: {
      if (off + 8 > data.length) throw new InvalidInputError('invalid msgpack');
      const dv = new DataView(data.buffer, data.byteOffset + off, 8);
      return [dv.getFloat64(0, false), off + 8];
    }
    case 0xdb: {
      if (off + 4 > data.length) throw new InvalidInputError('invalid msgpack');
      const len = ((data[off] << 24) | (data[off+1] << 16) | (data[off+2] << 8) | data[off+3]) >>> 0;
      return decodeStr(data, off + 4, len);
    }
    case 0xdc: {
      if (off + 2 > data.length) throw new InvalidInputError('invalid msgpack');
      const len = (data[off] << 8) | data[off+1];
      return decodeArray(data, off + 2, len);
    }
    case 0xde: {
      if (off + 2 > data.length) throw new InvalidInputError('invalid msgpack');
      const len = (data[off] << 8) | data[off+1];
      return decodeMap(data, off + 2, len);
    }
    default:
      throw new InvalidInputError(`invalid msgpack: unknown format byte 0x${c.toString(16)}`);
  }
}

export function decodeMsgpack(data: Uint8Array): [MsgpackValue, number] {
  return decodeValue(data, 0);
}

export function decodeMsgpackFull(data: Uint8Array): MsgpackValue {
  const [v, n] = decodeValue(data, 0);
  if (n !== data.length) throw new InvalidInputError('invalid msgpack: trailing bytes');
  return v;
}

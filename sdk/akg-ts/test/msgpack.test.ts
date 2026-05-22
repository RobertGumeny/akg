import { describe, it, expect } from 'vitest';
import { encodeMsgpack, decodeMsgpack, decodeMsgpackFull } from '../src/internal/msgpack.js';

function roundtrip(v: Parameters<typeof encodeMsgpack>[0]) {
  const encoded = encodeMsgpack(v);
  const [decoded] = decodeMsgpack(encoded);
  return decoded;
}

describe('msgpack', () => {
  it('encodes nil', () => {
    expect(encodeMsgpack(null)).toEqual(new Uint8Array([0xc0]));
    expect(roundtrip(null)).toBe(null);
  });

  it('encodes bool', () => {
    expect(encodeMsgpack(true)).toEqual(new Uint8Array([0xc3]));
    expect(encodeMsgpack(false)).toEqual(new Uint8Array([0xc2]));
    expect(roundtrip(true)).toBe(true);
    expect(roundtrip(false)).toBe(false);
  });

  it('encodes fixstr', () => {
    const v = 'hello';
    const enc = encodeMsgpack(v);
    expect(enc[0]).toBe(0xa0 | v.length);
    expect(roundtrip(v)).toBe(v);
  });

  it('encodes str32 for long strings', () => {
    const v = 'a'.repeat(32);
    const enc = encodeMsgpack(v);
    expect(enc[0]).toBe(0xdb);
    expect(roundtrip(v)).toBe(v);
  });

  it('encodes uint64', () => {
    const enc = encodeMsgpack(0);
    expect(enc[0]).toBe(0xcf);
    expect(roundtrip(0)).toBe(0);
    expect(roundtrip(255)).toBe(255);
    expect(roundtrip(65536)).toBe(65536);
    expect(roundtrip(1000000000)).toBe(1000000000);
  });

  it('encodes bigint', () => {
    const big = 2n ** 53n + 1n;
    const enc = encodeMsgpack(big);
    const [v] = decodeMsgpack(enc);
    expect(v).toBe(big);
  });

  it('encodes float64', () => {
    const v = 0.5;
    const enc = encodeMsgpack(v);
    expect(enc[0]).toBe(0xcb);
    expect(roundtrip(v)).toBe(v);
  });

  it('encodes arrays', () => {
    const v = [1, 'two', null];
    const rt = roundtrip(v) as unknown[];
    expect(rt[0]).toBe(1);
    expect(rt[1]).toBe('two');
    expect(rt[2]).toBe(null);
  });

  it('encodes maps with sorted keys', () => {
    const v = { z: 1, a: 2, m: 3 };
    const enc = encodeMsgpack(v);
    const [decoded] = decodeMsgpack(enc);
    expect(decoded).toEqual({ a: 2, m: 3, z: 1 });
    // Verify key order in bytes: 'a' should come before 'm' before 'z'
    const str = new TextDecoder().decode(enc);
    const aIdx = str.indexOf('a');
    const mIdx = str.indexOf('m');
    const zIdx = str.indexOf('z');
    expect(aIdx).toBeLessThan(mIdx);
    expect(mIdx).toBeLessThan(zIdx);
  });

  it('rejects invalid UTF-8 on decode', () => {
    // Create a fixstr with invalid UTF-8 (0xa1 = fixstr len 1, then 0xff = invalid byte)
    const bad = new Uint8Array([0xa1, 0xff]);
    expect(() => decodeMsgpack(bad)).toThrow();
  });

  it('rejects trailing bytes in decodeMsgpackFull', () => {
    const enc = encodeMsgpack(42);
    const withExtra = new Uint8Array([...enc, 0x00]);
    expect(() => decodeMsgpackFull(withExtra)).toThrow();
  });

  it('decodes uint8/uint16/uint32 to number', () => {
    // uint8
    expect(decodeMsgpack(new Uint8Array([0xcc, 200]))[0]).toBe(200);
    // uint16
    expect(decodeMsgpack(new Uint8Array([0xcd, 0x01, 0x00]))[0]).toBe(256);
    // uint32
    expect(decodeMsgpack(new Uint8Array([0xce, 0x00, 0x01, 0x00, 0x00]))[0]).toBe(65536);
  });
});

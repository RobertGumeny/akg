function rotl64(v: bigint, n: bigint): bigint {
  return ((v << n) | (v >> (64n - n))) & 0xffffffffffffffffn;
}

function fmix64(k: bigint): bigint {
  k = (k ^ (k >> 33n)) & 0xffffffffffffffffn;
  k = (k * 0xff51afd7ed558ccdn) & 0xffffffffffffffffn;
  k = (k ^ (k >> 33n)) & 0xffffffffffffffffn;
  k = (k * 0xc4ceb9fe1a85ec53n) & 0xffffffffffffffffn;
  k = (k ^ (k >> 33n)) & 0xffffffffffffffffn;
  return k;
}

export function murmur3x64_128(data: Uint8Array, seed: number): [bigint, bigint] {
  const c1 = 0x87c37b91114253d5n;
  const c2 = 0x4cf5ad432745937fn;
  let h1 = BigInt(seed) & 0xffffffffffffffffn;
  let h2 = BigInt(seed) & 0xffffffffffffffffn;

  const nblocks = Math.floor(data.length / 16);
  for (let i = 0; i < nblocks; i++) {
    const base = i * 16;
    let k1 = readUint64LE(data, base);
    let k2 = readUint64LE(data, base + 8);

    k1 = (k1 * c1) & 0xffffffffffffffffn;
    k1 = rotl64(k1, 31n);
    k1 = (k1 * c2) & 0xffffffffffffffffn;
    h1 ^= k1;

    h1 = rotl64(h1, 27n);
    h1 = (h1 + h2) & 0xffffffffffffffffn;
    h1 = (h1 * 5n + 0x52dce729n) & 0xffffffffffffffffn;

    k2 = (k2 * c2) & 0xffffffffffffffffn;
    k2 = rotl64(k2, 33n);
    k2 = (k2 * c1) & 0xffffffffffffffffn;
    h2 ^= k2;

    h2 = rotl64(h2, 31n);
    h2 = (h2 + h1) & 0xffffffffffffffffn;
    h2 = (h2 * 5n + 0x38495ab5n) & 0xffffffffffffffffn;
  }

  const tail = data.slice(nblocks * 16);
  let k1 = 0n;
  let k2 = 0n;

  for (let i = 0; i < tail.length && i < 8; i++) {
    k1 |= BigInt(tail[i]) << BigInt(8 * i);
  }
  for (let i = 8; i < tail.length; i++) {
    k2 |= BigInt(tail[i]) << BigInt(8 * (i - 8));
  }

  if (k2 !== 0n) {
    k2 = (k2 * c2) & 0xffffffffffffffffn;
    k2 = rotl64(k2, 33n);
    k2 = (k2 * c1) & 0xffffffffffffffffn;
    h2 ^= k2;
  }
  if (k1 !== 0n) {
    k1 = (k1 * c1) & 0xffffffffffffffffn;
    k1 = rotl64(k1, 31n);
    k1 = (k1 * c2) & 0xffffffffffffffffn;
    h1 ^= k1;
  }

  const len = BigInt(data.length);
  h1 ^= len;
  h2 ^= len;

  h1 = (h1 + h2) & 0xffffffffffffffffn;
  h2 = (h2 + h1) & 0xffffffffffffffffn;

  h1 = fmix64(h1);
  h2 = fmix64(h2);

  h1 = (h1 + h2) & 0xffffffffffffffffn;
  h2 = (h2 + h1) & 0xffffffffffffffffn;

  return [h1, h2];
}

function readUint64LE(data: Uint8Array, off: number): bigint {
  let v = 0n;
  for (let i = 0; i < 8; i++) {
    v |= BigInt(data[off + i]) << BigInt(8 * i);
  }
  return v;
}

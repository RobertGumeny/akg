import { InvalidInputError } from '../errors.js';
import { crc32ieee } from './crc32.js';
import { readUint32LE, readUint64LE_big, writeUint32LE, writeUint64LE } from './format.js';

export const WAL_RECORD_HEADER_SIZE = 13;

export const WAL_OP_PUT_NODE: number = 0x01;
export const WAL_OP_DELETE_NODE: number = 0x02;
export const WAL_OP_PUT_EDGE: number = 0x03;
export const WAL_OP_DELETE_EDGE: number = 0x04;
export const WAL_OP_COMMIT: number = 0x05;

export interface WALRecord {
  sequence: bigint;
  operation: number;
  payload: Uint8Array;
}

function validOp(op: number): boolean {
  return op === WAL_OP_PUT_NODE || op === WAL_OP_DELETE_NODE || op === WAL_OP_PUT_EDGE || op === WAL_OP_DELETE_EDGE || op === WAL_OP_COMMIT;
}

export function encodeWALRecord(r: WALRecord): Uint8Array {
  if (!validOp(r.operation)) throw new InvalidInputError('unknown wal operation');
  if (r.operation === WAL_OP_COMMIT && r.payload.length !== 0) throw new InvalidInputError('invalid wal record: commit must have empty payload');
  const size = WAL_RECORD_HEADER_SIZE + r.payload.length + 4;
  const buf = new Uint8Array(size);
  writeUint64LE(buf, 0, r.sequence);
  buf[8] = r.operation;
  writeUint32LE(buf, 9, r.payload.length);
  buf.set(r.payload, WAL_RECORD_HEADER_SIZE);
  const ckData = buf.slice(0, WAL_RECORD_HEADER_SIZE + r.payload.length);
  writeUint32LE(buf, WAL_RECORD_HEADER_SIZE + r.payload.length, crc32ieee(ckData));
  return buf;
}

export function decodeWALRecord(buf: Uint8Array): [WALRecord, number] {
  if (buf.length < WAL_RECORD_HEADER_SIZE) throw new InvalidInputError('invalid wal record: too short');
  const seq = readUint64LE_big(buf, 0);
  const op = buf[8];
  if (!validOp(op)) throw new InvalidInputError('unknown wal operation');
  const length = readUint32LE(buf, 9);
  const need = WAL_RECORD_HEADER_SIZE + length + 4;
  if (BigInt(length) > BigInt(buf.length - WAL_RECORD_HEADER_SIZE) || buf.length < need) {
    throw new InvalidInputError('invalid wal record: insufficient data for declared payload length');
  }
  const ckData = buf.slice(0, WAL_RECORD_HEADER_SIZE + length);
  const stored = readUint32LE(buf, WAL_RECORD_HEADER_SIZE + length);
  const computed = crc32ieee(ckData);
  if (stored !== computed) throw new InvalidInputError('wal checksum mismatch');
  if (op === WAL_OP_COMMIT && length !== 0) throw new InvalidInputError('invalid wal record: commit must have empty payload');
  return [
    { sequence: seq, operation: op, payload: new Uint8Array(buf.slice(WAL_RECORD_HEADER_SIZE, WAL_RECORD_HEADER_SIZE + length)) },
    need,
  ];
}

export function encodeWALRecords(records: WALRecord[]): Uint8Array {
  const parts: Uint8Array[] = [];
  let total = 0;
  for (const r of records) {
    const encoded = encodeWALRecord(r);
    parts.push(encoded);
    total += encoded.length;
  }
  const out = new Uint8Array(total);
  let pos = 0;
  for (const p of parts) {
    out.set(p, pos);
    pos += p.length;
  }
  return out;
}

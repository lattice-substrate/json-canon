#!/usr/bin/env node
/**
 * gen_v8_numbers.js
 *
 * Generate an IEEE-754 binary64 corpus using V8's JSON.stringify as the oracle,
 * as recommended by RFC 8785 for validating JCS number serialization.
 *
 * References:
 * - RFC 8785 §3.2.2.3 and Appendix guidance (V8 live reference): https://www.rfc-editor.org/rfc/rfc8785
 *
 * Output format (CSV):
 *   ieee754_hex,json_string
 *
 * Notes:
 * - Exclude NaN and Infinity (JSON numbers cannot be NaN/Infinity per RFC 8259; RFC 8785 requires error).
 * - This script is a *tooling dependency* only. The canonicalizer remains dependency-free.
 */

'use strict';

const fs = require('fs');
const crypto = require('crypto');

function u64ToFloat64(u64BigInt) {
  // Use ArrayBuffer/DataView to interpret bits.
  const buf = new ArrayBuffer(8);
  const view = new DataView(buf);
  // split BigInt into hi/lo 32-bit.
  const hi = Number((u64BigInt >> 32n) & 0xffffffffn);
  const lo = Number(u64BigInt & 0xffffffffn);
  // DataView uses big endian if specified; use big-endian to make deterministic here.
  view.setUint32(0, hi, false);
  view.setUint32(4, lo, false);
  return view.getFloat64(0, false);
}

function isFiniteNumber(x) {
  return Number.isFinite(x);
}

function toHex(u64BigInt) {
  return u64BigInt.toString(16).padStart(16, '0');
}

// Simple xorshift64* PRNG for determinism.
let state = 0x123456789abcdef0n;
function nextU64() {
  // xorshift64*
  state ^= state >> 12n;
  state ^= state << 25n;
  state ^= state >> 27n;
  return (state * 0x2545F4914F6CDD1Dn) & 0xffffffffffffffffn;
}

const outPath = process.argv[2] || 'v8_numbers.csv';
const count = Number(process.argv[3] || '1000000');

const out = fs.createWriteStream(outPath, { encoding: 'utf8' });
out.write('ieee754_hex,json_string\n');

let written = 0;
for (let i = 0; i < count; i++) {
  const u64 = nextU64();
  const x = u64ToFloat64(u64);
  if (!isFiniteNumber(x)) continue;
  // V8 oracle:
  const s = JSON.stringify(x);
  // JSON.stringify on a Number returns a JSON number string, or "null" for non-finite.
  if (s === 'null') continue;
  out.write(`${toHex(u64)},${s}\n`);
  written++;
}

out.end();
console.error(`Wrote ${written} finite values to ${outPath}`);

#!/usr/bin/env node
'use strict';

const TARGET_ROWS = 54445;
const SIGN_BIT = 0x8000000000000000n;
const MASK64 = 0xffffffffffffffffn;

const dv = new DataView(new ArrayBuffer(8));

function numberFromBits(bits) {
  dv.setBigUint64(0, bits, false);
  return dv.getFloat64(0, false);
}

function bitsFromNumber(x) {
  dv.setFloat64(0, x, false);
  return dv.getBigUint64(0, false);
}

function isFiniteBits(bits) {
  return Number.isFinite(numberFromBits(bits));
}

function formatBits(bits) {
  return bits.toString(16).padStart(16, '0');
}

const set = new Set();

function addBits(bits) {
  if (bits < 0n || bits > MASK64) {
    return;
  }
  if (!isFiniteBits(bits)) {
    return;
  }
  set.add(bits);
}

function addBothSigns(bits) {
  addBits(bits);
  addBits(bits ^ SIGN_BIT);
}

function nextUp(x) {
  if (!Number.isFinite(x)) return x;
  if (Object.is(x, -0)) return Number.MIN_VALUE;
  if (x === 0) return Number.MIN_VALUE;
  const bits = bitsFromNumber(x);
  if (x > 0) {
    return numberFromBits(bits + 1n);
  }
  return numberFromBits(bits - 1n);
}

function nextDown(x) {
  if (!Number.isFinite(x)) return x;
  if (x === 0) return -Number.MIN_VALUE;
  const bits = bitsFromNumber(x);
  if (x > 0) {
    return numberFromBits(bits - 1n);
  }
  return numberFromBits(bits + 1n);
}

// Signed zeros.
addBothSigns(0n);

// Dense subnormal prefix catches hard rounding cases near zero.
for (let i = 1n; i <= 4096n; i++) {
  addBothSigns(i);
}

// Boundaries between subnormal and normal, and around max finite.
for (let i = -128n; i <= 128n; i++) {
  addBothSigns(0x000fffffffffffffn + i);
  addBothSigns(0x0010000000000000n + i);
  addBothSigns(0x7fefffffffffffffn + i);
}

// Powers of two across the full binary64 exponent range.
for (let e = -1074; e <= 1023; e++) {
  const x = Math.pow(2, e);
  if (!Number.isFinite(x) || x === 0) {
    continue;
  }
  addBothSigns(bitsFromNumber(x));
}

// RFC 8785 Appendix B examples plus nearby neighbors.
const appendixB = [
  Number.MIN_VALUE,
  2.2250738585072014e-308,
  1e-6,
  1e-7,
  0.000001,
  0.0000009999999999999999,
  0.0000010000000000000002,
  0.1,
  0.2,
  1,
  1.2345678901234567,
  9007199254740991,
  9007199254740992,
  9007199254740993,
  1e20,
  1e21,
  Number.MAX_VALUE,
];
for (const x of appendixB) {
  if (!Number.isFinite(x)) {
    continue;
  }
  const around = [x, nextDown(x), nextUp(x), nextDown(nextDown(x)), nextUp(nextUp(x))];
  for (const y of around) {
    if (Number.isFinite(y)) {
      addBothSigns(bitsFromNumber(y));
    }
  }
}

// Deterministic pseudo-random fill to target size.
let state = 0x9e3779b97f4a7c15n;
function nextRand64() {
  state ^= state >> 12n;
  state ^= (state << 25n) & MASK64;
  state ^= state >> 27n;
  return (state * 0x2545f4914f6cdd1dn) & MASK64;
}

while (set.size < TARGET_ROWS) {
  addBits(nextRand64());
}

const rows = Array.from(set).sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
if (rows.length !== TARGET_ROWS) {
  throw new Error(`internal error: expected ${TARGET_ROWS} rows, got ${rows.length}`);
}

for (const bits of rows) {
  const x = numberFromBits(bits);
  process.stdout.write(`${formatBits(bits)},${String(x)}\n`);
}

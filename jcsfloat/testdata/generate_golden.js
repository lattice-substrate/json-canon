#!/usr/bin/env node
"use strict";

const TARGET_TOTAL = 54445;
const RANDOM_COUNT = 50000;

function bitsToNumber(bits) {
  const buf = Buffer.allocUnsafe(8);
  buf.writeBigUInt64BE(bits);
  return buf.readDoubleBE(0);
}

function numberToBits(num) {
  const buf = Buffer.allocUnsafe(8);
  buf.writeDoubleBE(num);
  return buf.readBigUInt64BE(0);
}

function isFiniteBits(bits) {
  const exp = (bits >> 52n) & 0x7ffn;
  return exp !== 0x7ffn;
}

function toHex(bits) {
  return bits.toString(16).padStart(16, "0");
}

const all = new Set();

function addBits(bits) {
  if (!isFiniteBits(bits)) return false;
  all.add(toHex(bits));
  return true;
}

function addNumber(num) {
  return addBits(numberToBits(num));
}

// Required anchors.
addBits(0x0000000000000000n); // +0
addBits(0x8000000000000000n); // -0

// RFC 8785 Appendix B representative values.
[
  0x0000000000000001n,
  0x8000000000000001n,
  0x7fefffffffffffffn,
  0xffefffffffffffffn,
  0x0010000000000000n,
  0x8010000000000000n,
  0x3ff0000000000000n,
  0xbff0000000000000n,
  0x3ff199999999999an,
  0x3fb999999999999an,
  0x3fd3333333333333n,
  0x3fd5555555555555n,
  0x3fe0000000000000n,
  0x3fefffffffffffffn,
  0x3ff0000000000001n,
  0x4340000000000000n,
  0x444b1ae4d6e2ef50n,
  0x44b52d02c7e14af6n,
  0x3eb0c6f7a0b5ed8dn,
  0x3f1a36e2eb1c432dn,
  0x3f50624dd2f1a9fcn,
  0x3f847ae147ae147bn,
  0x3fdfffffffffffffn,
  0x3fe0000000000001n,
  0x3ff8000000000000n,
].forEach(addBits);

// Every exact power of two from 2^-1074 to 2^1023.
for (let e = -1074; e <= 1023; e++) {
  addNumber(Math.pow(2, e));
}

// First and last 100 positive subnormals.
for (let i = 1n; i <= 100n; i++) {
  addBits(i);
}
const maxSubnormal = 0x000fffffffffffffn;
for (let i = 0n; i < 100n; i++) {
  addBits(maxSubnormal - i);
}

// Adjacent doubles around key formatting boundaries and common fractions.
function nextUp(bits) {
  if (bits === 0x7fefffffffffffffn) return bits;
  if ((bits & 0x8000000000000000n) !== 0n) {
    return bits - 1n;
  }
  return bits + 1n;
}

function nextDown(bits) {
  if (bits === 0x0000000000000000n) return 0x8000000000000001n;
  if ((bits & 0x8000000000000000n) !== 0n) {
    return bits + 1n;
  }
  return bits - 1n;
}

const anchors = [
  0.1,
  0.2,
  1 / 3,
  1,
  2,
  3,
  10,
  1e-7,
  1e-6,
  1e20,
  1e21,
];

for (const a of anchors) {
  let b = numberToBits(a);
  addBits(b);
  let up = b;
  let down = b;
  for (let i = 0; i < 120; i++) {
    up = nextUp(up);
    down = nextDown(down);
    addBits(up);
    addBits(down);
  }
}

// Fill deterministic non-random coverage up to exactly 4,445 vectors.
let sweep = 0n;
while (all.size < TARGET_TOTAL - RANDOM_COUNT) {
  addBits(sweep);
  addBits(0x8000000000000000n | sweep);
  sweep += 17n;
}

// Deterministic xorshift64* PRNG for 50,000 random finite bit patterns.
let state = 0x9e3779b97f4a7c15n;
function rand64() {
  state ^= state >> 12n;
  state ^= state << 25n;
  state ^= state >> 27n;
  return (state * 0x2545f4914f6cdd1dn) & 0xffffffffffffffffn;
}

let randomAdded = 0;
while (randomAdded < RANDOM_COUNT) {
  const bits = rand64();
  if (!isFiniteBits(bits)) {
    continue;
  }
  const before = all.size;
  addBits(bits);
  if (all.size > before) {
    randomAdded++;
  }
}

// Keep exactly TARGET_TOTAL entries.
const rows = Array.from(all).sort();
if (rows.length < TARGET_TOTAL) {
  throw new Error(`generated ${rows.length}, expected at least ${TARGET_TOTAL}`);
}
const finalRows = rows.slice(0, TARGET_TOTAL);

for (const hex of finalRows) {
  const n = bitsToNumber(BigInt(`0x${hex}`));
  process.stdout.write(`${hex},${String(n)}\n`);
}

console.error(`Generated ${finalRows.length} golden vectors`);

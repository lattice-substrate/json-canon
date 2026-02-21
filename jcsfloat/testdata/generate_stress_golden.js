#!/usr/bin/env node
'use strict';

function toHex64(bits) {
  return bits.toString(16).padStart(16, '0');
}

function isFiniteBits(bits) {
  return Number((bits >> 52n) & 0x7ffn) !== 0x7ff;
}

function add(set, bits) {
  if (isFiniteBits(bits)) {
    set.add(bits);
  }
}

function bitsFromDouble(x) {
  const buf = Buffer.allocUnsafe(8);
  buf.writeDoubleBE(x, 0);
  return buf.readBigUInt64BE(0);
}

function doubleFromBits(bits) {
  const buf = Buffer.allocUnsafe(8);
  buf.writeBigUInt64BE(bits, 0);
  return buf.readDoubleBE(0);
}

const set = new Set();

const explicit = [
  0,
  -0,
  Number.MIN_VALUE,
  Number.MIN_VALUE * 2,
  1e-6,
  1e21,
  1e20,
  Number.MAX_VALUE,
  -Number.MAX_VALUE,
  0.1,
  0.2,
  0.3,
  1.2345678901234567,
  5e-324,
  1.7976931348623157e+308,
];
for (const v of explicit) {
  add(set, bitsFromDouble(v));
}

for (const h of [
  '3fd5555555555555',
  '3fe0000000000000',
  '3ff0000000000001',
  '3ff199999999999a',
  '4330000000000000',
  '4340000000000000',
  '44b52d02c7e14af6',
  '7fefffffffffffff',
  '000fffffffffffff',
  '0010000000000000',
]) {
  add(set, BigInt('0x' + h));
}

let x = 0x9e3779b97f4a7c15n;
for (let i = 0; i < 220000; i++) {
  x = (x + 0x9e3779b97f4a7c15n) & 0xffffffffffffffffn;
  let z = x;
  z = ((z ^ (z >> 30n)) * 0xbf58476d1ce4e5b9n) & 0xffffffffffffffffn;
  z = ((z ^ (z >> 27n)) * 0x94d049bb133111ebn) & 0xffffffffffffffffn;
  z = (z ^ (z >> 31n)) & 0xffffffffffffffffn;
  add(set, z);
}

for (const base of [bitsFromDouble(1e-6), bitsFromDouble(1e21), bitsFromDouble(Number.MIN_VALUE), bitsFromDouble(Number.MAX_VALUE)]) {
  for (let d = 0n; d < 2000n; d++) {
    add(set, (base + d) & 0xffffffffffffffffn);
    if (base > d) {
      add(set, base - d);
    }
  }
}

const bits = Array.from(set).sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
for (const b of bits) {
  const x = doubleFromBits(b);
  process.stdout.write(`${toHex64(b)},${String(x)}\n`);
}

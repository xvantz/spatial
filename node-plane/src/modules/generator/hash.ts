const OFFSET_64 = BigInt("14695981039346656037");
const PRIME_64 = BigInt("1099511628211");

const buffer = Buffer.alloc(4);

export const hashPlayer = (userId: number, x: number, y: number, z: number): bigint => {
  let h = OFFSET_64;

  // UserId
  buffer.writeUInt32LE(userId, 0);
  for (const b of buffer) {
    h ^= BigInt(b);
    h *= PRIME_64;
    h &= BigInt("0xFFFFFFFFFFFFFFFF");
  }

  // X
  buffer.writeFloatLE(x, 0);
  for (const b of buffer) {
    h ^= BigInt(b);
    h *= PRIME_64;
    h &= BigInt("0xFFFFFFFFFFFFFFFF");
  }

  // Y
  buffer.writeFloatLE(y, 0);
  for (const b of buffer) {
    h ^= BigInt(b);
    h *= PRIME_64;
    h &= BigInt("0xFFFFFFFFFFFFFFFF");
  }

  // Z
  buffer.writeFloatLE(z, 0);
  for (const b of buffer) {
    h ^= BigInt(b);
    h *= PRIME_64;
    h &= BigInt("0xFFFFFFFFFFFFFFFF");
  }

  return h;
};

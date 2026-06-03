import { describe, it, expect } from 'vitest';
import { hashPlayer } from '../modules/generator/hash';

describe('hashPlayer', () => {
  it('should produce same hash for same input', () => {
    const h1 = hashPlayer(1, 10.5, 20.5, 30.5);
    const h2 = hashPlayer(1, 10.5, 20.5, 30.5);
    expect(h1).toBe(h2);
  });

  it('should produce different hash for different positions', () => {
    const h1 = hashPlayer(1, 10, 20, 30);
    const h2 = hashPlayer(1, 11, 20, 30);
    expect(h1).not.toBe(h2);
  });

  it('should handle concurrent calls without corruption', async () => {
    // Одинаковые вызовы должны давать одинаковый результат
    const calls = Array.from({ length: 100 }, (_, i) =>
      Promise.resolve().then(() => hashPlayer(i % 10, 42, 42, 42)),
    );
    const results = await Promise.all(calls);
    for (let i = 0; i < 10; i++) {
      expect(results[i]).toBe(results[i + 10]);
    }
  });
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { decodeEnvelope, decodeHeartbeat, encodeEnvelope, encodeTags, goFormatG, goQuote, goUnixSeconds } from '../src/goCompat.js';

test('%g de Go: exponencial cuando el exponente es < -4 o >= 6', () => {
  assert.deepEqual([3.29, 100, 123456.7, 1234567, 1e6, 0.0001, 0.00001, -2.5e-7, 0].map(goFormatG),
    ['3.29', '100', '123456.7', '1.234567e+06', '1e+06', '0.0001', '1e-05', '-2.5e-07', '0']);
});

test('%q de Go escapa comillas, diagonales y controles', () => {
  assert.equal(goQuote('C:\\ "x"\n'), '"C:\\\\ \\"x\\"\\n"');
  assert.equal(goQuote('ñandú'), '"ñandú"');
});

test('un envelope con fecha inválida se descarta como en encoding/json', () => {
  assert.equal(decodeEnvelope('{"agent_id":"a","timestamp":"ayer","results":[]}'), null);
  assert.equal(decodeEnvelope('{"agent_id":"a","sequence":-1}'), null);
  assert.equal(decodeEnvelope('no json'), null);
});

test('se vuelve a serializar con el orden de campos y omitempty de Go', () => {
  const env = decodeEnvelope(JSON.stringify({ results: [{ module: 'cpu', duration_ms: 3, data: { usage_percent: 1 }, collected_at: '2026-10-01T08:00:00-06:00' }, { module: 'ups', error: 'sin UPS', collected_at: '2026-10-01T08:00:00-06:00' }], sequence: 7, agent_id: 'a', extra: 1 }));
  assert.equal(encodeEnvelope(env), '{"schema":"","agent_id":"a","hostname":"","version":"","os":"","arch":"","sequence":7,"timestamp":"0001-01-01T00:00:00Z","results":[{"module":"cpu","collected_at":"2026-10-01T08:00:00-06:00","data":{"usage_percent":1},"duration_ms":3},{"module":"ups","collected_at":"2026-10-01T08:00:00-06:00","error":"sin UPS","duration_ms":0}]}');
});

test('tags nil se guardan como null y vacías como []', () => {
  assert.equal(encodeTags(null), 'null');
  assert.equal(encodeTags([]), '[]');
  assert.equal(encodeTags(['lab']), '["lab"]');
});

test('heartbeat y conversión a segundos Unix', () => {
  assert.equal(decodeHeartbeat('{"agent_id":"a","self_mem_mb":"x"}'), null);
  assert.equal(decodeHeartbeat('{"agent_id":"a","self_mem_mb":20}').self_mem_mb, 20);
  assert.equal(goUnixSeconds('2026-10-01T08:00:00-06:00'), 1790863200);
  assert.equal(goUnixSeconds('0001-01-01T00:00:00Z'), -62135596800);
});

// Compatibilidad con cmd/mrti-monitor (Go). Los agentes y quienes consumen
// la API (MRTI-Obs, Prometheus) se escribieron contra el servidor de Go, así
// que este servidor reproduce sus formatos tal cual: cómo decodifica los
// mensajes (encoding/json hacia structs), cómo los vuelve a serializar y
// cómo imprime números y etiquetas en /metrics (fmt %g y %q).

const RFC3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

class DecodeError extends Error {}

const isObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

function str(value) {
  if (value === undefined || value === null) return '';
  if (typeof value !== 'string') throw new DecodeError('string esperado');
  return value;
}

function num(value, { unsigned = false, integer = false } = {}) {
  if (value === undefined || value === null) return 0;
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new DecodeError('número esperado');
  if (integer && !Number.isInteger(value)) throw new DecodeError('entero esperado');
  if (unsigned && value < 0) throw new DecodeError('entero sin signo esperado');
  return value;
}

function bool(value) {
  if (value === undefined || value === null) return false;
  if (typeof value !== 'boolean') throw new DecodeError('booleano esperado');
  return value;
}

// time.Time de Go: null deja el valor cero; si no es RFC 3339 el mensaje
// completo se descarta (json.Unmarshal devuelve error).
const ZERO_TIME = '0001-01-01T00:00:00Z';
function time(value) {
  if (value === undefined || value === null) return ZERO_TIME;
  if (typeof value !== 'string' || !RFC3339.test(value) || Number.isNaN(Date.parse(value))) throw new DecodeError('fecha RFC 3339 esperada');
  return value;
}

export function goUnixSeconds(rfc3339) {
  if (rfc3339 === ZERO_TIME) return -62135596800;
  return Math.floor(Date.parse(rfc3339) / 1000);
}

function stringArray(value) {
  if (value === undefined || value === null) return null;
  if (!Array.isArray(value)) throw new DecodeError('arreglo esperado');
  return value.map(str);
}

// Envelope (model.Envelope). Devuelve null cuando Go lo habría rechazado.
export function decodeEnvelope(raw) {
  try {
    const body = JSON.parse(raw);
    if (!isObject(body)) return null;
    if (body.results !== undefined && body.results !== null && !Array.isArray(body.results)) return null;
    const results = (body.results || []).map((item) => {
      if (!isObject(item)) throw new DecodeError('resultado inválido');
      return {
        module: str(item.module),
        collected_at: time(item.collected_at),
        // json.RawMessage: se conserva cualquier JSON (null cuenta como vacío).
        data: item.data === undefined || item.data === null ? null : item.data,
        error: str(item.error),
        duration_ms: num(item.duration_ms, { integer: true }),
      };
    });
    return {
      schema: str(body.schema), agent_id: str(body.agent_id), hostname: str(body.hostname),
      version: str(body.version), os: str(body.os), arch: str(body.arch),
      tags: stringArray(body.tags), sequence: num(body.sequence, { unsigned: true, integer: true }),
      timestamp: time(body.timestamp), results,
    };
  } catch { return null; }
}

// Serialización igual a json.Marshal(model.Envelope): mismo orden de campos
// y campos omitempty (tags, data, error) fuera cuando están vacíos.
export function encodeEnvelope(env) {
  const out = { schema: env.schema, agent_id: env.agent_id, hostname: env.hostname, version: env.version, os: env.os, arch: env.arch };
  if (env.tags && env.tags.length) out.tags = env.tags;
  out.sequence = env.sequence;
  out.timestamp = env.timestamp;
  out.results = env.results.map((result) => {
    const item = { module: result.module, collected_at: result.collected_at };
    if (result.data !== null) item.data = result.data;
    if (result.error) item.error = result.error;
    item.duration_ms = result.duration_ms;
    return item;
  });
  return JSON.stringify(out);
}

// json.Marshal de []string nil produce "null"; el servidor de Go guarda eso.
export function encodeTags(tags) {
  return JSON.stringify(tags && tags.length ? tags : (tags ? [] : null));
}

export function decodeHeartbeat(raw) {
  try {
    const body = JSON.parse(raw);
    if (!isObject(body)) return null;
    return {
      agent_id: str(body.agent_id), hostname: str(body.hostname), version: str(body.version), os: str(body.os), arch: str(body.arch),
      uptime_seconds: num(body.uptime_seconds, { integer: true }), active_modules: stringArray(body.active_modules),
      self_cpu_percent: num(body.self_cpu_percent), self_mem_mb: num(body.self_mem_mb), timestamp: time(body.timestamp),
    };
  } catch { return null; }
}

export function decodeCommandResult(raw) {
  try {
    const body = JSON.parse(raw);
    if (!isObject(body)) return null;
    return {
      command_id: str(body.command_id), agent_id: str(body.agent_id), ok: bool(body.ok),
      output: str(body.output), error: str(body.error), finished_at: time(body.finished_at),
    };
  } catch { return null; }
}

export function encodeCommandResult(result) {
  const out = { command_id: result.command_id, agent_id: result.agent_id, ok: result.ok };
  if (result.output) out.output = result.output;
  if (result.error) out.error = result.error;
  out.finished_at = result.finished_at;
  return JSON.stringify(out);
}

// fmt %g de Go = strconv.FormatFloat(f, 'g', -1, 64): dígitos mínimos que
// reproducen el valor y, con esa precisión, notación exponencial cuando el
// exponente decimal es < -4 o >= 6 (p. ej. 1234567 -> "1.234567e+06",
// distinto de JS). El exponente lleva al menos dos dígitos.
export function goFormatG(value) {
  if (Number.isNaN(value)) return 'NaN';
  if (value === Infinity) return '+Inf';
  if (value === -Infinity) return '-Inf';
  if (value === 0) return Object.is(value, -0) ? '-0' : '0';
  const [mantissa, expPart] = value.toExponential().split('e');
  const exp = Number(expPart);
  if (exp >= -4 && exp < 6) return value.toString();
  const absExp = String(Math.abs(exp)).padStart(2, '0');
  return `${mantissa}e${exp < 0 ? '-' : '+'}${absExp}`;
}

// fmt %q de Go para las etiquetas de /metrics (strconv.Quote).
export function goQuote(text) {
  let out = '"';
  for (const char of String(text)) {
    const code = char.codePointAt(0);
    if (char === '"') out += '\\"';
    else if (char === '\\') out += '\\\\';
    else if (char === '\n') out += '\\n';
    else if (char === '\r') out += '\\r';
    else if (char === '\t') out += '\\t';
    else if (char === '\x07') out += '\\a';
    else if (char === '\b') out += '\\b';
    else if (char === '\f') out += '\\f';
    else if (char === '\v') out += '\\v';
    else if (code < 0x20 || code === 0x7f) out += `\\x${code.toString(16).padStart(2, '0')}`;
    else out += char;
  }
  return `${out}"`;
}

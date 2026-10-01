// GENERADO desde MRTI-Platform@363afc8 (src/request-context.js).
// No editar aquí: cambiar en MRTI-Platform y ejecutar bin/sync.mjs.
// Identificador de solicitud y log de acceso comunes a los backends MRTI.
//
// - Acepta el X-Request-Id que pone Nginx ($request_id) o genera uno nuevo.
// - Lo devuelve en la respuesta y lo deja disponible en getRequestId() para
//   que las llamadas a otros módulos (core-auth, clientes HTTP) lo reenvíen.
// - Escribe una línea JSON por solicitud relevante en stdout (pm2 la guarda).
import { AsyncLocalStorage } from 'node:async_hooks';
import { randomUUID } from 'node:crypto';

const storage = new AsyncLocalStorage();
const VALID_REQUEST_ID = /^[A-Za-z0-9._-]{8,64}$/;
const MUTATING = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

export const REQUEST_ID_HEADER = 'X-Request-Id';

export function getRequestId() {
  return storage.getStore()?.requestId ?? null;
}

// Headers para reenviar en fetch hacia otro módulo MRTI.
export function propagationHeaders(extra = {}) {
  const requestId = getRequestId();
  return requestId ? { ...extra, [REQUEST_ID_HEADER]: requestId } : { ...extra };
}

// Modo de log: MRTI_ACCESS_LOG=all | default | off.
// default registra escrituras, errores (>=400) y solicitudes lentas; las
// lecturas exitosas se omiten para no llenar los logs de pm2.
export function shouldLog({ mode, method, status, ms, slowMs }) {
  if (mode === 'off') return false;
  if (mode === 'all') return true;
  return MUTATING.has(method) || status >= 400 || ms >= slowMs;
}

export function logEvent(service, event, fields = {}, write = console.log) {
  write(JSON.stringify({ ts: new Date().toISOString(), svc: service, evt: event, rid: getRequestId(), ...fields }));
}

export function requestContext({
  service,
  mode = process.env.MRTI_ACCESS_LOG || 'default',
  slowMs = Number(process.env.MRTI_SLOW_REQUEST_MS || 2000),
  skip = (req) => req.path === '/api/health',
  write = console.log,
} = {}) {
  if (!service) throw new Error('requestContext requiere el nombre del servicio');
  return function mrtiRequestContext(req, res, next) {
    const incoming = req.get(REQUEST_ID_HEADER);
    const requestId = incoming && VALID_REQUEST_ID.test(incoming) ? incoming : randomUUID();
    req.requestId = requestId;
    res.setHeader(REQUEST_ID_HEADER, requestId);

    const started = process.hrtime.bigint();
    res.on('finish', () => {
      if (skip(req)) return;
      const ms = Number((process.hrtime.bigint() - started) / 1000000n);
      if (!shouldLog({ mode, method: req.method, status: res.statusCode, ms, slowMs })) return;
      write(JSON.stringify({
        ts: new Date().toISOString(),
        svc: service,
        evt: 'http',
        rid: requestId,
        method: req.method,
        // Sin query string: puede traer tokens de descarga o datos personales.
        path: (req.originalUrl || req.url).split('?')[0],
        status: res.statusCode,
        ms,
        user: req.portalUser?.id || req.user?.id || null,
      }));
    });

    storage.run({ requestId }, next);
  };
}

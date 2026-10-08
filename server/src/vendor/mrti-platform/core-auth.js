// GENERADO desde MRTI-Platform@7f88163 (src/core-auth.js).
// No editar aquí: cambiar en MRTI-Platform y ejecutar bin/sync.mjs.
// Validación de sesión contra MRTI Core para módulos consumidores.
//
// Ningún módulo decodifica el JWT: reenvía el header Authorization a Core
// (`/api/auth/module-access/:code` y `/api/auth/me`) y traduce la respuesta.
// Contrato (CORE_INFRA_MIGRATION_GUIDE.md §8): sin token 401, sesión inválida
// 401, sin módulo 403, Core no disponible 503.
import { logEvent, propagationHeaders } from './request-context.js';

const LEGACY_REPORT_INTERVAL_MS = 10 * 60 * 1000;

export function createCoreAuth({
  service,
  moduleCode,
  coreUrl = process.env.MRTI_CORE_URL,
  // Fallback de la Fase 4. Cada uso queda registrado como
  // `legacy_auth_fallback` para medir el tráfico antes de retirarlo (Fase 5).
  legacyUrl = process.env.MRTI_INFRA_URL || 'http://127.0.0.1:3002',
  timeoutMs = 5000,
  fetchImpl = (...args) => fetch(...args),
  write = console.log,
} = {}) {
  if (!service) throw new Error('createCoreAuth requiere el nombre del servicio');
  const baseUrl = coreUrl || legacyUrl;
  const usingLegacy = !coreUrl;
  let legacyCount = 0;
  let legacyReportedAt = 0;

  if (usingLegacy) {
    console.warn(`[${service}] MRTI_CORE_URL no está definida; validando sesiones contra MRTI_INFRA_URL (ruta obsoleta).`);
  }

  function noteLegacyUse(route) {
    if (!usingLegacy) return;
    legacyCount += 1;
    const now = Date.now();
    if (now - legacyReportedAt < LEGACY_REPORT_INTERVAL_MS) return;
    legacyReportedAt = now;
    logEvent(service, 'legacy_auth_fallback', { target: baseUrl, route, count: legacyCount }, write);
  }

  function callCore(route, authorization) {
    noteLegacyUse(route);
    return fetchImpl(`${baseUrl}${route}`, {
      headers: propagationHeaders({ Authorization: authorization }),
      signal: AbortSignal.timeout(timeoutMs),
    });
  }

  function moduleAccessRequiredFor(code) {
    const route = `/api/auth/module-access/${encodeURIComponent(code)}`;
    return async function requireModuleAccess(req, res, next) {
      const authorization = req.headers.authorization;
      if (!authorization) {
        return res.status(401).json({ error: 'No autenticado' });
      }
      try {
        const query = code === moduleCode
          ? `?${new URLSearchParams({ method: req.method || 'GET', path: (req.originalUrl || req.url || '/').split('?')[0] })}` : '';
        const response = await callCore(route + query, authorization);
        if (response.status === 204) return next();
        if (response.status === 401) {
          return res.status(401).json({ error: 'No autenticado o sesión expirada' });
        }
        if (response.status === 403) {
          return res.status(403).json({ error: 'Tu área no tiene acceso a este módulo' });
        }
        return res.status(503).json({ error: 'No se pudo validar la sesión con MRTI' });
      } catch {
        return res.status(503).json({ error: 'No se pudo contactar a MRTI Core/Infra' });
      }
    };
  }

  // Devuelve el usuario, null si la sesión no es válida, o lanza si Core no
  // responde: una caída de Core no significa que la sesión haya expirado.
  async function resolveCurrentUser(authorization) {
    if (!authorization) return null;
    const response = await callCore('/api/auth/me', authorization);
    if (response.status === 401 || response.status === 403) return null;
    if (!response.ok) throw new Error('Core no disponible');
    const body = await response.json();
    if (!body.profile?.id) throw new Error('Respuesta de identidad inválida');
    return { id: body.profile.id, name: body.profile.full_name, email: body.profile.email, role: body.profile.role };
  }

  // Lecturas auxiliares (auditoría, atribución): best-effort, nunca lanza.
  async function fetchCurrentUser(authorization) {
    try { return await resolveCurrentUser(authorization); } catch { return null; }
  }

  // Responde 401/503 y devuelve null si no hay usuario utilizable.
  async function requireCurrentUser(req, res) {
    try {
      const user = await resolveCurrentUser(req.headers.authorization);
      if (!user) res.status(401).json({ error: 'Sesión inválida o expirada' });
      return user;
    } catch {
      res.status(503).json({ error: 'No se pudo validar la sesión con MRTI Core' });
      return null;
    }
  }

  // Autoservicio: sólo exige una sesión válida, no acceso al módulo.
  async function portalSessionRequired(req, res, next) {
    const authorization = req.headers.authorization;
    if (!authorization) return res.status(401).json({ error: 'No autenticado' });
    const user = await requireCurrentUser(req, res);
    if (!user) return;
    req.portalUser = user;
    return next();
  }

  function administratorRequiredWith(message = 'Sólo un administrador puede realizar esta acción') {
    return async function requireAdministrator(req, res, next) {
      const user = req.portalUser || await requireCurrentUser(req, res);
      if (!user) return;
      if (String(user.role || '').toLowerCase() !== 'administrator') {
        return res.status(403).json({ error: message });
      }
      req.portalUser = user;
      return next();
    };
  }

  return {
    baseUrl,
    usingLegacy,
    moduleAccessRequiredFor,
    moduleAccessRequired: moduleCode ? moduleAccessRequiredFor(moduleCode) : undefined,
    resolveCurrentUser,
    fetchCurrentUser,
    requireCurrentUser,
    portalSessionRequired,
    administratorRequiredWith,
  };
}

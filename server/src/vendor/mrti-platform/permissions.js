// GENERADO desde MRTI-Platform@2d6505e (src/permissions.js).
// No editar aquí: cambiar en MRTI-Platform y ejecutar bin/sync.mjs.
// Contrato puro de permisos granulares. Core guarda políticas; los módulos
// conservan sus reglas de negocio y el alcance por registro/empresa/equipo.
export const LEVELS = ['none', 'read', 'write', 'full'];
export const LEVEL_LABELS = { none: 'Sin acceso', read: 'Sólo consulta', write: 'Consultar y editar', full: 'Gestión completa' };
const section = (id, name, routes, pages = []) => ({ id, name, routes, pages });
export const PERMISSION_CATALOG = [
  { code: 'rh', name: 'RH', base: '/rh', api: '/api/rh', sections: [
    section('dashboard', 'Resumen', ['dashboard'], ['']),
    section('indicators', 'Indicadores ejecutivos', ['indicators'], ['indicadores']),
    section('people', 'Directorio y expediente', ['employees','contpaq/employees','employee-documents','employee-labor','labor-document-'], ['empleados']),
    section('lifecycle', 'Altas y bajas integrales', ['lifecycle-'], ['ciclo-laboral']),
    section('organization', 'Organigrama, puestos y unidades', ['org','org-','orgchart','positions','position-','departments'], ['organigrama','estructura-organizacional','puestos','unidades']),
    section('history', 'Historial y movimientos', ['activity','activity-history','movement-report','audit-events'], ['historial']),
    section('payroll', 'Costo de nómina', ['payroll-cost'], ['costo-nomina']),
    section('contpaq', 'Control CONTPAQi', ['contpaq','contpaq-control','portal-accounts'], ['control-contpaq']),
    section('calendar', 'Calendario, ausencias y salas', ['celebration-','meeting-','leave-','holidays'], ['tiempo-espacios','calendario','celebraciones','salas','vacaciones']),
    section('news', 'Portada y noticias', ['company-news','company-home'], ['noticias']),
    section('work-plan', 'Plan de trabajo', ['work-plan'], ['plan-trabajo']),
    section('committee', 'Comité RH', ['committee'], ['comite-rh']),
    section('documents', 'Plantillas y documentos RH', ['documents'], ['documentos-rh']),
    section('tools', 'Herramientas y compensaciones', ['compensation-'], ['herramientas','compensaciones','firmas-correo']),
  ] },
  { code: 'activos', name: 'Activos', base: '/activos', api: '/api', sections: [
    section('dashboard', 'Dashboard', ['activos-suite/dashboard','activos-suite/summary'], ['','operacion']),
    section('alerts', 'Alertas', ['activos-suite/alerts','activos-suite/license-notifications'], ['alertas']),
    section('notes', 'Referencias técnicas', ['technical-notes','asset-knowledge'], ['referencias-tecnicas','notas-tecnicas']),
    section('offboarding', 'Bajas de personal', ['activos/offboarding'], ['bajas-personal']),
    section('catalogs', 'Catálogos operativos', ['activos-suite/resources','activos-suite/documents','activos-suite/unit-inventory'], ['catalogos','inventario/unidades','unidades']),
    section('security', 'Seguridad y credenciales', ['activos-suite/antivirus-licenses','activos-suite/teamviewer','activos/[^/]+/remission-credentials','activos/[^/]+/remission-credential-status'], ['seguridad-acceso']),
    section('checklist', 'Checklist de equipos', ['activos-suite/setup-checklist'], ['checklist-equipos']),
    section('administration', 'Revisión de unidades y sincronización', ['activos-suite/unit-review','activos-suite/sync'], ['administracion']),
    section('inventory', 'Inventario y asignaciones', ['activos'], ['inventario','nuevo','[0-9]+']),
  ] },
  { code: 'mrti-obs', name: 'Monitor', base: '/mrti-obs', api: '/api', sections: [
    section('monitoring', 'Monitoreo y métricas', ['monitoring','metrics','mrti','notifications'], ['','monitoring','servers','alerts','history']),
    section('topology', 'Sitios, planos y racks', ['db/sites','db/buildings','db/floors','db/areas','db/racks','db/floor_plans','db/device_positions','uploads'], ['sites','floor-plans','racks']),
    section('devices', 'Dispositivos', ['db/devices','db/device_','db/connections'], ['devices']),
    section('discovery', 'Descubrimiento', ['discovery'], ['discovery']),
    section('ups', 'UPS', ['ups'], ['ups']),
    section('credentials', 'Credenciales', ['credentials'], ['credentials']),
    section('settings', 'Configuración y catálogos', ['db','audit-events'], ['settings']),
    section('news', 'Pantalla de noticias', [], ['news-screen']),
  ] },
  { code: 'tickets', name: 'Tickets', base: '/tickets', api: '/api', sections: [
    section('dashboard', 'Centro operativo', ['dashboard'], ['']),
    section('reports', 'Reportes y SLA', ['reports','sla'], ['reports','sla']),
    section('knowledge', 'Base de conocimiento', ['kb-articles'], ['knowledge-base','kb']),
    section('settings', 'Equipos y configuración', ['business-areas','ticket-user-creation-limits','categories','subcategories','priorities','statuses','audit-events'], ['settings','categories','business-areas']),
    section('tickets', 'Bandeja y atención de tickets', ['tickets','assignees'], ['tickets','new','board']),
  ] },
  { code: 'mrti-legal', name: 'Legal', base: '/mrti-legal', api: '/api/legal', sections: [
    section('dashboard', 'Resumen', ['dashboard'], ['']),
    section('records', 'Expedientes, contratos y documentos', ['cases','contracts','permits','corporate-documents','powers','documents','versions','search','events','links'], ['asuntos','contratos','permisos','documentos','buscar','expedientes']),
    section('representatives', 'Poderes y representantes', ['representatives'], ['representantes']),
    section('obligations', 'Obligaciones', ['obligations'], ['obligaciones']),
    section('alerts', 'Alertas', ['alerts'], ['alertas']),
    section('roles', 'Roles del módulo', ['roles','module-roles'], ['roles']),
    section('audit', 'Auditoría', ['audit'], ['auditoria']),
  ] },
  { code: 'compras', name: 'SAP', base: '/compras', api: '/api/compras', sections: [
    ...[['resumen','Resumen ejecutivo','executive',''],['seguimiento','Seguimiento de compras','tracking','seguimiento'],['pagos','Pagos y saldos','payments','pagos'],['presupuesto','Presupuesto','budget','presupuesto'],['inventario','Inventario','inventory','inventario'],['flota','Flota y equipo','fleet','flota'],['sat69b','Alerta SAT 69-B','sat69b','sat-69b'],['proveedores','Proveedores','suppliers','proveedores'],['contabilidad','Contabilidad','ledger','contabilidad'],['controles','Controles y auditoría','controls','controles'],['settings','Permisos y sincronización','permissions','permisos']].map(([id,name,route,page]) => section(id,name,[route],[page])),
  ] },
  { code: 'mantenimiento', name: 'Mantenimiento', base: '/mantenimiento', api: '/api/mantenimiento', sections: [
    section('dashboard', 'Resumen', ['dashboard'], ['']),
    section('plans', 'Planes preventivos', ['plans','equipment/[^/]+/plans'], ['planes']),
    section('equipment', 'Equipos e importación', ['equipment','sap'], ['equipos']),
    section('readings', 'Horómetros y lecturas', ['readings'], ['lecturas']),
    section('orders', 'Órdenes de trabajo', ['orders'], ['ordenes']),
  ] },
  { code: 'agent-core', name: 'Agent Core', base: '/agent-core', api: '/api/v1', sections: [
    section('commands', 'Comandos remotos', ['agents/[^/]+/commands','commands'], []),
    section('agents', 'Agentes y telemetría', ['agents','alerts','export'], ['']),
  ] },
];
export const canonicalModule = (code) => code === 'mrti-infra' ? 'mrti-obs' : code;
export function routeSection(code, url, browser = false) {
  const module = PERMISSION_CATALOG.find((m) => m.code === canonicalModule(code));
  if (!module) return null;
  let path;
  try { path = decodeURIComponent(String(url).split('?')[0]).toLowerCase().replace(/\/{2,}/g, '/'); } catch { return null; }
  const base = browser ? module.base : module.api;
  if (path !== base && !path.startsWith(base + '/')) return null;
  path = path.slice(base.length).replace(/^\/+|\/+$/g, '');
  for (const item of module.sections) {
    for (const prefix of browser ? item.pages : item.routes) {
      if (prefix === '' ? path === '' : new RegExp(`^(?:${prefix})(?:/|$|(?<=[-_/]))`).test(path)) return item.id;
    }
  }
  return null;
}
export function levelAllows(level, method) {
  if (level === 'legacy' || level === 'full') return true;
  if (level === 'none') return false;
  if (['GET','HEAD','OPTIONS'].includes(method)) return level === 'read' || level === 'write';
  return level === 'write' && ['POST','PUT','PATCH'].includes(method);
}
export function permissionFor(profile, code, sectionId = null) {
  code = canonicalModule(code);
  const allowed = profile?.is_active !== false && (profile?.role === 'administrator' || profile?.allowed_modules?.includes(code));
  if (!allowed) return 'none';
  if (profile?.role === 'administrator') return 'full';
  const config = profile?.access_policy?.[code];
  return config?.sections?.[sectionId] ?? config?.level ?? 'legacy';
}
export function requestAllowed(profile, code, method, url) {
  const module = PERMISSION_CATALOG.find((m) => m.code === canonicalModule(code));
  let path;
  try { path = decodeURIComponent(String(url).split('?')[0]).toLowerCase().replace(/\/{2,}/g, '/'); } catch { return false; }
  const relative = module && path.slice(module.api.length);
  // Metadatos de sesión sin datos de negocio: permiten construir un menú acotado.
  if (['/me','/session','/catalog','/meta','/my-module-role','/physical-areas','/companies'].includes(relative) && ['GET','HEAD'].includes(method)) return profile?.allowed_modules?.includes(canonicalModule(code)) || profile?.role === 'administrator';
  const level = permissionFor(profile, code, routeSection(code, url));
  if (/\/(archive|restore|undelete|retire|delete|restricted-access|permissions|secret)(\/|$)/.test(path) && !['legacy','full'].includes(level)) return false;
  return levelAllows(level, method);
}

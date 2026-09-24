# Alineación visual de Agent Core

Fecha: 2026-09-24.

## Resultado

Agent Core usa el mismo lenguaje visual de la plataforma: marca MRTI con nombre
de módulo, navegación global autorizada, menú lateral adaptable, temas claro y
oscuro, campanilla consolidada, identidad y menú de cuenta.

El contenido operativo incorpora:

- Encabezado y explicación de la pantalla.
- Resumen de agentes registrados, en línea, sin conexión y alertas recientes.
- Tarjetas accesibles por equipo con estado, sistema, arquitectura, versión,
  último reporte y consumo propio del agente.
- Alertas dentro de un panel consistente y detalle completo en diálogo.
- Textos operativos en español, estados vacíos y adaptación a móvil.
- Iconos de origen correctos en la campanilla para Tickets, RH, Activos, Legal
  y Agent.

Las rutas, almacenamiento y contratos de telemetría no cambiaron. El servicio
sigue atendiendo directamente en `:8477`; Nginx conserva por ahora su redirección
compatible desde `/agent-core`. En acceso directo se usa el logo MRTI embebido
para evitar una solicitud de imagen entre puertos bloqueada por el navegador.

## Evidencia

- `go test ./...` correcto.
- Build limpio de `cmd/mrti-monitor` con Go 1.26.5 temporal.
- `gofmt -d` y `git diff --check` sin diferencias o errores.
- Smoke aislado y publicado en 1440, 390 y 320 px: navegación global, cuenta,
  resumen, tarjetas, diálogo, tema y menú móvil sin errores JavaScript ni
  desbordamiento horizontal.
- Fixture de Core retirado; cero residuos.
- `mrti-monitor.service` quedó activo con el binario nuevo.

Capturas: `/tmp/mrti-agent-core-visual-1440.png`,
`/tmp/mrti-agent-core-visual-390.png` y
`/tmp/mrti-agent-core-visual-320.png`.

## Rollback

El binario anterior está en
`/tmp/mrti-agent-core-visual-q9TQSc/mrti-monitor.previous`. Para volver,
restaurarlo atómicamente en `/var/www/mrt/MRTI/bin/mrti-monitor` y reiniciar el
proceso. Revertir el commit restaura también el código fuente. No hay migración
ni cambio de datos.

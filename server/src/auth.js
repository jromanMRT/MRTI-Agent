// La sesión la emite MRTI Core; la validación es la pieza compartida de
// MRTI-Platform (vendor/mrti-platform/core-auth.js), contrato 401/403/503.
import { createCoreAuth } from './vendor/mrti-platform/core-auth.js';

const auth = createCoreAuth({ service: 'mrti-agent-core-api', moduleCode: 'agent-core' });

export const moduleAccessRequired = auth.moduleAccessRequired;

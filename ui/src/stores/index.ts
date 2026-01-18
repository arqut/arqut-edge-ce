// Export all stores from a single entry point for EN to extend

import { createPinia } from 'pinia';

export * from './ui';
export * from './drawer';
export * from './proxyServices';
export * from './integrations';

// Default export required by Quasar
export default function () {
  return createPinia();
}

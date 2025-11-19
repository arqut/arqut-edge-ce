# Arqut Edge CE UI

Vue 3 + Quasar UI for Arqut Edge Community Edition.

## Architecture

This UI is designed to be extended by the Enterprise Edition (EN). Key extension points:

### Layout Extensions
- `MainLayout.vue` provides slots for:
  - `header-actions` - Additional header buttons
  - `nav-items` - Additional navigation items
  - `page-wrapper` - Wrap pages with additional functionality

### Page Extensions
- `ProxyServicesPage.vue` provides slots for:
  - `header-actions` - Additional header buttons (e.g., export, analytics)
  - `service-actions` - Additional service action buttons
  - `mobile-menu-items` - Additional mobile menu items
  - `additional-panels` - Additional panels below service list

### Form Extensions
- `ProxyServiceForm.vue` provides slots for:
  - `additional-fields` - Additional form fields for EN features

## Development

```bash
# Install dependencies
npm install

# Start dev server (proxies /api to localhost:3030)
npm run dev

# Build for production
npm run build

# Lint
npm run lint

# Format
npm run format
```
# Hellfire Web UI

Modern, type-safe web interface for Hellfire router configuration built with React 19, Vite, and TanStack Router.

## Stack

- **React 19** - Latest React with improved performance
- **TypeScript** - Full type safety
- **Vite** - Fast development and build tooling
- **TanStack Router** - File-based routing with type-safe navigation
- **TanStack Query** - Server state management and caching
- **Tailwind CSS** - Utility-first styling
- **shadcn/ui** - High-quality, accessible components
- **Auto-generated API Client** - Type-safe API client from OpenAPI spec

## Architecture

### Simple & Extensible Design

```
web/
├── src/
│   ├── routes/              # File-based routing (1 file = 1 route)
│   │   ├── __root.tsx       # Root layout with navigation
│   │   ├── index.tsx        # Dashboard (/)
│   │   ├── network.tsx      # Network config (/network)
│   │   ├── firewall.tsx     # Firewall rules (/firewall)
│   │   ├── dhcp.tsx         # DHCP settings (/dhcp)
│   │   └── system.tsx       # System settings (/system)
│   ├── lib/
│   │   ├── api/             # Auto-generated API client
│   │   └── utils.ts         # Utility functions (cn, etc)
│   ├── components/
│   │   └── ui/              # shadcn components (copy when needed)
│   └── hooks/
│       └── use-api.ts       # TanStack Query wrappers
├── openapi.json             # OpenAPI spec (copied from Go)
└── vite.config.ts           # Vite config with API proxy
```

### Key Principles

1. **Convention over Configuration** - File-based routing, minimal setup
2. **Co-located Logic** - Each route owns its data fetching and mutations
3. **Type Safety Everywhere** - OpenAPI → TypeScript client → React components
4. **Easy to Extend** - Add a route? Create one file. Update API? Regenerate client.

## Development

### First Time Setup

```bash
# From project root
just web-install          # Install dependencies
just web-generate-client  # Generate API client from OpenAPI spec
```

### Development Workflow

```bash
# Run backend + frontend together
just dev

# Or run separately:
just serve      # Backend on :8080
just web-dev    # Frontend on :5173 (proxies /api to :8080)
```

### Working with the API

1. **Update Go API handlers** with Swagger annotations
2. **Regenerate client**:
   ```bash
   just web-generate-client
   ```
3. **Use in components**:
   ```tsx
   import { useQuery } from '@tanstack/react-query'
   import { getConfig } from '@/lib/api'

   function Network() {
     const { data } = useQuery({
       queryKey: ['config', 'network'],
       queryFn: () => getConfig({ name: 'network' })
     })

     return <div>{/* Use data */}</div>
   }
   ```

### Adding a New Route

Create a file in `src/routes/`:

```tsx
// src/routes/monitoring.tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/monitoring')({
  component: Monitoring,
})

function Monitoring() {
  return <div>Monitoring page</div>
}
```

The route is now available at `/monitoring` with full type safety!

### Adding shadcn Components

```bash
# From web directory
npx shadcn@latest add button
npx shadcn@latest add card
npx shadcn@latest add form
```

Components are copied to `src/components/ui/` - modify as needed.

## Production Build

```bash
# Build web UI
just web-build

# Build everything (Go + Web)
just build-all-full

# Start production server
./bin/hf serve --port 8080
# Web UI available at http://localhost:8080
# API at http://localhost:8080/api
```

## Project Structure

### Routes (`src/routes/`)

File-based routing powered by TanStack Router:

- `__root.tsx` - Root layout, navigation, global providers
- `index.tsx` - Dashboard (`/`)
- `network.tsx` - Network configuration (`/network`)
- `firewall.tsx` - Firewall rules (`/firewall`)
- `dhcp.tsx` - DHCP settings (`/dhcp`)
- `system.tsx` - System configuration (`/system`)

### API Client (`src/lib/api/`)

Auto-generated from OpenAPI spec:

- Fully typed request/response
- Auto-complete for all endpoints
- Runtime validation
- Error handling

### Components (`src/components/`)

- `ui/` - shadcn components (copy/paste, fully customizable)
- Custom app components as needed

### Styling

- Tailwind CSS for utilities
- CSS variables for theming (light/dark mode ready)
- shadcn design system

## Configuration Files

- `vite.config.ts` - Vite config, path aliases, API proxy
- `tailwind.config.js` - Tailwind config
- `tsconfig.app.json` - TypeScript config with path aliases
- `openapi-ts.config.ts` - API client generation config
- `components.json` - shadcn config

## API Proxy (Development)

In development, Vite proxies `/api/*` to the Go backend:

- Frontend: `http://localhost:5173`
- Backend: `http://localhost:8080`
- API calls from frontend → proxied to backend
- No CORS issues

## Type Safety

The entire stack is type-safe:

1. Go handlers → OpenAPI spec (via Swagger)
2. OpenAPI spec → TypeScript types
3. TypeScript types → React components
4. File routes → Type-safe navigation

Example:

```tsx
// All of this is fully typed!
import { useNavigate } from '@tanstack/react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getConfig, setConfigOption } from '@/lib/api'

const navigate = useNavigate()
navigate({ to: '/network' }) // Type-checked!

const { data } = useQuery({
  queryKey: ['network'],
  queryFn: () => getConfig({ name: 'network' })
})

const mutation = useMutation({
  mutationFn: setConfigOption
})
```

## Extending the UI

### Add a New Configuration Page

1. Create route file: `src/routes/vpn.tsx`
2. Add API endpoints in Go with Swagger annotations
3. Run `just web-generate-client`
4. Use generated API client in your component

### Add a Custom Hook

```tsx
// src/hooks/use-network-config.ts
import { useQuery } from '@tanstack/react-query'
import { getConfig } from '@/lib/api'

export function useNetworkConfig() {
  return useQuery({
    queryKey: ['config', 'network'],
    queryFn: () => getConfig({ name: 'network' })
  })
}
```

### Add Shared Components

Add to `src/components/` and import with `@/` alias:

```tsx
import { MyComponent } from '@/components/my-component'
```

## Learn More

- [TanStack Router](https://tanstack.com/router) - File-based routing
- [TanStack Query](https://tanstack.com/query) - Data fetching
- [Tailwind CSS](https://tailwindcss.com) - Styling
- [shadcn/ui](https://ui.shadcn.com) - Components
- [Vite](https://vitejs.dev) - Build tool

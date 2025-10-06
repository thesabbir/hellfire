# Authentication Implementation

## Overview
The Hellfire web application uses **cookie-based authentication** with automatic token injection via request interceptors.

## Architecture

### Key Files

1. **`src/lib/api-client.ts`** - Custom API client wrapper (not auto-generated)
   - Sets up request interceptor for auth token
   - Won't be overwritten by API generation

2. **`src/lib/auth.tsx`** - Auth context and React hooks
   - Manages auth state
   - Handles login/logout
   - Cookie management

3. **`src/lib/api/client.gen.ts`** - Auto-generated API client
   - Base client configuration
   - **Gets overwritten** when running `npm run generate-client`

## How It Works

### 1. Cookie Storage
Authentication tokens are stored in cookies (not localStorage):

```typescript
const AUTH_COOKIE_NAME = 'hellfire_auth_token'

// Set cookie on login/onboarding
Cookies.set(AUTH_COOKIE_NAME, token, {
  expires: 7, // 7 days
  sameSite: 'strict',
  secure: window.location.protocol === 'https:',
})
```

**Benefits of cookies over localStorage:**
- Automatic inclusion in requests
- HttpOnly option (if set server-side)
- SameSite protection against CSRF
- Automatic expiration
- Better security practices

### 2. Request Interceptor

The auth token is automatically injected into every API request:

```typescript
// src/lib/api-client.ts
generatedClient.interceptors.request.use((request) => {
  const token = Cookies.get(AUTH_COOKIE_NAME)
  if (token) {
    request.headers.set('Authorization', `Bearer ${token}`)
  }
  return request
})
```

This interceptor:
- Runs before every API request
- Reads the cookie dynamically (always gets fresh value)
- Adds `Authorization: Bearer <token>` header
- Works even after page refreshes

### 3. Auth Context

```typescript
// Check auth on app load
async function checkAuth() {
  const token = Cookies.get(AUTH_COOKIE_NAME)
  if (!token) return

  const response = await getAuthMe()
  if (response.data) {
    setUser(response.data)
  } else {
    Cookies.remove(AUTH_COOKIE_NAME)
  }
}
```

## Why This Approach?

### Problem: API Client Generation
The API client at `src/lib/api/client.gen.ts` is auto-generated from the OpenAPI spec. Any customizations made directly to this file will be **lost** when regenerating the client.

### Solution: Wrapper Pattern
We created `src/lib/api-client.ts` which:
1. Imports the generated client
2. Adds the auth interceptor
3. Re-exports the enhanced client

This way:
- ✅ Custom auth logic is preserved
- ✅ API client can be regenerated safely
- ✅ Auth works automatically for all requests

## File Relationships

```
src/lib/
├── api/
│   ├── client.gen.ts       # Auto-generated base client
│   ├── sdk.gen.ts          # Auto-generated API methods
│   ├── types.gen.ts        # Auto-generated types
│   └── index.ts            # Re-exports + custom client
│
├── api-client.ts           # ✨ Custom wrapper (auth interceptor)
└── auth.tsx                # Auth context & React hooks
```

## API Generation Workflow

When you regenerate the API client:

```bash
# This overwrites client.gen.ts AND runs post-generation script
npm run generate-client

# The post-generation script automatically adds the custom client export
# to src/lib/api/index.ts after regeneration
```

### Automatic Post-Generation

The `npm run generate-client` script now runs two steps:

1. **Generate API client** - Overwrites `client.gen.ts`, `sdk.gen.ts`, `types.gen.ts`, `index.ts`
2. **Post-generation script** - Automatically adds the custom export to `index.ts`:

```typescript
// Re-export the custom client with auth interceptor
export { client } from '../api-client';
```

This is handled by `scripts/post-generate.sh` which:
- ✅ Checks if the export already exists (idempotent)
- ✅ Adds the export if missing
- ✅ Preserves your custom auth setup

So when you import from `@/lib/api`, you get the enhanced client with auth.

## Usage Examples

### Login

```typescript
import { useAuth } from '@/lib/auth'

function LoginPage() {
  const { login } = useAuth()

  const handleSubmit = async (username: string, password: string) => {
    await login(username, password)
    // Token is automatically stored in cookie
    // All subsequent API calls include the token
  }
}
```

### Protected API Calls

```typescript
import { getConfigByName } from '@/lib/api'

// Token is automatically added by interceptor
const response = await getConfigByName({
  path: { name: 'firewall' }
})
```

### Logout

```typescript
const { logout } = useAuth()

// Removes cookie and redirects to login
await logout()
```

## Authentication Flow

1. **App Start**
   - Check for `hellfire_auth_token` cookie
   - If exists, validate with `/auth/me`
   - Set user in auth context

2. **Login/Onboarding**
   - Call `/auth/login` or `/onboarding`
   - Receive token in response
   - Store in cookie
   - Update auth context

3. **API Requests**
   - Interceptor reads cookie
   - Adds `Authorization` header
   - Request proceeds with auth

4. **Logout**
   - Call `/auth/logout`
   - Remove cookie
   - Clear auth context
   - Redirect to login

## Security Considerations

### Current Implementation
- ✅ SameSite=strict (CSRF protection)
- ✅ Secure flag in production (HTTPS only)
- ✅ 7-day expiration
- ✅ Token in Authorization header (not cookie)

### Future Enhancements
- Consider HttpOnly cookies (requires backend change)
- Implement token refresh mechanism
- Add CSRF token for state-changing requests
- Consider short-lived access tokens + refresh tokens

## Common Issues & Solutions

### Issue: Auth not persisting after refresh
**Cause:** Cookie not being set properly
**Solution:** Check browser dev tools > Application > Cookies

### Issue: 401 Unauthorized on API calls
**Cause:** Interceptor not adding token
**Solution:** Verify interceptor is registered in `api-client.ts`

### Issue: Changes to client.gen.ts lost
**Cause:** File is auto-generated
**Solution:** Put customizations in `api-client.ts` instead

## Development Notes

### Don't Edit These Files
- `src/lib/api/client.gen.ts`
- `src/lib/api/sdk.gen.ts`
- `src/lib/api/types.gen.ts`

### Safe to Edit
- `src/lib/api-client.ts` - Auth interceptor
- `src/lib/auth.tsx` - Auth logic
- `src/lib/api/index.ts` - Re-exports (though it's also generated)

### Testing Auth

```typescript
// Check if cookie exists
import Cookies from 'js-cookie'
import { AUTH_COOKIE_NAME } from '@/lib/api-client'

const token = Cookies.get(AUTH_COOKIE_NAME)
console.log('Auth token:', token)
```

## References

- Cookie library: [js-cookie](https://github.com/js-cookie/js-cookie)
- API client generator: [@hey-api/openapi-ts](https://github.com/hey-api/openapi-ts)
- Auth best practices: [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)

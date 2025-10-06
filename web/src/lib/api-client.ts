/**
 * Custom API client with authentication interceptor
 * This file won't be overwritten by the API client generator
 *
 * Using hey-api's native interceptor support:
 * https://heyapi.dev/openapi-ts/clients/fetch.html#interceptors
 */
import Cookies from 'js-cookie'
import { client as generatedClient } from './api/client.gen'

export const AUTH_COOKIE_NAME = 'hellfire_auth_token'

// Add request interceptor to dynamically add auth header
// This runs before every request and always reads the fresh cookie value
generatedClient.interceptors.request.use((request) => {
  const token = Cookies.get(AUTH_COOKIE_NAME)
  if (token) {
    request.headers.set('Authorization', `Bearer ${token}`)
  }
  return request
})

// Export the client with interceptor configured
export const client = generatedClient

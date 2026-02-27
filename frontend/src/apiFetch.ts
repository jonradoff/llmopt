// Authenticated fetch wrapper — injects JWT + tenant headers on every API call.

import { getAccessToken, getActiveTenant, clearAuth } from './auth';

export async function apiFetch(
  url: string,
  options: RequestInit = {}
): Promise<Response> {
  const token = getAccessToken();
  const tenantId = getActiveTenant();

  const headers = new Headers(options.headers);
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  if (tenantId) {
    headers.set('X-Tenant-ID', tenantId);
  }

  const resp = await fetch(url, { ...options, headers });

  // On 401 with an existing token, the token is invalid/expired — clear and redirect.
  // If there's no token, the user was never logged in — don't redirect.
  if (resp.status === 401 && token) {
    clearAuth();
  }

  return resp;
}

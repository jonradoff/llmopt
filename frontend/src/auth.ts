// Auth utilities — reads tokens from localStorage shared with LastSaaS frontend.

const TOKEN_KEY = 'lastsaas_access_token';
const REFRESH_KEY = 'lastsaas_refresh_token';
const TENANT_KEY = 'lastsaas_active_tenant';

export function getAccessToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function getActiveTenant(): string | null {
  return localStorage.getItem(TENANT_KEY);
}

export function isLoggedIn(): boolean {
  return !!getAccessToken();
}

export function clearAuth(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
  localStorage.removeItem(TENANT_KEY);
  window.location.href = '/login';
}

export interface UserInfo {
  id: string;
  email: string;
  displayName: string;
  role?: string; // role in active tenant
  isRootTenant?: boolean;
}

export async function fetchCurrentUser(): Promise<UserInfo | null> {
  const token = getAccessToken();
  const tenantId = getActiveTenant();
  if (!token || !tenantId) return null;

  try {
    const resp = await fetch('/api/auth/me', {
      headers: {
        Authorization: `Bearer ${token}`,
        'X-Tenant-ID': tenantId,
      },
    });
    if (!resp.ok) return null;
    const data = await resp.json();

    // Determine role in active tenant and whether it's root
    const membership = data.memberships?.find(
      (m: { tenantId: string }) => m.tenantId === tenantId
    );

    return {
      id: data.user?.id || data.id,
      email: data.user?.email || data.email,
      displayName: data.user?.displayName || data.displayName || '',
      role: membership?.role,
      isRootTenant: membership?.isRoot ?? false,
    };
  } catch {
    return null;
  }
}

export async function fetchUnreadCount(): Promise<number> {
  const token = getAccessToken();
  const tenantId = getActiveTenant();
  if (!token || !tenantId) return 0;

  try {
    const resp = await fetch('/api/messages/unread-count', {
      headers: {
        Authorization: `Bearer ${token}`,
        'X-Tenant-ID': tenantId,
      },
    });
    if (!resp.ok) return 0;
    const data = await resp.json();
    return data.count ?? 0;
  } catch {
    return 0;
  }
}

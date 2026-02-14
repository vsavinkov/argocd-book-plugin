const EXTENSION_BASE = '/extensions/booking';

export interface BookingStatus {
  booked: boolean;
  bookedBy?: string;
  bookedAt?: string;
}

let cachedUsername: string | null = null;

async function getUsername(): Promise<string> {
  if (cachedUsername) return cachedUsername;
  const resp = await fetch('/api/v1/session/userinfo', { credentials: 'same-origin' });
  if (!resp.ok) throw new Error('Failed to get user info');
  const info = await resp.json();
  cachedUsername = info.username || info.loggedIn && 'unknown';
  return cachedUsername || 'unknown';
}

function authFetch(url: string, opts: RequestInit = {}): Promise<Response> {
  return fetch(url, {
    ...opts,
    credentials: 'same-origin',
  });
}

export async function getStatus(appName: string, project: string): Promise<BookingStatus> {
  const resp = await authFetch(`${EXTENSION_BASE}/api/status`, {
    headers: {
      'Argocd-Application-Name': appName,
      'Argocd-Project-Name': project,
    },
  });
  if (!resp.ok) {
    throw new Error(`Failed to get booking status: ${resp.statusText}`);
  }
  return resp.json();
}

export async function bookApp(appName: string, project: string): Promise<void> {
  const username = await getUsername();
  const resp = await authFetch(`${EXTENSION_BASE}/api/book`, {
    method: 'POST',
    headers: {
      'Argocd-Application-Name': appName,
      'Argocd-Project-Name': project,
      'Argocd-Username': username,
    },
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}));
    throw new Error(body.error || `Failed to book: ${resp.statusText}`);
  }
}

export async function unbookApp(appName: string, project: string): Promise<void> {
  const username = await getUsername();
  const resp = await authFetch(`${EXTENSION_BASE}/api/unbook`, {
    method: 'POST',
    headers: {
      'Argocd-Application-Name': appName,
      'Argocd-Project-Name': project,
      'Argocd-Username': username,
    },
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}));
    throw new Error(body.error || `Failed to unbook: ${resp.statusText}`);
  }
}

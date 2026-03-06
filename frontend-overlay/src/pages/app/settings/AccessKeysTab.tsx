import { useState, useEffect, useCallback } from 'react';
import { Key, Plus, Trash2, Copy, Check, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';

// Authenticated fetch — injects the JWT + tenant headers that the SaaS middleware requires.
function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const token = localStorage.getItem('lastsaas_access_token');
  const tenantId = localStorage.getItem('lastsaas_active_tenant');
  const headers = new Headers(options.headers);
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (tenantId) headers.set('X-Tenant-ID', tenantId);
  headers.set('Content-Type', 'application/json');
  return fetch(url, { ...options, headers });
}

interface AccessKey {
  id: string;
  name: string;
  key_preview: string;
  created_at: string;
  last_used_at?: string;
  is_active: boolean;
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

export default function AccessKeysTab() {
  const [keys, setKeys] = useState<AccessKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  const [copiedKey, setCopiedKey] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const fetchKeys = useCallback(async () => {
    setLoading(true);
    try {
      const res = await authFetch('/api/user/access-keys');
      if (res.ok) {
        const data = await res.json();
        setKeys(data.keys ?? []);
      }
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchKeys(); }, [fetchKeys]);

  const handleCreate = async () => {
    if (!newKeyName.trim()) return;
    setCreating(true);
    try {
      const res = await authFetch('/api/user/access-keys', {
        method: 'POST',
        body: JSON.stringify({ name: newKeyName.trim() }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        toast.error(err.error || 'Failed to create key');
        return;
      }
      const data = await res.json();
      setRevealedKey(data.raw_key);
      setNewKeyName('');
      setShowCreateForm(false);
      await fetchKeys();
    } catch {
      toast.error('Failed to create key');
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      const res = await authFetch(`/api/user/access-keys/${id}`, { method: 'DELETE' });
      if (res.ok || res.status === 204) {
        toast.success('Access key revoked');
        setKeys(prev => prev.filter(k => k.id !== id));
      } else {
        toast.error('Failed to revoke key');
      }
    } catch {
      toast.error('Failed to revoke key');
    } finally {
      setDeletingId(null);
    }
  };

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedKey(true);
      setTimeout(() => setCopiedKey(false), 2000);
    } catch {
      toast.error('Failed to copy to clipboard');
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-lg font-semibold text-white flex items-center gap-2">
          <Key className="w-5 h-5 text-primary-400" />
          Access Keys
        </h2>
        <p className="text-dark-400 text-sm mt-1">
          Create API keys to connect MCP clients (Claude Code, Claude Desktop, etc.) to your LLM Optimizer account.
          Keys start with <code className="text-primary-300 bg-dark-800 px-1 py-0.5 rounded text-xs">lok_</code>.
        </p>
      </div>

      {/* One-time reveal modal */}
      {revealedKey && (
        <div className="bg-amber-500/10 border border-amber-500/30 rounded-xl p-4 space-y-3">
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
            <div>
              <p className="text-amber-300 font-semibold text-sm">Save your key now — it won't be shown again</p>
              <p className="text-amber-400/80 text-xs mt-0.5">Copy this key and store it somewhere safe.</p>
            </div>
          </div>
          <div className="flex items-center gap-2 bg-dark-900 border border-dark-700 rounded-lg p-3">
            <code className="flex-1 text-sm text-white font-mono break-all select-all">{revealedKey}</code>
            <button
              onClick={() => handleCopy(revealedKey)}
              className="shrink-0 p-1.5 rounded-lg bg-dark-700 hover:bg-dark-600 text-dark-300 hover:text-white transition-colors"
              title="Copy key"
            >
              {copiedKey ? <Check className="w-4 h-4 text-accent-emerald" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
          <button
            onClick={() => setRevealedKey(null)}
            className="text-xs text-amber-400/70 hover:text-amber-300 transition-colors"
          >
            I've saved my key — dismiss
          </button>
        </div>
      )}

      {/* Create form */}
      {showCreateForm ? (
        <div className="bg-dark-800/50 border border-dark-700 rounded-xl p-4 space-y-3">
          <p className="text-sm font-medium text-white">New Access Key</p>
          <input
            type="text"
            value={newKeyName}
            onChange={e => setNewKeyName(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleCreate()}
            placeholder="Key name (e.g. Claude Code, Home laptop)"
            className="w-full bg-dark-900 border border-dark-700 rounded-lg px-3 py-2 text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 transition-colors"
            maxLength={64}
            autoFocus
          />
          <div className="flex gap-2">
            <button
              onClick={handleCreate}
              disabled={creating || !newKeyName.trim()}
              className="px-4 py-2 text-sm font-medium bg-primary-600 hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg transition-colors"
            >
              {creating ? 'Creating…' : 'Create Key'}
            </button>
            <button
              onClick={() => { setShowCreateForm(false); setNewKeyName(''); }}
              className="px-4 py-2 text-sm font-medium text-dark-400 hover:text-white bg-dark-700 hover:bg-dark-600 rounded-lg transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <button
          onClick={() => setShowCreateForm(true)}
          className="flex items-center gap-2 px-4 py-2 text-sm font-medium bg-primary-600 hover:bg-primary-500 text-white rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Access Key
        </button>
      )}

      {/* Key list */}
      {loading ? (
        <div className="text-dark-500 text-sm">Loading…</div>
      ) : keys.length === 0 ? (
        <div className="bg-dark-800/30 border border-dark-700/50 rounded-xl p-6 text-center">
          <Key className="w-8 h-8 text-dark-600 mx-auto mb-2" />
          <p className="text-dark-400 text-sm">No access keys yet.</p>
          <p className="text-dark-500 text-xs mt-1">Create one above to connect MCP clients.</p>
        </div>
      ) : (
        <div className="space-y-2">
          {keys.map(key => (
            <div key={key.id} className="flex items-center gap-3 bg-dark-800/50 border border-dark-700/50 rounded-xl px-4 py-3">
              <Key className="w-4 h-4 text-dark-500 shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-white truncate">{key.name}</p>
                <p className="text-xs text-dark-500 font-mono">{key.key_preview}</p>
              </div>
              <div className="text-right shrink-0">
                <p className="text-xs text-dark-500">Created {formatDate(key.created_at)}</p>
                {key.last_used_at && (
                  <p className="text-xs text-dark-600">Last used {formatDate(key.last_used_at)}</p>
                )}
              </div>
              <button
                onClick={() => handleDelete(key.id)}
                disabled={deletingId === key.id}
                className="shrink-0 p-1.5 rounded-lg text-dark-500 hover:text-red-400 hover:bg-red-500/10 disabled:opacity-50 transition-colors"
                title="Revoke key"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Usage help */}
      <div className="bg-dark-800/30 border border-dark-700/30 rounded-xl p-4 text-xs text-dark-400 space-y-1">
        <p className="font-medium text-dark-300">How to use</p>
        <p>When connecting an MCP client to <code className="text-primary-400">https://llmopt.metavert.io/mcp</code>, enter your <code className="text-primary-400">lok_</code> key in the authorization prompt instead of your password.</p>
        <p>Each key grants full access to your account's MCP tools. Revoke any key you no longer need.</p>
      </div>
    </div>
  );
}

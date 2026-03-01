import { useState, useEffect, useCallback } from 'react';
import { User, KeyRound, CheckCircle, AlertCircle, Download, Trash2, HelpCircle, RefreshCw, X, DollarSign, Star } from 'lucide-react';
import { toast } from 'sonner';
import { useAuth } from '../../../contexts/AuthContext';
import { useTenant } from '../../../contexts/TenantContext';
import { authApi } from '../../../api/client';
import { getErrorMessage } from '../../../utils/errors';

// --- Reusable from upstream ---

function PasswordStrength({ password }: { password: string }) {
  const checks = [
    { label: '10+ characters', met: password.length >= 10 },
    { label: 'Uppercase letter', met: /[A-Z]/.test(password) },
    { label: 'Lowercase letter', met: /[a-z]/.test(password) },
    { label: 'Number', met: /\d/.test(password) },
    { label: 'Special character', met: /[^A-Za-z0-9]/.test(password) },
  ];
  const score = checks.filter(c => c.met).length;
  const strength = score <= 2 ? 'Weak' : score <= 3 ? 'Fair' : score <= 4 ? 'Good' : 'Strong';
  const color = score <= 2 ? 'bg-red-500' : score <= 3 ? 'bg-amber-500' : score <= 4 ? 'bg-primary-500' : 'bg-accent-emerald';
  const textColor = score <= 2 ? 'text-red-400' : score <= 3 ? 'text-amber-400' : score <= 4 ? 'text-primary-400' : 'text-accent-emerald';

  return (
    <div className="mt-2">
      <div className="flex items-center gap-2 mb-1.5">
        <div className="flex-1 h-1.5 bg-dark-700 rounded-full overflow-hidden flex gap-0.5">
          {[1, 2, 3, 4, 5].map(i => (
            <div key={i} className={`flex-1 rounded-full transition-colors ${i <= score ? color : 'bg-dark-700'}`} />
          ))}
        </div>
        <span className={`text-xs font-medium ${textColor}`}>{strength}</span>
      </div>
      <div className="flex flex-wrap gap-x-3 gap-y-0.5">
        {checks.map(c => (
          <span key={c.label} className={`text-xs ${c.met ? 'text-accent-emerald' : 'text-dark-500'}`}>
            {c.met ? '\u2713' : '\u2717'} {c.label}
          </span>
        ))}
      </div>
    </div>
  );
}

// --- API Key helpers ---

interface APIKeyInfo {
  id: string;
  tenant_id: string;
  provider: string;
  key_prefix: string;
  preferred_model: string;
  status: string;
  last_verified_at: string | null;
  created_at: string;
  updated_at: string;
}

function getAuthHeaders(): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = localStorage.getItem('lastsaas_access_token');
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const tenantId = localStorage.getItem('lastsaas_active_tenant');
  if (tenantId) headers['X-Tenant-ID'] = tenantId;
  return headers;
}

function handleAuthError(resp: Response): void {
  if (resp.status === 401) {
    localStorage.removeItem('lastsaas_access_token');
    localStorage.removeItem('lastsaas_refresh_token');
    window.location.href = '/login';
  }
}

async function fetchAPIKeys(): Promise<APIKeyInfo[]> {
  const resp = await fetch('/api/settings/api-keys', { headers: getAuthHeaders() });
  if (!resp.ok) { handleAuthError(resp); return []; }
  const data = await resp.json();
  return data.keys || [];
}

async function saveAPIKey(provider: string, key: string, preferredModel: string): Promise<APIKeyInfo> {
  const resp = await fetch(`/api/settings/api-keys/${provider}`, {
    method: 'PUT',
    headers: getAuthHeaders(),
    body: JSON.stringify({ key, preferred_model: preferredModel }),
  });
  if (!resp.ok) {
    handleAuthError(resp);
    const err = await resp.json().catch(() => ({ error: 'Failed to save key' }));
    throw new Error(err.error || 'Failed to save key');
  }
  return resp.json();
}

async function deleteAPIKey(provider: string): Promise<void> {
  const resp = await fetch(`/api/settings/api-keys/${provider}`, {
    method: 'DELETE',
    headers: getAuthHeaders(),
  });
  if (!resp.ok) { handleAuthError(resp); throw new Error('Failed to remove key'); }
}

async function verifyAPIKey(provider: string): Promise<{ status: string }> {
  const resp = await fetch(`/api/settings/api-keys/${provider}/verify`, {
    method: 'POST',
    headers: getAuthHeaders(),
  });
  if (!resp.ok) { handleAuthError(resp); throw new Error('Verification failed'); }
  return resp.json();
}

async function fetchPrimaryProvider(): Promise<string> {
  const resp = await fetch('/api/settings/primary-provider', { headers: getAuthHeaders() });
  if (!resp.ok) return 'anthropic';
  const data = await resp.json();
  return data.primary_provider || 'anthropic';
}

async function setPrimaryProvider(provider: string): Promise<void> {
  const resp = await fetch('/api/settings/primary-provider', {
    method: 'PUT',
    headers: getAuthHeaders(),
    body: JSON.stringify({ provider }),
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ message: 'Failed to set primary provider' }));
    throw new Error(err.message || 'Failed to set primary provider');
  }
}

// --- Provider definitions ---

interface ProviderDef {
  id: string;
  name: string;
  description: string;
  models: { id: string; name: string }[];
  placeholder: string;
  helpSteps: { text: string; link?: { url: string; label: string } }[];
  billingUrl: string;
  billingLabel: string;
}

const PROVIDERS: ProviderDef[] = [
  {
    id: 'anthropic',
    name: 'Anthropic',
    description: 'Claude models for analysis',
    models: [
      { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6 (Recommended)' },
      { id: 'claude-haiku-4-5-20251001', name: 'Claude Haiku 4.5 (Faster, lower cost)' },
    ],
    placeholder: 'sk-ant-...',
    helpSteps: [
      { text: 'Go to console.anthropic.com and sign in (or create an account).', link: { url: 'https://console.anthropic.com', label: 'console.anthropic.com' } },
      { text: 'Navigate to Manage \u2192 API Keys.' },
      { text: 'Click Create Key, give it a name, and copy the key.' },
      { text: 'Paste the key here. Your key is encrypted and stored securely.' },
    ],
    billingUrl: 'https://console.anthropic.com/settings/billing',
    billingLabel: 'console.anthropic.com/settings/billing',
  },
  {
    id: 'openai',
    name: 'OpenAI',
    description: 'GPT models for analysis',
    models: [
      { id: 'gpt-4o', name: 'GPT-4o (Recommended)' },
      { id: 'gpt-4o-mini', name: 'GPT-4o Mini (Faster, lower cost)' },
    ],
    placeholder: 'sk-...',
    helpSteps: [
      { text: 'Go to platform.openai.com and sign in (or create an account).', link: { url: 'https://platform.openai.com', label: 'platform.openai.com' } },
      { text: 'Navigate to API Keys in the left sidebar.' },
      { text: 'Click Create new secret key, give it a name, and copy the key.' },
      { text: 'Paste the key here. Your key is encrypted and stored securely.' },
    ],
    billingUrl: 'https://platform.openai.com/settings/organization/billing/overview',
    billingLabel: 'platform.openai.com billing',
  },
  {
    id: 'grok',
    name: 'Grok',
    description: 'xAI models for analysis',
    models: [
      { id: 'grok-3', name: 'Grok 3 (Recommended)' },
      { id: 'grok-3-mini', name: 'Grok 3 Mini (Faster, lower cost)' },
    ],
    placeholder: 'xai-...',
    helpSteps: [
      { text: 'Go to console.x.ai and sign in (or create an account).', link: { url: 'https://console.x.ai', label: 'console.x.ai' } },
      { text: 'Navigate to API Keys.' },
      { text: 'Click Create API Key, give it a name, and copy the key.' },
      { text: 'Paste the key here. Your key is encrypted and stored securely.' },
    ],
    billingUrl: 'https://console.x.ai/billing',
    billingLabel: 'console.x.ai/billing',
  },
  {
    id: 'gemini',
    name: 'Gemini',
    description: 'Google AI models for analysis',
    models: [
      { id: 'gemini-2.5-pro', name: 'Gemini 2.5 Pro (Recommended)' },
      { id: 'gemini-2.0-flash', name: 'Gemini 2.0 Flash (Faster, lower cost)' },
    ],
    placeholder: 'AIza...',
    helpSteps: [
      { text: 'Go to aistudio.google.com and sign in with your Google account.', link: { url: 'https://aistudio.google.com', label: 'aistudio.google.com' } },
      { text: 'Click Get API key in the left sidebar.' },
      { text: 'Click Create API key, select a project, and copy the key.' },
      { text: 'Paste the key here. Your key is encrypted and stored securely.' },
    ],
    billingUrl: 'https://aistudio.google.com',
    billingLabel: 'aistudio.google.com',
  },
];

// --- Help Modal ---

function ProviderHelpModal({ provider, onClose }: { provider: ProviderDef; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-lg" role="dialog" aria-modal="true">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-white">How to Get a {provider.name} API Key</h3>
          <button onClick={onClose} className="text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <ol className="space-y-3 text-sm text-dark-300">
          {provider.helpSteps.map((step, i) => (
            <li key={i} className="flex gap-3">
              <span className="flex-shrink-0 w-6 h-6 rounded-full bg-primary-500/20 text-primary-400 flex items-center justify-center text-xs font-bold">{i + 1}</span>
              <span>
                {step.link ? (
                  <>
                    {step.text.split(step.link.label)[0]}
                    <a href={step.link.url} target="_blank" rel="noopener noreferrer" className="text-primary-400 hover:text-primary-300 underline">{step.link.label}</a>
                    {step.text.split(step.link.label)[1]}
                  </>
                ) : step.text}
              </span>
            </li>
          ))}
        </ol>
        <div className="mt-4 p-3 bg-dark-800 rounded-lg">
          <p className="text-xs text-dark-400">
            You'll need credits on your {provider.name} account.{' '}
            Go to <a href={provider.billingUrl} target="_blank" rel="noopener noreferrer" className="text-dark-300 underline hover:text-white">{provider.billingLabel}</a> to manage billing.
          </p>
        </div>
        <button
          onClick={onClose}
          className="mt-4 w-full py-2 bg-dark-800 text-dark-300 text-sm font-medium rounded-lg hover:bg-dark-700 transition-colors"
        >
          Got it
        </button>
      </div>
    </div>
  );
}

// --- Status Badge ---

function statusBadge(status: string) {
  switch (status) {
    case 'active':
      return <span className="flex items-center gap-1 text-xs text-accent-emerald"><CheckCircle className="w-3.5 h-3.5" /> Active</span>;
    case 'invalid':
      return <span className="flex items-center gap-1 text-xs text-red-400"><AlertCircle className="w-3.5 h-3.5" /> Invalid Key</span>;
    case 'no_credits':
      return <span className="flex items-center gap-1 text-xs text-amber-400"><DollarSign className="w-3.5 h-3.5" /> No Credits</span>;
    default:
      return <span className="flex items-center gap-1 text-xs text-dark-400"><AlertCircle className="w-3.5 h-3.5" /> Unknown</span>;
  }
}

// --- Credit Refill Helper ---

function CreditRefillHelper({ provider }: { provider: ProviderDef }) {
  return (
    <div className="mt-2 p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg">
      <div className="flex items-start gap-2">
        <DollarSign className="w-4 h-4 text-amber-400 mt-0.5 flex-shrink-0" />
        <div className="text-xs text-amber-300">
          <p className="font-medium mb-1">Your API key needs credits</p>
          <p className="text-amber-400">
            Go to <a href={provider.billingUrl} target="_blank" rel="noopener noreferrer" className="underline hover:text-amber-300">{provider.billingLabel}</a> to add credits.
          </p>
        </div>
      </div>
    </div>
  );
}

// --- Generalized Provider Card ---

function ProviderKeyCard({ provider, existingKey, isPrimary, onUpdate, onSetPrimary }: {
  provider: ProviderDef;
  existingKey: APIKeyInfo | null;
  isPrimary: boolean;
  onUpdate: () => void;
  onSetPrimary: () => void;
}) {
  const [keyInput, setKeyInput] = useState('');
  const [model, setModel] = useState(existingKey?.preferred_model || provider.models[0].id);
  const [saving, setSaving] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [showHelp, setShowHelp] = useState(false);

  useEffect(() => {
    if (existingKey?.preferred_model) {
      setModel(existingKey.preferred_model);
    }
  }, [existingKey?.preferred_model]);

  const handleSave = async () => {
    if (!keyInput.trim()) return;
    setSaving(true);
    try {
      await saveAPIKey(provider.id, keyInput.trim(), model);
      setKeyInput('');
      toast.success(`${provider.name} API key saved and verified`);
      onUpdate();
      window.dispatchEvent(new Event('apikey-updated'));
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const handleVerify = async () => {
    setVerifying(true);
    try {
      const result = await verifyAPIKey(provider.id);
      toast.success(`${provider.name} key status: ${result.status}`);
      onUpdate();
      window.dispatchEvent(new Event('apikey-updated'));
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setVerifying(false);
    }
  };

  const handleRemove = async () => {
    setRemoving(true);
    try {
      await deleteAPIKey(provider.id);
      toast.success(`${provider.name} API key removed`);
      onUpdate();
      window.dispatchEvent(new Event('apikey-updated'));
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setRemoving(false);
    }
  };

  return (
    <div className={`bg-dark-800/50 border rounded-xl p-4 ${isPrimary ? 'border-primary-500/50' : 'border-dark-700'}`}>
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-white">{provider.name}</span>
          {existingKey && statusBadge(existingKey.status)}
          {isPrimary && (
            <span className="flex items-center gap-1 text-xs text-primary-400 bg-primary-500/10 px-1.5 py-0.5 rounded">
              <Star className="w-3 h-3" /> Primary
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {existingKey && existingKey.status === 'active' && !isPrimary && (
            <button
              onClick={onSetPrimary}
              className="text-xs text-dark-400 hover:text-primary-400 transition-colors"
              title="Set as primary provider"
            >
              Set Primary
            </button>
          )}
          <button onClick={() => setShowHelp(true)} className="text-dark-400 hover:text-white transition-colors" title="How to get an API key">
            <HelpCircle className="w-4 h-4" />
          </button>
        </div>
      </div>

      {existingKey ? (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <code className="text-sm text-dark-300 bg-dark-900 px-2 py-1 rounded">{existingKey.key_prefix}</code>
            <div className="flex items-center gap-2">
              <button
                onClick={handleVerify}
                disabled={verifying}
                className="flex items-center gap-1 text-xs text-dark-400 hover:text-white transition-colors disabled:opacity-50"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${verifying ? 'animate-spin' : ''}`} />
                Verify
              </button>
              <button
                onClick={handleRemove}
                disabled={removing}
                className="flex items-center gap-1 text-xs text-red-400 hover:text-red-300 transition-colors disabled:opacity-50"
              >
                <Trash2 className="w-3.5 h-3.5" />
                Remove
              </button>
            </div>
          </div>

          {existingKey.status === 'no_credits' && <CreditRefillHelper provider={provider} />}

          <div>
            <label className="block text-xs text-dark-400 mb-1">Preferred Model</label>
            <select
              value={model}
              onChange={async (e) => {
                const newModel = e.target.value;
                setModel(newModel);
                try {
                  await saveAPIKey(provider.id, '', newModel);
                  onUpdate();
                } catch {
                  // Model update without key change needs the existing key
                }
              }}
              className="w-full px-3 py-1.5 bg-dark-900 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500 transition-colors"
            >
              {provider.models.map(m => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs text-dark-400 mb-1">Replace Key</label>
            <div className="flex gap-2">
              <input
                type="password"
                value={keyInput}
                onChange={e => setKeyInput(e.target.value)}
                placeholder={provider.placeholder}
                className="flex-1 px-3 py-1.5 bg-dark-900 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 transition-colors"
              />
              <button
                onClick={handleSave}
                disabled={saving || !keyInput.trim()}
                className="px-3 py-1.5 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {saving ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-xs text-dark-400">Enter your {provider.name} API key to use {provider.name} for analysis.</p>
          <div>
            <label className="block text-xs text-dark-400 mb-1">Preferred Model</label>
            <select
              value={model}
              onChange={e => setModel(e.target.value)}
              className="w-full px-3 py-1.5 bg-dark-900 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500 transition-colors"
            >
              {provider.models.map(m => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </select>
          </div>
          <div className="flex gap-2">
            <input
              type="password"
              value={keyInput}
              onChange={e => setKeyInput(e.target.value)}
              placeholder={provider.placeholder}
              className="flex-1 px-3 py-1.5 bg-dark-900 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 transition-colors"
            />
            <button
              onClick={handleSave}
              disabled={saving || !keyInput.trim()}
              className="px-3 py-1.5 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
      )}

      {showHelp && <ProviderHelpModal provider={provider} onClose={() => setShowHelp(false)} />}
    </div>
  );
}

// --- Read-Only Provider Card (for non-owners) ---

function ProviderKeyCardReadOnly({ provider, existingKey, isPrimary, ownerName }: {
  provider: ProviderDef;
  existingKey: APIKeyInfo | null;
  isPrimary: boolean;
  ownerName: string;
}) {
  return (
    <div className={`bg-dark-800/50 border rounded-xl p-4 ${isPrimary ? 'border-primary-500/50' : 'border-dark-700'}`}>
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-white">{provider.name}</span>
          {existingKey && statusBadge(existingKey.status)}
          {isPrimary && (
            <span className="flex items-center gap-1 text-xs text-primary-400 bg-primary-500/10 px-1.5 py-0.5 rounded">
              <Star className="w-3 h-3" /> Primary
            </span>
          )}
        </div>
      </div>

      {existingKey ? (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <code className="text-sm text-dark-300 bg-dark-900 px-2 py-1 rounded">{existingKey.key_prefix}</code>
          </div>

          {existingKey.status === 'no_credits' && (
            <div className="p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg">
              <div className="flex items-start gap-2">
                <DollarSign className="w-4 h-4 text-amber-400 mt-0.5 flex-shrink-0" />
                <p className="text-xs text-amber-300">
                  The API key needs credits. {ownerName ? `Contact ${ownerName} to resolve this.` : 'Contact the team owner to resolve this.'}
                </p>
              </div>
            </div>
          )}

          {existingKey.status === 'invalid' && (
            <div className="p-3 bg-red-500/10 border border-red-500/20 rounded-lg">
              <div className="flex items-start gap-2">
                <AlertCircle className="w-4 h-4 text-red-400 mt-0.5 flex-shrink-0" />
                <p className="text-xs text-red-300">
                  The API key is invalid. {ownerName ? `Contact ${ownerName} to update it.` : 'Contact the team owner to update it.'}
                </p>
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs text-dark-400 mb-1">Preferred Model</label>
            <div className="px-3 py-1.5 bg-dark-900 border border-dark-700 rounded-lg text-sm text-dark-300">
              {provider.models.find(m => m.id === existingKey.preferred_model)?.name || existingKey.preferred_model || 'Default'}
            </div>
          </div>
        </div>
      ) : (
        <div className="text-xs text-dark-500">No key configured</div>
      )}
    </div>
  );
}

// --- Main ProfileTab ---

export default function ProfileTab() {
  const { user, refreshUser } = useAuth();
  const { role } = useTenant();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletePassword, setDeletePassword] = useState('');
  const [deleting, setDeleting] = useState(false);
  const [exporting, setExporting] = useState(false);

  // API Key state
  const [apiKeys, setApiKeys] = useState<APIKeyInfo[]>([]);
  const [keysLoading, setKeysLoading] = useState(true);
  const [ownerName, setOwnerName] = useState('');
  const [primaryProvider, setPrimaryProviderState] = useState('anthropic');

  const isOwner = role === 'owner';

  const loadKeys = useCallback(async () => {
    try {
      const [keys, primary] = await Promise.all([fetchAPIKeys(), fetchPrimaryProvider()]);
      setApiKeys(keys);
      setPrimaryProviderState(primary);
    } catch {
      // ignore
    } finally {
      setKeysLoading(false);
    }
  }, []);

  useEffect(() => {
    loadKeys();
    // Fetch owner name for non-owners
    if (!isOwner) {
      fetch('/api/settings/api-keys/status', { headers: getAuthHeaders() })
        .then(r => r.ok ? r.json() : null)
        .then(data => {
          if (data?.owner_name) setOwnerName(data.owner_name);
        })
        .catch(() => {});
    }
  }, [loadKeys, isOwner]);

  const handleSetPrimary = async (providerId: string) => {
    try {
      await setPrimaryProvider(providerId);
      setPrimaryProviderState(providerId);
      toast.success(`${PROVIDERS.find(p => p.id === providerId)?.name} set as primary provider`);
    } catch (err) {
      toast.error(getErrorMessage(err));
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');
    setChangingPassword(true);
    try {
      await authApi.changePassword(currentPassword, newPassword);
      setPasswordSuccess('Password changed successfully');
      setCurrentPassword('');
      setNewPassword('');
      toast.success('Password changed successfully');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setPasswordError(msg || 'Failed to change password');
    } finally {
      setChangingPassword(false);
    }
  };

  const handleExportData = async () => {
    setExporting(true);
    try {
      const blob = await authApi.exportData();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'account-data.json';
      a.click();
      URL.revokeObjectURL(url);
      toast.success('Data exported successfully');
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setExporting(false);
    }
  };

  const handleDeleteAccount = async () => {
    setDeleting(true);
    try {
      await authApi.deleteAccount(deletePassword);
      toast.success('Account deleted');
      window.location.href = '/login';
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setDeleting(false);
    }
  };

  const handleResendVerification = async () => {
    if (!user?.email) return;
    try {
      await authApi.resendVerification(user.email);
      await refreshUser();
    } catch {
      // ignore
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      {/* Profile */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
          <User className="w-5 h-5 text-dark-400" />
          Profile
        </h2>
        <div className="space-y-3">
          <div className="flex items-center justify-between py-2">
            <span className="text-sm text-dark-400">Name</span>
            <span className="text-sm text-white">{user?.displayName}</span>
          </div>
          <div className="flex items-center justify-between py-2 border-t border-dark-800">
            <span className="text-sm text-dark-400">Email</span>
            <span className="text-sm text-white">{user?.email}</span>
          </div>
          <div className="flex items-center justify-between py-2 border-t border-dark-800">
            <span className="text-sm text-dark-400">Email Verified</span>
            <div className="flex items-center gap-2">
              {user?.emailVerified ? (
                <span className="flex items-center gap-1 text-sm text-accent-emerald">
                  <CheckCircle className="w-4 h-4" /> Verified
                </span>
              ) : (
                <div className="flex items-center gap-2">
                  <span className="flex items-center gap-1 text-sm text-amber-400">
                    <AlertCircle className="w-4 h-4" /> Not verified
                  </span>
                  <button
                    onClick={handleResendVerification}
                    className="text-xs text-primary-400 hover:text-primary-300 transition-colors"
                  >
                    Resend
                  </button>
                </div>
              )}
            </div>
          </div>
          <div className="flex items-center justify-between py-2 border-t border-dark-800">
            <span className="text-sm text-dark-400">Auth Methods</span>
            <div className="flex gap-2">
              {user?.authMethods.map((method) => (
                <span key={method} className="px-2 py-0.5 bg-dark-800 rounded text-xs text-dark-300 capitalize">
                  {method}
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* API Keys */}
      <div id="api-keys" className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
          <KeyRound className="w-5 h-5 text-dark-400" />
          AI Providers
        </h2>
        <p className="text-sm text-dark-400 mb-4">
          {isOwner
            ? 'Connect your API keys to power AI analysis. Configure at least one provider to get started. The primary provider is used for all analyses.'
            : 'API keys are managed by the team owner.'}
        </p>

        {keysLoading ? (
          <div className="text-sm text-dark-500">Loading...</div>
        ) : (
          <div className="space-y-3">
            {PROVIDERS.map(provider => {
              const existingKey = apiKeys.find(k => k.provider === provider.id) || null;
              const isPrimary = primaryProvider === provider.id;
              return isOwner ? (
                <ProviderKeyCard
                  key={provider.id}
                  provider={provider}
                  existingKey={existingKey}
                  isPrimary={isPrimary}
                  onUpdate={loadKeys}
                  onSetPrimary={() => handleSetPrimary(provider.id)}
                />
              ) : (
                <ProviderKeyCardReadOnly
                  key={provider.id}
                  provider={provider}
                  existingKey={existingKey}
                  isPrimary={isPrimary}
                  ownerName={ownerName}
                />
              );
            })}
          </div>
        )}
      </div>

      {/* Change Password */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
          <KeyRound className="w-5 h-5 text-dark-400" />
          Change Password
        </h2>

        {passwordError && (
          <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{passwordError}</div>
        )}
        {passwordSuccess && (
          <div className="mb-4 bg-accent-emerald/10 border border-accent-emerald/20 rounded-lg p-3 text-sm text-accent-emerald">{passwordSuccess}</div>
        )}

        <form onSubmit={handleChangePassword} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Current Password</label>
            <input
              type="password"
              required
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">New Password</label>
            <input
              type="password"
              required
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
              placeholder="Min 10 chars, mixed case, number, special"
            />
            {newPassword && <PasswordStrength password={newPassword} />}
          </div>
          <button
            type="submit"
            disabled={changingPassword}
            className="py-2.5 px-6 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all text-sm"
          >
            {changingPassword ? 'Changing...' : 'Change Password'}
          </button>
        </form>
      </div>

      {/* Data Export */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-2">
          <Download className="w-5 h-5 text-dark-400" />
          Export My Data
        </h2>
        <p className="text-sm text-dark-400 mb-4">Download a JSON file containing your profile, memberships, and messages.</p>
        <button
          onClick={handleExportData}
          disabled={exporting}
          className="py-2 px-4 bg-dark-800 text-dark-200 text-sm font-medium rounded-lg hover:bg-dark-700 disabled:opacity-50 transition-colors"
        >
          {exporting ? 'Exporting...' : 'Download Data'}
        </button>
      </div>

      {/* Delete Account */}
      <div className="bg-red-500/5 border border-red-500/20 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-red-400 flex items-center gap-2 mb-2">
          <Trash2 className="w-5 h-5" />
          Delete Account
        </h2>
        <p className="text-sm text-dark-400 mb-4">
          Permanently delete your account and all associated data. This action cannot be undone.
        </p>
        <button
          onClick={() => setShowDeleteModal(true)}
          className="py-2 px-4 bg-red-500/10 text-red-400 text-sm font-medium rounded-lg border border-red-500/20 hover:bg-red-500/20 transition-colors"
        >
          Delete My Account
        </button>
      </div>

      {/* Delete Account Modal */}
      {showDeleteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setShowDeleteModal(false)} />
          <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-md" role="dialog" aria-modal="true">
            <h3 className="text-lg font-semibold text-red-400 mb-2">Delete Account</h3>
            <p className="text-sm text-dark-400 mb-4">
              This will permanently delete your account and all data. If you own any teams with other members, you must transfer ownership first.
            </p>
            {user?.authMethods.includes('password') && (
              <div className="mb-4">
                <label className="block text-sm font-medium text-dark-300 mb-1.5">Confirm your password</label>
                <input
                  type="password"
                  value={deletePassword}
                  onChange={e => setDeletePassword(e.target.value)}
                  className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-red-500 transition-colors"
                  placeholder="Enter your password"
                />
              </div>
            )}
            <div className="flex justify-end gap-3">
              <button
                onClick={() => { setShowDeleteModal(false); setDeletePassword(''); }}
                className="px-4 py-2 text-sm text-dark-400 hover:text-white transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleDeleteAccount}
                disabled={deleting || (user?.authMethods.includes('password') && !deletePassword)}
                className="px-4 py-2 bg-red-500 text-white text-sm font-medium rounded-lg hover:bg-red-600 disabled:opacity-50 transition-colors"
              >
                {deleting ? 'Deleting...' : 'Permanently Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

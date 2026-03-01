import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, Users, Settings, LogOut, Shield, ChevronDown, Bell, CreditCard, Zap,
  FileText, Image, Globe, Star, Heart, BookOpen, MessageCircle, HelpCircle, Upload, ArrowLeft,
  UserCircle, AlertTriangle, DollarSign,
} from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { useTenant } from '../contexts/TenantContext';
import { useBranding } from '../contexts/BrandingContext';
import { messagesApi, plansApi, bundlesApi } from '../api/client';
import { useState, useRef, useEffect, useCallback } from 'react';
import type { LucideIcon } from 'lucide-react';

const iconMap: Record<string, LucideIcon> = {
  LayoutDashboard, Users, Settings, CreditCard, FileText, Image, Globe, Shield, Zap, Star, Heart, BookOpen, MessageCircle, HelpCircle, Upload,
};

export default function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, isAuthenticated, logout, memberships } = useAuth();
  const { activeTenant, setActiveTenant } = useTenant();
  const { branding } = useBranding();
  const [showTenantMenu, setShowTenantMenu] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);
  const [showCredits, setShowCredits] = useState(false);
  const [tenantCredits, setTenantCredits] = useState(0);
  const [hasBundles, setHasBundles] = useState(false);
  const [showTeam, setShowTeam] = useState(true);
  const [healthLight, setHealthLight] = useState<'green' | 'amber' | 'red' | 'gray'>('gray');
  const [keyStatus, setKeyStatus] = useState<'active' | 'invalid' | 'no_credits' | 'unconfigured'>('unconfigured');
  const [hasKey, setHasKey] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const isActive = (path: string) => location.pathname === path || location.pathname.startsWith(path + '/');

  const fetchKeyStatus = useCallback(() => {
    const token = localStorage.getItem('lastsaas_access_token');
    const tenantId = localStorage.getItem('lastsaas_active_tenant');
    if (!token) return;
    const headers: Record<string, string> = { 'Authorization': `Bearer ${token}` };
    if (tenantId) headers['X-Tenant-ID'] = tenantId;
    fetch('/api/settings/api-keys/status', { headers })
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        if (data) {
          setHasKey(data.has_key || false);
          setKeyStatus(data.status || 'unconfigured');
        }
      })
      .catch(() => {});
  }, []);

  // Re-fetch key status when ProfileTab saves/deletes a key
  useEffect(() => {
    const handler = () => fetchKeyStatus();
    window.addEventListener('apikey-updated', handler);
    return () => window.removeEventListener('apikey-updated', handler);
  }, [fetchKeyStatus]);

  useEffect(() => {
    if (isAuthenticated) {
      messagesApi.unreadCount()
        .then((data) => setUnreadCount(data.count))
        .catch(() => {});
      plansApi.list()
        .then((data) => {
          const hasCredits = data.plans.some(p => p.usageCreditsPerMonth > 0 || p.bonusCredits > 0);
          setShowCredits(hasCredits);
          setTenantCredits(data.tenantSubscriptionCredits + data.tenantPurchasedCredits);
          setShowTeam(data.maxPlanUserLimit !== 1);
        })
        .catch(() => {});
      bundlesApi.list()
        .then((data) => setHasBundles(data.bundles.length > 0))
        .catch(() => {});
      // Fetch user's API key status
      fetchKeyStatus();
    }
    // Fetch health status (public endpoint, no auth needed)
    fetch('/api/health/history')
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        if (data?.length > 0) {
          const latest = data[0];
          const anyAvailable = latest.models?.some((m: { status: string }) => m.status === 'available');
          const allError = latest.models?.every((m: { status: string }) => m.status === 'error');
          setHealthLight(anyAvailable ? 'green' : allError ? 'red' : 'amber');
        }
      })
      .catch(() => {});
  }, [isAuthenticated]);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowTenantMenu(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  // LLM Optimizer nav: Back to App, Team, Plan, Settings
  const defaultNavItems = [
    ...(showTeam ? [{ path: '/last/team', icon: Users, label: 'Team' }] : []),
    { path: '/last/plan', icon: CreditCard, label: 'Plan' },
  ];

  const navItems = branding.navItems.length > 0
    ? branding.navItems
        .filter(item => item.visible)
        .filter(item => {
          if (item.id === 'team' && !showTeam) return false;
          if (item.id === 'dashboard') return false;
          return true;
        })
        .sort((a, b) => a.sortOrder - b.sortOrder)
        .map(item => ({
          path: item.target.startsWith('/last') ? item.target : `/last${item.target}`,
          icon: iconMap[item.icon] || FileText,
          label: item.label,
        }))
    : defaultNavItems;

  const appName = branding.appName || 'LLM Optimizer';
  const logoMode = branding.logoMode || 'text';
  const logoUrl = branding.logoUrl;

  return (
    <div className="min-h-screen bg-dark-950">
      {/* Header */}
      <header className="sticky top-0 z-50 bg-dark-900/80 backdrop-blur-xl border-b border-dark-800">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            {/* Logo + Nav */}
            <div className="flex items-center gap-6">
              {/* Back to App */}
              <a href="/" className="flex items-center gap-2 text-dark-400 hover:text-white transition-colors">
                <ArrowLeft className="w-4 h-4" />
                <span className="text-sm hidden sm:block">Back to App</span>
              </a>

              <Link to="/last/dashboard" className="flex items-center gap-2">
                {(logoMode === 'image' || logoMode === 'both') && logoUrl ? (
                  <img src={logoUrl} alt={appName} className="h-8 w-8 rounded-lg object-contain" />
                ) : (
                  <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center">
                    <span className="text-white font-bold text-sm">{appName.slice(0, 2).toUpperCase()}</span>
                  </div>
                )}
                {(logoMode === 'text' || logoMode === 'both') && (
                  <span className="font-semibold text-white hidden sm:block">{appName}</span>
                )}
              </Link>

              {isAuthenticated && (
                <nav className="hidden md:flex items-center gap-1">
                  {navItems.map((item) => (
                    <Link
                      key={item.path}
                      to={item.path}
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${
                        isActive(item.path) && !navItems.some(n => n.path !== item.path && n.path.startsWith(item.path + '/') && isActive(n.path))
                          ? 'bg-primary-500/20 text-primary-400'
                          : 'text-dark-400 hover:text-white hover:bg-dark-800/50'
                      }`}
                    >
                      <item.icon className="w-4 h-4" />
                      <span>{item.label}</span>
                    </Link>
                  ))}
                  {memberships.some(m => m.isRoot) && (
                    <Link
                      to="/last"
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${
                        location.pathname === '/last' || (location.pathname.startsWith('/last/') && !location.pathname.startsWith('/last/team') && !location.pathname.startsWith('/last/plan') && !location.pathname.startsWith('/last/settings') && !location.pathname.startsWith('/last/messages') && !location.pathname.startsWith('/last/dashboard') && !location.pathname.startsWith('/last/buy') && !location.pathname.startsWith('/last/billing'))
                          ? 'bg-accent-purple/20 text-accent-purple'
                          : 'text-dark-400 hover:text-white hover:bg-dark-800/50'
                      }`}
                    >
                      <Shield className="w-4 h-4" />
                      <span>Admin</span>
                    </Link>
                  )}
                </nav>
              )}
            </div>

            {/* Right side */}
            {isAuthenticated && (
              <div className="flex items-center gap-4">
                {/* Tenant Switcher */}
                {memberships.length > 1 && (
                  <div className="relative" ref={menuRef}>
                    <button
                      onClick={() => setShowTenantMenu(!showTenantMenu)}
                      className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-dark-800 border border-dark-700 text-sm text-dark-300 hover:text-white transition-colors"
                    >
                      <span className="max-w-[120px] truncate">{activeTenant?.tenantName}</span>
                      <ChevronDown className="w-3.5 h-3.5" />
                    </button>
                    {showTenantMenu && (
                      <div className="absolute right-0 mt-2 w-56 bg-dark-800 border border-dark-700 rounded-xl shadow-xl py-1 z-50">
                        {memberships.map((m) => (
                          <button
                            key={m.tenantId}
                            onClick={() => {
                              setActiveTenant(m);
                              setShowTenantMenu(false);
                            }}
                            className={`w-full text-left px-4 py-2.5 text-sm transition-colors ${
                              m.tenantId === activeTenant?.tenantId
                                ? 'bg-primary-500/10 text-primary-400'
                                : 'text-dark-300 hover:bg-dark-700 hover:text-white'
                            }`}
                          >
                            <div className="flex items-center justify-between">
                              <span className="truncate">{m.tenantName}</span>
                              <span className="text-xs text-dark-500 capitalize">{m.role}</span>
                            </div>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Credits indicator */}
                {showCredits && (
                  <button
                    onClick={() => navigate(hasBundles ? '/last/buy-credits' : '/last/plan')}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-dark-800 border border-dark-700 text-sm text-dark-300 hover:text-white hover:border-primary-500/30 transition-colors"
                    title="Usage credits"
                  >
                    <Zap className="w-4 h-4 text-primary-400" />
                    <span className="font-medium">{tenantCredits.toLocaleString()}</span>
                  </button>
                )}

                {/* Messages */}
                <Link
                  to="/last/messages"
                  className="relative text-dark-400 hover:text-white transition-colors"
                >
                  <Bell className="w-5 h-5" />
                  {unreadCount > 0 && (
                    <span className="absolute -top-1.5 -right-1.5 bg-primary-500 text-white text-[10px] font-medium rounded-full w-4 h-4 flex items-center justify-center">
                      {unreadCount}
                    </span>
                  )}
                </Link>

                {/* User info + Logout */}
                <Link
                  to="/last/settings"
                  className="flex items-center gap-1.5 text-sm text-dark-400 hover:text-white transition-colors hidden sm:flex"
                  title="Settings"
                >
                  <UserCircle className="w-4 h-4" />
                  <span>{user?.displayName}</span>
                </Link>
                <button
                  onClick={handleLogout}
                  className="flex items-center gap-2 text-dark-400 hover:text-white transition-colors"
                >
                  <LogOut className="w-4 h-4" />
                </button>

                {/* Status / API Key indicator — rightmost */}
                {!hasKey || keyStatus === 'unconfigured' ? (
                  <Link
                    to="/last/settings"
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-400 hover:bg-amber-500/20 transition-colors text-sm font-medium"
                    title="Configure your API key"
                  >
                    <AlertTriangle className="w-4 h-4" />
                    <span className="hidden sm:inline">API Key Needed</span>
                    <span className="sm:hidden">!</span>
                  </Link>
                ) : keyStatus === 'no_credits' ? (
                  <a
                    href="/?tab=status"
                    className="relative flex items-center gap-2 px-3 py-1.5 rounded-lg text-dark-400 hover:text-white hover:bg-dark-800/50 transition-colors"
                    title="API key needs credits"
                  >
                    <span className="relative">
                      <span className={`w-2.5 h-2.5 rounded-full inline-block ${
                        healthLight === 'green' ? 'bg-emerald-400' :
                        healthLight === 'red' ? 'bg-red-400' :
                        healthLight === 'amber' ? 'bg-amber-400' : 'bg-dark-600'
                      }`} />
                      <DollarSign className="w-3 h-3 text-amber-400 absolute -top-1.5 -right-2" />
                    </span>
                    <span className="text-sm">Status</span>
                  </a>
                ) : keyStatus === 'invalid' ? (
                  <Link
                    to="/last/settings"
                    className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 hover:bg-red-500/20 transition-colors text-sm"
                    title="API key is invalid"
                  >
                    <span className="w-2.5 h-2.5 rounded-full bg-red-400" />
                    <span className="text-sm">Key Invalid</span>
                  </Link>
                ) : (
                  <a
                    href="/?tab=status"
                    className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-dark-400 hover:text-white hover:bg-dark-800/50 transition-colors"
                    title="AI Model Status"
                  >
                    <span className={`w-2.5 h-2.5 rounded-full ${
                      healthLight === 'green' ? 'bg-emerald-400' :
                      healthLight === 'red' ? 'bg-red-400' :
                      healthLight === 'amber' ? 'bg-amber-400' : 'bg-dark-600'
                    }`} />
                    <span className="text-sm">Status</span>
                  </a>
                )}
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Outlet context={{ setUnreadCount, showTeam }} />
      </main>
    </div>
  );
}

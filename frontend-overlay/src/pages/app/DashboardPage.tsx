import { useEffect } from 'react';

// DashboardPage — redirects to the main LLM Optimizer app.
// The SaaS frontend only handles auth/admin/settings; the real app lives at /.
export default function DashboardPage() {
  useEffect(() => {
    window.location.href = '/';
  }, []);
  return null;
}

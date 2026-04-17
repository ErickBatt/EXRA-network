'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { supabase } from '@/lib/supabase';
import { fetchJson } from '@/lib/api';
import './admin.css';

type AdminTokenomicsResponse = {
  request_id: string;
  actor_email: string;
  policy_finalized: boolean;
  max_supply: number;
  swap_circuit_breaker: boolean;
  stats: {
    total_exra_pending_mint: number;
    total_exra_minted: number;
    total_exra_burned: number;
  };
};

type AdminQueueResponse = {
  request_id: string;
  actor_email: string;
  items: OracleQueueItem[];
};

type OracleQueueItem = {
  id: number;
  device_id: string;
  amount_exra: number;
  status: string;
  retry_count: number;
  error_text: string;
  dlq_reason: string;
};

type AdminPayoutResponse = {
  request_id: string;
  items: PayoutItem[];
};

type PayoutItem = {
  id: string;
  device_id: string;
  amount_usd: number;
  status: string;
  recipient_wallet: string;
  created_at: string;
};

type AdminIncidentsResponse = {
  request_id: string;
  summary: {
    failed_mint_queue: number;
    retryable_mint_queue: number;
    pending_payouts: number;
    swap_guard_active: boolean;
  };
};

export default function AdminPage() {
  const router = useRouter();
  const [sessionReady, setSessionReady] = useState(false);
  const [adminEmail, setAdminEmail] = useState('');
  const [adminSecret, setAdminSecret] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const [tokenomics, setTokenomics] = useState<AdminTokenomicsResponse | null>(null);
  const [queue, setQueue] = useState<OracleQueueItem[]>([]);
  const [payouts, setPayouts] = useState<PayoutItem[]>([]);
  const [incidents, setIncidents] = useState<AdminIncidentsResponse['summary'] | null>(null);

  useEffect(() => {
    const init = async () => {
      const { data: { session } } = await supabase.auth.getSession();
      if (!session) {
        router.push('/auth');
        return;
      }
      setSessionReady(true);
      const rememberedEmail = localStorage.getItem('exra_admin_email') || '';
      if (rememberedEmail) setAdminEmail(rememberedEmail);
    };
    init();
  }, [router]);

  const adminHeaders = useMemo(() => ({
    'X-Exra-Token': adminSecret,
    'X-Admin-Email': adminEmail,
  }), [adminSecret, adminEmail]);

  const ensureAdminAuth = () => {
    if (!adminEmail || !adminSecret) {
      setError('Enter admin email and admin secret');
      return false;
    }
    setError('');
    localStorage.setItem('exra_admin_email', adminEmail);
    return true;
  };

  const loadAll = async () => {
    if (!ensureAdminAuth()) return;
    setLoading(true);
    try {
      const [statsRes, queueRes, payoutsRes, incidentsRes] = await Promise.all([
        fetchJson<AdminTokenomicsResponse>('/api/admin/tokenomics/stats', undefined, { headers: adminHeaders }),
        fetchJson<AdminQueueResponse>('/api/admin/oracle/queue?limit=50', undefined, { headers: adminHeaders }),
        fetchJson<AdminPayoutResponse>('/api/admin/payouts', undefined, { headers: adminHeaders }),
        fetchJson<AdminIncidentsResponse>('/api/admin/incidents', undefined, { headers: adminHeaders }),
      ]);
      setTokenomics(statsRes);
      setQueue(queueRes.items || []);
      setPayouts(payoutsRes.items || []);
      setIncidents(incidentsRes.summary || null);
    } catch (e: any) {
      setError(e?.message || 'Failed to load admin data');
    } finally {
      setLoading(false);
    }
  };

  const retryQueueItem = async (id: number) => {
    if (!ensureAdminAuth()) return;
    try {
      await fetchJson(`/api/admin/oracle/queue/${id}/retry`, undefined, {
        method: 'POST',
        headers: adminHeaders,
      });
      await loadAll();
    } catch (e: any) {
      setError(e?.message || 'Retry failed');
    }
  };

  const processQueue = async () => {
    if (!ensureAdminAuth()) return;
    try {
      await fetchJson('/api/admin/oracle/process', undefined, {
        method: 'POST',
        headers: adminHeaders,
      });
      await loadAll();
    } catch (e: any) {
      setError(e?.message || 'Process queue failed');
    }
  };

  const approvePayout = async (id: string) => {
    if (!ensureAdminAuth()) return;
    try {
      await fetchJson(`/api/admin/payout/${id}/approve`, undefined, {
        method: 'POST',
        headers: adminHeaders,
      });
      await loadAll();
    } catch (e: any) {
      setError(e?.message || 'Approve payout failed');
    }
  };

  const rejectPayout = async (id: string) => {
    if (!ensureAdminAuth()) return;
    try {
      await fetchJson(`/api/admin/payout/${id}/reject`, undefined, {
        method: 'POST',
        headers: adminHeaders,
      });
      await loadAll();
    } catch (e: any) {
      setError(e?.message || 'Reject payout failed');
    }
  };

  if (!sessionReady) {
    return <div className="admin-root"><div className="admin-card">Loading...</div></div>;
  }

  return (
    <div className="admin-root">
      <div className="admin-header">
        <h1>Admin Console</h1>
        <div className="admin-links">
          <Link href="/marketplace">Marketplace</Link>
          <Link href="/">Site</Link>
        </div>
      </div>

      <div className="admin-card auth">
        <div className="admin-row">
          <input
            type="email"
            placeholder="admin email"
            value={adminEmail}
            onChange={(e) => setAdminEmail(e.target.value)}
          />
          <input
            type="password"
            placeholder="ADMIN_SECRET"
            value={adminSecret}
            onChange={(e) => setAdminSecret(e.target.value)}
          />
          <button onClick={loadAll} disabled={loading}>{loading ? 'Loading...' : 'Load'}</button>
          <button onClick={processQueue} disabled={loading}>Process Queue</button>
        </div>
        {error ? <div className="error">{error}</div> : null}
      </div>

      <div className="admin-grid">
        <div className="admin-card">
          <h3>Incidents</h3>
          <div className="kv">failed queue: <b>{incidents?.failed_mint_queue ?? 0}</b></div>
          <div className="kv">retryable queue: <b>{incidents?.retryable_mint_queue ?? 0}</b></div>
          <div className="kv">pending payouts: <b>{incidents?.pending_payouts ?? 0}</b></div>
          <div className="kv">swap guard: <b>{incidents?.swap_guard_active ? 'active' : 'normal'}</b></div>
        </div>

        <div className="admin-card">
          <h3>Tokenomics</h3>
          <div className="kv">policy finalized: <b>{tokenomics?.policy_finalized ? 'true' : 'false'}</b></div>
          <div className="kv">max supply: <b>{tokenomics?.max_supply ?? 0}</b></div>
          <div className="kv">pending mint: <b>{tokenomics?.stats?.total_exra_pending_mint ?? 0}</b></div>
          <div className="kv">minted: <b>{tokenomics?.stats?.total_exra_minted ?? 0}</b></div>
          <div className="kv">burned: <b>{tokenomics?.stats?.total_exra_burned ?? 0}</b></div>
        </div>
      </div>

      <div className="admin-card">
        <h3>Oracle Queue</h3>
        <div className="table">
          {queue.map((q) => (
            <div key={q.id} className="row">
              <span>#{q.id}</span>
              <span>{q.status}</span>
              <span>{q.amount_exra.toFixed(4)} EXRA</span>
              <span>retries: {q.retry_count}</span>
              <span>{q.error_text || q.dlq_reason || '-'}</span>
              {(q.status === 'failed' || q.status === 'retryable') ? (
                <button onClick={() => retryQueueItem(q.id)}>retry</button>
              ) : null}
            </div>
          ))}
          {queue.length === 0 ? <div className="row">No queue items</div> : null}
        </div>
      </div>

      <div className="admin-card">
        <h3>Payouts</h3>
        <div className="table">
          {payouts.map((p) => (
            <div key={p.id} className="row">
              <span>{p.id.slice(0, 8)}...</span>
              <span>{p.device_id}</span>
              <span>${Number(p.amount_usd).toFixed(4)}</span>
              <span>{p.status}</span>
              <span>{p.recipient_wallet}</span>
              {p.status === 'pending' ? (
                <>
                  <button onClick={() => approvePayout(p.id)}>approve</button>
                  <button onClick={() => rejectPayout(p.id)}>reject</button>
                </>
              ) : null}
            </div>
          ))}
          {payouts.length === 0 ? <div className="row">No payouts</div> : null}
        </div>
      </div>
    </div>
  );
}


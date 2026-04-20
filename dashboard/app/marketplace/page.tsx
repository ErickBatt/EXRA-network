'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { motion, AnimatePresence } from 'framer-motion';
import {
  LayoutGrid,
  Globe2,
  Activity,
  Wallet,
  CreditCard,
  ArrowLeft,
  ShieldCheck,
  LogIn,
  Search,
  RefreshCw,
  Plus,
  Copy,
  X,
  ArrowRight,
  CheckCircle2,
  AlertCircle,
  Info,
  Zap,
} from 'lucide-react';
import { supabase } from '@/lib/supabase';
import { fetchJson } from '@/lib/api';
import {
  buyerFetch,
  setBuyerApiKey,
  clearBuyerApiKey,
  revealBuyerApiKey,
  BuyerApiUnauthorized,
} from '@/lib/buyerApi';
import Link from 'next/link';
import ProxyGuide from '@/components/ProxyGuide';
import UsageChart from '@/components/UsageChart';
import LiveMap from '@/components/LiveMap';
import { WalletSelector } from '@/components/blockchain/WalletSelector';
import { StakingPanel } from '@/components/blockchain/StakingPanel';
import './marketplace.css';

type Node = {
  id: string;
  country: string;
  device_type: string;
  device_tier: string;
  bandwidth_mbps: number;
  status: string;
  is_residential: boolean;
  price_per_gb: number;
  auto_price: boolean;
};

type Session = {
  id: string;
  node_id: string;
  started_at: string;
  ended_at: string | null;
  bytes_used: number;
  cost_usd: number;
  active: boolean;
};

type BuyerProfile = {
  id: string;
  // api_key is intentionally optional: with the cookie-auth proxy it is
  // not needed for everyday calls and is fetched on demand via
  // revealBuyerApiKey() when the user wants to copy it for an external
  // client (e.g. ProxyGuide). Backend may still include it in /me for
  // backwards compatibility — we just don't rely on it here.
  api_key?: string;
  email: string;
  balance_usd: number;
};

type Offer = {
  id: string;
  country: string;
  target_gb: number;
  max_price_per_gb: number;
  status: string;
  reserved_exra: number;
  settled_exra: number;
};

type ToastMsg = { id: number; kind: 'success' | 'error' | 'info'; text: string };

export default function MarketplacePage() {
  const [activeTab, setActiveTab] = useState<'overview' | 'nodes' | 'sessions' | 'topup' | 'peaq'>('overview');
  const [user, setUser] = useState<any>(null);
  const [buyer, setBuyer] = useState<BuyerProfile | null>(null);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [stats, setStats] = useState<any>({ online_nodes: 0, countries: 0, total_traffic_gb: 0 });
  const [loading, setLoading] = useState(true);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);
  const [topupAmount, setTopupAmount] = useState<number>(0);
  const [topupSuccess, setTopupSuccess] = useState(false);
  const [countryFilter, setCountryFilter] = useState<string>('ALL');
  const [marketPrice, setMarketPrice] = useState<number>(1.5);
  const [offers, setOffers] = useState<Offer[]>([]);
  const [offerCountry, setOfferCountry] = useState<string>('IN');
  const [offerTargetGb, setOfferTargetGb] = useState<number>(10);
  const [offerMaxPrice, setOfferMaxPrice] = useState<number>(1.5);
  // Lazily revealed only when user clicks Reveal/Copy in the Developer
  // Access card or expands ProxyGuide. Keeping it out of state by default
  // means a passive XSS payload cannot scrape it on page load.
  const [revealedApiKey, setRevealedApiKey] = useState<string>('');
  const [toasts, setToasts] = useState<ToastMsg[]>([]);

  const router = useRouter();

  const pushToast = (kind: ToastMsg['kind'], text: string) => {
    const id = Date.now() + Math.random();
    setToasts((t) => [...t, { id, kind, text }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 3200);
  };

  useEffect(() => {
    const init = async () => {
      fetchNodes();
      fetchStats();

      // Try to load buyer profile if logged in. The API key now lives in
      // an httpOnly cookie set by /buyer-api/auth/set, so we try the
      // cookie-authenticated proxy first; if that 401s we fall back to
      // the legacy localStorage key for a one-shot migration, then wipe
      // it so it never sits in JS-readable storage again.
      try {
        const { data: { session } } = await supabase.auth.getSession();
        if (session) {
          setUser(session.user);
          let buyerData: BuyerProfile | null = null;
          try {
            buyerData = await buyerFetch<BuyerProfile>('/api/buyer/me');
          } catch (err) {
            if (err instanceof BuyerApiUnauthorized) {
              const legacy = localStorage.getItem('exra_buyer_api_key') || '';
              if (legacy) {
                try {
                  await setBuyerApiKey(legacy);
                  buyerData = await buyerFetch<BuyerProfile>('/api/buyer/me');
                  // Migration succeeded — drop the JS-readable copy so
                  // any future XSS cannot exfil it.
                  localStorage.removeItem('exra_buyer_api_key');
                } catch (migrateErr) {
                  console.error('buyer cookie migration failed:', migrateErr);
                  await clearBuyerApiKey();
                }
              }
            } else {
              throw err;
            }
          }
          if (buyerData) {
            setBuyer(buyerData);
            fetchOffers();
          }
        }
      } catch (err) {
        console.error('Failed to load buyer:', err);
      } finally {
        setLoading(false);
      }
    };
    init();
  }, []);

  useEffect(() => {
    if (buyer && activeTab === 'sessions') {
      fetchSessions();
    }
  }, [activeTab, buyer]);

  const fetchNodes = async () => {
    try {
      const data = await fetchJson<Node[]>('/api/nodes?sort=price_asc');
      setNodes(data || []);
    } catch (e) { console.error(e); }
  };

  const fetchMarketPrice = async (country: string) => {
    if (!country || country === 'ALL') {
      setMarketPrice(1.5);
      return;
    }
    try {
      const data = await fetchJson<{ avg_price: number }>(`/api/nodes/market-price?country=${country}`);
      setMarketPrice(data?.avg_price ?? 1.5);
    } catch (e) {
      console.error(e);
      setMarketPrice(1.5);
    }
  };

  const fetchStats = async () => {
    try {
      const data = await fetchJson<any>('/nodes/stats');
      setStats({
        online_nodes: data.online_nodes || 0,
        countries: data.countries || 0,
        total_traffic_gb: ((data.total_traffic_bytes || 0) / 1e9).toFixed(1)
      });
    } catch (e) { console.error(e); }
  };

  const fetchSessions = async () => {
    if (!buyer) return;
    try {
      const data = await buyerFetch<Session[]>('/api/buyer/sessions');
      setSessions(data || []);
    } catch (e) { console.error(e); }
  };

  const fetchOffers = async () => {
    try {
      const data = await buyerFetch<Offer[]>('/api/offers?limit=20');
      setOffers(data || []);
    } catch (e) { console.error(e); }
  };

  // Mirrors the backend's validateCountry/validateFloat in handlers so we
  // surface the same constraints inline instead of round-tripping a 400.
  // Returns null when the form is valid.
  const offerValidationError = (): string | null => {
    const country = (offerCountry || '').trim();
    if (!country) return 'Country is required';
    if (country.length > 8) return 'Country code is too long';
    if (!/^[A-Za-z]+$/.test(country)) return 'Country must contain only letters';
    if (!Number.isFinite(offerTargetGb) || offerTargetGb <= 0) return 'Target GB must be greater than 0';
    if (offerTargetGb > 100_000) return 'Target GB exceeds maximum (100000)';
    if (!Number.isFinite(offerMaxPrice) || offerMaxPrice <= 0) return 'Max price/GB must be greater than 0';
    if (offerMaxPrice > 1_000) return 'Max price/GB exceeds maximum (1000)';
    return null;
  };

  const createOffer = async () => {
    if (!buyer) return;
    const v = offerValidationError();
    if (v) {
      alert(v);
      return;
    }
    try {
      await buyerFetch('/api/offers', {
        method: 'POST',
        body: JSON.stringify({
          country: offerCountry.trim(),
          target_gb: offerTargetGb,
          max_price_per_gb: offerMaxPrice
        })
      });
      fetchOffers();
      pushToast('success', 'Offer created');
    } catch (e) {
      pushToast('error', `Failed to create offer: ${e}`);
    }
  };

  const assignOffer = async (offerId: string) => {
    if (!buyer) return;
    try {
      await buyerFetch(`/api/offers/${offerId}/assign`, { method: 'POST' });
      fetchOffers();
      fetchSessions();
      setActiveTab('sessions');
      pushToast('success', 'Offer assigned — session starting');
    } catch (e) {
      pushToast('error', `Failed to assign offer: ${e}`);
    }
  };

  const handleTopUp = async () => {
    if (!buyer || topupAmount <= 0) return;
    setLoading(true);
    try {
      await buyerFetch('/api/buyer/topup', {
        method: 'POST',
        body: JSON.stringify({ amount_usd: topupAmount })
      });
      const newBuyer = await buyerFetch<BuyerProfile>('/api/buyer/me');
      setBuyer(newBuyer);
      setTopupSuccess(true);
      pushToast('success', `Balance topped up by $${topupAmount.toFixed(2)}`);
      setTimeout(() => setTopupSuccess(false), 3000);
    } catch (e) {
      console.error(e);
      pushToast('error', 'Top up failed. Is the server running?');
    } finally {
      setLoading(false);
    }
  };

  const startSession = async (nodeId: string) => {
    if (!buyer) return;
    try {
      await buyerFetch('/api/session/start', {
        method: 'POST',
        body: JSON.stringify({ node_id: nodeId })
      });
      setSelectedNode(null);
      setActiveTab('sessions');
      pushToast('success', 'Session started');
    } catch (e) {
      pushToast('error', `Failed to start session: ${e}`);
    }
  };

  const endSession = async (sessionId: string) => {
    if (!buyer) return;
    try {
      await buyerFetch(`/api/session/${sessionId}/end`, { method: 'POST' });
      fetchSessions();
      pushToast('info', 'Session stopped');
    } catch (e) {
      pushToast('error', `Failed to end session: ${e}`);
    }
  };

  // Pulls the API key out of the httpOnly cookie via the reveal endpoint.
  // Idempotent — caches in component state once revealed.
  const ensureApiKeyRevealed = async (): Promise<string> => {
    if (revealedApiKey) return revealedApiKey;
    const k = await revealBuyerApiKey();
    setRevealedApiKey(k);
    return k;
  };

  const copyApiKeyToClipboard = async () => {
    try {
      const k = await ensureApiKeyRevealed();
      await navigator.clipboard.writeText(k);
      pushToast('success', 'API key copied to clipboard');
    } catch (e) {
      pushToast('error', 'Failed to reveal API key: ' + e);
    }
  };

  const getFlag = (country: string) => {
    const flags: any = { IN: '🇮🇳', BR: '🇧🇷', NG: '🇳🇬', ID: '🇮🇩', MX: '🇲🇽', PH: '🇵🇭' };
    return flags[country] || '🌍';
  };

  const visibleNodes = countryFilter === 'ALL' ? nodes : nodes.filter((n) => n.country === countryFilter);
  const countries = Array.from(new Set(nodes.map((n) => n.country).filter(Boolean))).sort();

  if (loading && !buyer) {
    return (
      <div className="marketplace-root" style={{ alignItems: 'center', justifyContent: 'center' }}>
        <div className="spinner" style={{ width: '36px', height: '36px', borderWidth: '3px' }}></div>
      </div>
    );
  }

  return (
    <div className="marketplace-root">
      {/* SIDEBAR */}
      <aside className="sidebar">
        <div className="sidebar-logo">ex<span>ra</span></div>

        <div className="sidebar-section">marketplace</div>
        <div className={`nav-item ${activeTab === 'overview' ? 'active' : ''}`} onClick={() => setActiveTab('overview')}>
          <LayoutGrid size={15} strokeWidth={1.8} />
          Overview
        </div>
        <div className={`nav-item ${activeTab === 'nodes' ? 'active' : ''}`} onClick={() => setActiveTab('nodes')}>
          <Globe2 size={15} strokeWidth={1.8} />
          Browse Nodes
          <span className="nav-badge">{nodes.length}</span>
        </div>
        <div className={`nav-item ${activeTab === 'sessions' ? 'active' : ''}`} onClick={() => setActiveTab('sessions')}>
          <Activity size={15} strokeWidth={1.8} />
          Sessions
          <span className="nav-badge blue">{sessions.filter(s => s.active).length}</span>
        </div>

        <div className={`nav-item ${activeTab === 'peaq' ? 'active' : ''}`} onClick={() => setActiveTab('peaq')}>
          <Wallet size={15} strokeWidth={1.8} />
          Peaq Network
        </div>

        <div className="sidebar-section">account</div>
        <div className={`nav-item ${activeTab === 'topup' ? 'active' : ''}`} onClick={() => setActiveTab('topup')}>
          <CreditCard size={15} strokeWidth={1.8} />
          Top Up
        </div>
        <Link className="nav-item" href="/">
          <ArrowLeft size={15} strokeWidth={1.8} />
          Back to site
        </Link>
        <Link className="nav-item" href="/admin">
          <ShieldCheck size={15} strokeWidth={1.8} />
          Admin Console
        </Link>

        <div className="sidebar-bottom">
          {buyer ? (
            <div className="buyer-info">
              <div className="buyer-avatar">{buyer.email.substring(0, 2).toUpperCase()}</div>
              <div style={{ minWidth: 0, flex: 1 }}>
                <div className="buyer-name" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {buyer.email.split('@')[0]}
                </div>
                <div className="buyer-balance">${buyer.balance_usd.toFixed(2)}</div>
              </div>
            </div>
          ) : (
            <Link href="/auth" className="nav-item" style={{ justifyContent: 'center', color: 'var(--neon)', padding: '10px' }}>
              <LogIn size={15} strokeWidth={1.8} />
              Sign in
            </Link>
          )}
        </div>
      </aside>

      {/* MAIN */}
      <main className="main">
        {/* TOPBAR */}
        <div className="topbar">
          <div className="topbar-title">{activeTab.charAt(0).toUpperCase() + activeTab.slice(1)}</div>
          <div className="topbar-right">
            {buyer ? (
              <>
                <div className="topbar-balance">
                  <Zap size={13} strokeWidth={2} color="#22d3ee" />
                  Balance
                  <span className="topbar-balance-val">${buyer.balance_usd.toFixed(2)}</span>
                </div>
                <button className="btn-topup-main" onClick={() => setActiveTab('topup')}>
                  <Plus size={14} strokeWidth={2.4} /> Top Up
                </button>
              </>
            ) : (
              <Link href="/auth" className="btn-topup-main">
                Sign in <ArrowRight size={14} strokeWidth={2.4} />
              </Link>
            )}
          </div>
        </div>

        <div className="content">
          {/* STATS */}
          <div className="stats-row-dash">
            <div className="stat-card-dash">
              <div className="stat-label-dash">online nodes</div>
              <div className="stat-val-dash"><span>{stats.online_nodes}</span></div>
              <div className="stat-badge-dash neutral">{stats.countries} countries</div>
            </div>
            <div className="stat-card-dash">
              <div className="stat-label-dash">total traffic</div>
              <div className="stat-val-dash"><span>{stats.total_traffic_gb}</span></div>
              <div className="stat-sub-dash">GB routed</div>
            </div>
            <div className="stat-card-dash">
              <div className="stat-label-dash">price per GB</div>
              <div className="stat-val-dash"><span>$</span>{marketPrice.toFixed(2)}</div>
              <div className="stat-badge-dash up">{countryFilter === 'ALL' ? 'global avg' : `${countryFilter} avg`}</div>
            </div>
            <div className="stat-card-dash">
              <div className="stat-label-dash">your balance</div>
              <div className="stat-val-dash"><span>$</span>{buyer ? buyer.balance_usd.toFixed(2) : '—'}</div>
              <div className="stat-sub-dash">{buyer ? 'USDT available' : 'login to see'}</div>
            </div>
          </div>

          {/* TAB CONTENT */}
          {activeTab === 'overview' && (
            <div className="dashboard-overview">
              <div>
                <div style={{ fontSize: '10.5px', color: 'var(--ink-dim)', fontFamily: "'Geist Mono', monospace", textTransform: 'uppercase', marginBottom: '10px', letterSpacing: '0.12em', fontWeight: 500 }}>
                  live node map
                </div>
                <LiveMap nodes={nodes} height={420} />
              </div>

              <div className="overview-hero">
                <div className="hero-content">
                  <h1>Decentralized bandwidth on demand</h1>
                  <p>Access high-quality residential and mobile proxies across {stats.countries} countries. Pay per GB, settle on-chain.</p>
                  <div className="hero-actions">
                    <button className="btn-hero-primary" onClick={() => setActiveTab('nodes')}>
                      Browse marketplace <ArrowRight size={14} strokeWidth={2.4} />
                    </button>
                    <button className="btn-hero-secondary" onClick={() => setActiveTab('topup')}>Get credits</button>
                  </div>
                </div>
                <div className="hero-visual">
                  <div className="world-map-mock">
                    <div className="map-dot" style={{ top: '30%', left: '40%' }}></div>
                    <div className="map-dot" style={{ top: '60%', left: '20%' }}></div>
                    <div className="map-dot" style={{ top: '50%', left: '70%' }}></div>
                    <div className="map-line" style={{ top: '30%', left: '40%', width: '100px', transform: 'rotate(20deg)' }}></div>
                  </div>
                </div>
              </div>

              <div className="overview-secondary">
                <div className="table-wrap">
                  <div className="table-header"><span className="table-header-title">Developer access</span></div>
                  <div className="card-body-dash">
                    <div className="api-key-row">
                      <div className="api-label">Your API token</div>
                      <div className="api-val-wrap">
                        <code className="api-key-code">
                          {revealedApiKey || '••••••••••••••••  (hidden)'}
                        </code>
                        {!revealedApiKey && (
                          <button
                            className="btn-copy-mini"
                            onClick={() => ensureApiKeyRevealed().catch((e) => alert('Reveal failed: ' + e))}
                          >Reveal</button>
                        )}
                        <button className="btn-copy-mini" onClick={copyApiKeyToClipboard}>Copy</button>
                      </div>
                    </div>
                    <div className="api-hint">Use this token as a password for proxy authentication or server-to-server API calls.</div>
                  </div>
                </div>
                <div className="table-wrap">
                  <div className="table-header"><span className="table-header-title">Create offer</span></div>
                  <div className="card-body-dash">
                    <div className="api-label">Country</div>
                    <div className="search-box-dash" style={{ minWidth: 0, marginBottom: 12 }}>
                      <input type="text" value={offerCountry} onChange={(e) => setOfferCountry(e.target.value.toUpperCase())} />
                    </div>
                    <div className="api-label">Target GB</div>
                    <div className="search-box-dash" style={{ minWidth: 0, marginBottom: 12 }}>
                      <input type="number" value={offerTargetGb} onChange={(e) => setOfferTargetGb(parseFloat(e.target.value) || 0)} />
                    </div>
                    <div className="api-label">Max price (EXRA/GB)</div>
                    <div className="search-box-dash" style={{ minWidth: 0, marginBottom: 14 }}>
                      <input type="number" value={offerMaxPrice} onChange={(e) => setOfferMaxPrice(parseFloat(e.target.value) || 0)} />
                    </div>
                    {(() => {
                      const v = offerValidationError();
                      return (
                        <>
                          {v && (
                            <div style={{ color: 'var(--red, #c44)', fontSize: '11px', marginBottom: '8px' }}>{v}</div>
                          )}
                          <button
                            className="btn-hero-primary"
                            onClick={createOffer}
                            disabled={!!v}
                            style={v ? { opacity: 0.5, cursor: 'not-allowed' } : undefined}
                          >Create offer</button>
                        </>
                      );
                    })()}
                  </div>
                </div>

                <div className="table-wrap">
                  <div className="table-header">
                    <span className="table-header-title">Live sessions</span>
                    <Link href="#" onClick={(e) => { e.preventDefault(); setActiveTab('sessions'); }} className="header-link">View all →</Link>
                  </div>
                  <div className="card-body-dash">
                    {sessions.filter(s => s.active).length === 0 ? (
                      <div className="empty-mini">No active sessions. Start a node to see metrics.</div>
                    ) : (
                      sessions.filter(s => s.active).slice(0, 1).map(s => (
                        <div key={s.id} className="mini-session-stat">
                          <div className="ms-meta">
                            <span className="ms-title">Session {s.id.substring(0, 6)}</span>
                            <span className="ms-usage">{(s.bytes_used / 1e6).toFixed(2)} MB</span>
                          </div>
                          <UsageChart isActive={true} />
                        </div>
                      ))
                    )}
                  </div>
                </div>
                <div className="table-wrap">
                  <div className="table-header"><span className="table-header-title">Operations</span></div>
                  <div className="card-body-dash">
                    <div className="api-hint">Operational actions moved to the dedicated admin console with role checks and audit logging.</div>
                    <Link href="/admin" className="btn-hero-primary" style={{ display: 'inline-flex', marginTop: 14 }}>
                      Open admin console <ArrowRight size={14} strokeWidth={2.4} />
                    </Link>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'nodes' && (
            <div className="tab-content">
              <div className="filters-bar">
                <div className="filters-left">
                  <div className="search-box-dash">
                    <Search size={13} strokeWidth={2} color="#71717a" />
                    <select
                      value={countryFilter}
                      onChange={(e) => {
                        const next = e.target.value;
                        setCountryFilter(next);
                        fetchMarketPrice(next);
                      }}
                    >
                      <option value="ALL">All countries</option>
                      {countries.map((country) => (
                        <option key={country} value={country}>{country}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <button className="filter-btn-dash" onClick={fetchNodes}>
                  <RefreshCw size={12} strokeWidth={2} />
                  refresh
                </button>
              </div>

              <div className="table-wrap">
                <div className="table-header">
                  <span className="table-header-title">Available nodes</span>
                  <div className="live-indicator"><span className="live-dot"></span>live data</div>
                </div>
                <table>
                  <thead>
                    <tr>
                      <th>node id</th>
                      <th>country</th>
                      <th>type</th>
                      <th>speed</th>
                      <th>tier</th>
                      <th>status</th>
                      <th>price</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleNodes.map(node => (
                      <tr key={node.id}>
                        <td>
                          <div className="node-id">
                            <div className="node-avatar">{node.id.substring(0, 2).toUpperCase()}</div>
                            <span className="node-did">{node.id.substring(0, 8)}…</span>
                          </div>
                        </td>
                        <td>
                          <div className="country-cell">
                            <div className="country-flag">{getFlag(node.country)}</div>
                            <span className="country-name">{node.country}</span>
                          </div>
                        </td>
                        <td><span className="device-pill">{node.device_type}</span></td>
                        <td>
                          <div className="speed-bar-wrap">
                            <div className="speed-bar">
                              <div className="speed-fill" style={{ width: `${Math.min(100, (node.bandwidth_mbps / 100) * 100)}%` }}></div>
                            </div>
                            <span className="speed-val">{node.bandwidth_mbps} Mbps</span>
                          </div>
                        </td>
                        <td><span className={`tier-badge ${node.device_tier === 'compute' ? 'tier-2' : 'tier-1'}`}>{node.device_tier}</span></td>
                        <td><span className="status-online"><span className="status-dot"></span>{node.status}</span></td>
                        <td><span className="price-cell">${(node.price_per_gb ?? 1.5).toFixed(2)}/GB{node.auto_price ? ' (auto)' : ''}</span></td>
                        <td>
                          <button className="btn-buy-node" onClick={() => { if (!buyer) { router.push('/auth'); return; } setSelectedNode(node); }}>
                            buy
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="table-wrap">
                <div className="table-header">
                  <span className="table-header-title">Offers</span>
                  <button className="filter-btn-dash" onClick={() => fetchOffers()}>
                    <RefreshCw size={12} strokeWidth={2} /> refresh
                  </button>
                </div>
                <table>
                  <thead>
                    <tr>
                      <th>id</th>
                      <th>country</th>
                      <th>target</th>
                      <th>max price</th>
                      <th>status</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {offers.map((o) => (
                      <tr key={o.id}>
                        <td><span className="node-did">{o.id.substring(0, 8)}…</span></td>
                        <td><span className="country-name">{o.country || 'ANY'}</span></td>
                        <td><span className="price-cell">{o.target_gb} GB</span></td>
                        <td><span className="price-cell">{o.max_price_per_gb.toFixed(2)} EXRA</span></td>
                        <td><span className={`tier-badge ${o.status === 'pending' ? 'tier-1' : 'tier-2'}`}>{o.status}</span></td>
                        <td>
                          {o.status === 'pending' ? (
                            <button className="btn-buy-node" onClick={() => assignOffer(o.id)}>assign</button>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {activeTab === 'sessions' && (
            <div className="session-grid-dash">
              {sessions.length === 0 && <div className="empty-state">No sessions found.</div>}
              {sessions.map(session => (
                <div className={`session-card-refined ${session.active ? 'active' : ''}`} key={session.id}>
                  <div className="scr-header">
                    <div className="scr-node">
                      <div className="scr-node-avatar">{session.node_id.substring(0, 2).toUpperCase()}</div>
                      <div className="scr-node-info">
                        <div className="scr-node-id">Node: {session.node_id.substring(0, 8)}</div>
                        <div className="scr-node-status">{session.active ? 'streaming' : 'completed'}</div>
                      </div>
                    </div>
                    {session.active && <button className="btn-stop-mini" onClick={() => endSession(session.id)}>Stop</button>}
                  </div>

                  <div className="scr-body">
                    <div className="scr-stat">
                      <span className="scr-label">Usage</span>
                      <span className="scr-val">{(session.bytes_used / 1e6).toFixed(2)} MB</span>
                    </div>
                    <div className="scr-stat">
                      <span className="scr-label">Cost</span>
                      <span className="scr-val accent">${session.cost_usd.toFixed(4)}</span>
                    </div>
                    {session.active && (
                      <div className="scr-chart">
                        <UsageChart isActive={true} />
                      </div>
                    )}
                  </div>

                  {session.active && buyer && (
                    <div className="scr-footer">
                       <div className="scr-guide-label">Connection Guide</div>
                       {revealedApiKey ? (
                         <ProxyGuide apiKey={revealedApiKey} />
                       ) : (
                         <button
                           className="btn-copy-mini"
                           onClick={() => ensureApiKeyRevealed().catch((e) => alert('Reveal failed: ' + e))}
                         >Reveal API key to see connection details</button>
                       )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {activeTab === 'topup' && (
            <div style={{ maxWidth: '480px' }}>
              <div className="table-wrap">
                <div className="table-header"><span className="table-header-title">Top up balance</span></div>
                <div style={{ padding: '22px' }}>
                  <div className="modal-row">
                    <span className="modal-row-label">current balance</span>
                    <span className="modal-row-val accent">${buyer?.balance_usd.toFixed(2) ?? '—'}</span>
                  </div>
                  <div className="api-label" style={{ marginTop: 18, marginBottom: 8 }}>amount (USD)</div>
                  <div className="search-box-dash" style={{ minWidth: 0, marginBottom: 16 }}>
                    <span style={{ color: 'var(--neon-bright)', fontFamily: 'Geist Mono, monospace' }}>$</span>
                    <input type="number" placeholder="10.00" value={topupAmount || ''} onChange={(e) => setTopupAmount(parseFloat(e.target.value) || 0)} />
                  </div>
                  <button className="btn-modal-confirm" style={{ width: '100%', padding: '12px' }} onClick={handleTopUp} disabled={loading || topupAmount <= 0 || !buyer}>
                    {loading ? <div className="spinner"></div> : <>Add funds <ArrowRight size={14} strokeWidth={2.4} /></>}
                  </button>
                  {topupSuccess && (
                    <div style={{ color: 'var(--success)', fontSize: '12px', marginTop: 12, textAlign: 'center', fontFamily: 'Geist Mono, monospace' }}>
                      Balance updated successfully
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {activeTab === 'peaq' && (
            <div className="tab-content" style={{ maxWidth: '920px' }}>
              <div className="stats-row-dash" style={{ gridTemplateColumns: 'repeat(2, 1fr)', marginBottom: '18px' }}>
                <div className="stat-card-dash">
                  <div className="stat-label-dash">network status</div>
                  <div className="stat-val-dash" style={{ color: 'var(--success)' }}>online</div>
                  <div className="stat-sub-dash">Peaq L1 Mainnet</div>
                </div>
                <div className="stat-card-dash">
                  <div className="stat-label-dash">exra staking</div>
                  <div className="stat-val-dash"><span>100</span></div>
                  <div className="stat-sub-dash">EXRA required</div>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.5fr', gap: '18px', alignItems: 'start' }}>
                <div className="table-wrap">
                  <div className="table-header"><span className="table-header-title">Identity &amp; wallet</span></div>
                  <div className="card-body-dash">
                    <p style={{ fontSize: '12px', color: 'var(--ink-dim)', lineHeight: 1.55, marginBottom: 14 }}>
                      Connect your Substrate wallet to manage DID and staking.
                    </p>
                    <WalletSelector />
                  </div>
                </div>

                <StakingPanel />
              </div>
            </div>
          )}
        </div>
      </main>

      {/* BUY MODAL */}
      <AnimatePresence>
        {selectedNode && (
          <motion.div
            className="modal-overlay"
            onClick={() => setSelectedNode(null)}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.18 }}
          >
            <motion.div
              className="modal-dash"
              onClick={(e) => e.stopPropagation()}
              initial={{ y: 16, opacity: 0, scale: 0.97 }}
              animate={{ y: 0, opacity: 1, scale: 1 }}
              exit={{ y: 10, opacity: 0, scale: 0.98 }}
              transition={{ type: 'spring', stiffness: 320, damping: 28 }}
            >
              <div className="modal-header">
                <span className="modal-title">Start session</span>
                <button className="modal-close" onClick={() => setSelectedNode(null)}>
                  <X size={14} strokeWidth={2} />
                </button>
              </div>
              <div className="modal-body">
                <div className="modal-node-info">
                  <div className="modal-node-avatar">{getFlag(selectedNode.country)}</div>
                  <div>
                    <div className="modal-node-country">{selectedNode.country}</div>
                    <div className="modal-node-meta">{selectedNode.device_type} · {selectedNode.bandwidth_mbps} Mbps</div>
                  </div>
                  <div className="modal-node-price">
                    <div className="modal-price-val">${(selectedNode.price_per_gb ?? 1.5).toFixed(2)}</div>
                    <div className="modal-price-unit">per GB</div>
                  </div>
                </div>
                <div className="modal-row">
                  <span className="modal-row-label">node ID</span>
                  <span className="modal-row-val">{selectedNode.id}</span>
                </div>
                <div className="modal-row">
                  <span className="modal-row-label">your balance</span>
                  <span className="modal-row-val accent">${buyer?.balance_usd.toFixed(2)}</span>
                </div>
              </div>
              <div className="modal-footer">
                <button className="btn-modal-cancel" onClick={() => setSelectedNode(null)}>Cancel</button>
                <button className="btn-modal-confirm" onClick={() => startSession(selectedNode.id)}>
                  Start session <ArrowRight size={14} strokeWidth={2.4} />
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* TOASTS */}
      <div className="mp-toast-wrap" aria-live="polite">
        <AnimatePresence>
          {toasts.map((t) => (
            <motion.div
              key={t.id}
              className={`mp-toast ${t.kind}`}
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
              transition={{ type: 'spring', stiffness: 300, damping: 26 }}
            >
              {t.kind === 'success' && <CheckCircle2 size={16} strokeWidth={2} color="#10b981" />}
              {t.kind === 'error' && <AlertCircle size={16} strokeWidth={2} color="#ef4444" />}
              {t.kind === 'info' && <Info size={16} strokeWidth={2} color="#22d3ee" />}
              <span>{t.text}</span>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </div>
  );
}

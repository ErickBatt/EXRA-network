'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';

// /api/audit/mints is routed by nginx from exra.space/api/* → Go backend.
// Use relative URL so it works from any browser without env vars.
const AUDIT_URL = '/api/audit/mints';

type MintEntry = {
  id: number;
  device_id: string;
  amount_exra: number;
  status: string;
  tx_hash: string;
  tonscan_url: string;
  minted_at: string;
};

type AuditResponse = {
  mints: MintEntry[];
  total: number;
  contract: string;
  tonscan_contract: string;
};

export default function AuditPage() {
  const [data, setData] = useState<AuditResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(`${AUDIT_URL}?limit=200`, { cache: 'no-store' })
      .then(r => r.json())
      .then(setData)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  const totalExra = data?.mints.reduce((s, m) => s + m.amount_exra, 0) ?? 0;

  return (
    <div style={{ background: '#111110', minHeight: '100vh', color: '#e8e4d8', fontFamily: "'JetBrains Mono', monospace", padding: '32px 24px', maxWidth: '900px', margin: '0 auto' }}>

      <nav style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '40px' }}>
        <Link href="/" style={{ color: '#c8f03c', textDecoration: 'none', fontSize: '18px', fontWeight: 700 }}>
          ex<span style={{ color: '#e8e4d8' }}>ra</span>
        </Link>
        <span style={{ fontSize: '11px', color: '#5a5850', textTransform: 'uppercase', letterSpacing: '0.1em' }}>mint audit log</span>
      </nav>

      <div style={{ marginBottom: '32px' }}>
        <h1 style={{ fontSize: '24px', fontWeight: 700, marginBottom: '8px' }}>
          Token Mint Transparency
        </h1>
        <p style={{ color: '#9a9485', fontSize: '13px', lineHeight: 1.6, maxWidth: '600px' }}>
          Every EXRA token is minted only when earned through real work — bandwidth shared or compute contributed.
          All mint transactions are recorded here and verifiable on-chain.
        </p>
      </div>

      {/* Contract info */}
      <div style={{ background: '#1a1a16', border: '1px solid #2a2a24', borderRadius: '12px', padding: '16px 20px', marginBottom: '24px' }}>
        <div style={{ fontSize: '11px', color: '#5a5850', textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: '8px' }}>EXRA Jetton Contract (TON Mainnet)</div>
        <a
          href={data?.tonscan_contract ?? 'https://tonscan.org/address/EQB_f4bDOrHkr4XpJzlbUaQkyAgm7PM7jap8hknD9J5h7fmd'}
          target="_blank"
          rel="noopener noreferrer"
          style={{ color: '#c8f03c', fontSize: '13px', wordBreak: 'break-all' }}
        >
          {data?.contract ?? 'EQB_f4bDOrHkr4XpJzlbUaQkyAgm7PM7jap8hknD9J5h7fmd'}
        </a>
      </div>

      {/* Stats */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '12px', marginBottom: '28px' }}>
        {[
          { label: 'Total Minted', value: totalExra.toFixed(2) + ' EXRA' },
          { label: 'Mint Events', value: String(data?.total ?? 0) },
          { label: 'Max Supply', value: '1,000,000,000 EXRA' },
        ].map(s => (
          <div key={s.label} style={{ background: '#1a1a16', border: '1px solid #2a2a24', borderRadius: '10px', padding: '14px 16px' }}>
            <div style={{ fontSize: '10px', color: '#5a5850', textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: '6px' }}>{s.label}</div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: '#c8f03c' }}>{s.value}</div>
          </div>
        ))}
      </div>

      {/* Table */}
      <div style={{ background: '#1a1a16', border: '1px solid #2a2a24', borderRadius: '12px', overflow: 'hidden' }}>
        <div style={{ padding: '14px 20px', borderBottom: '1px solid #2a2a24', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ fontSize: '12px', fontWeight: 600 }}>Mint History</span>
          <span style={{ fontSize: '11px', color: '#5a5850' }}>confirmed on-chain only</span>
        </div>

        {loading && (
          <div style={{ padding: '40px', textAlign: 'center', color: '#5a5850' }}>Loading...</div>
        )}

        {!loading && (!data?.mints || data.mints.length === 0) && (
          <div style={{ padding: '40px', textAlign: 'center', color: '#5a5850', fontSize: '13px' }}>
            No confirmed mints yet. First tokens will appear here once the first payout is processed.
          </div>
        )}

        {!loading && data?.mints && data.mints.length > 0 && (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
            <thead>
              <tr style={{ color: '#5a5850', textTransform: 'uppercase', letterSpacing: '0.06em', fontSize: '10px' }}>
                {['#', 'Device', 'Amount', 'Status', 'TX', 'Date'].map(h => (
                  <th key={h} style={{ padding: '10px 20px', textAlign: 'left', fontWeight: 500 }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.mints.map((m, i) => (
                <tr key={m.id} style={{ borderTop: '1px solid #1e1e1a' }}>
                  <td style={{ padding: '12px 20px', color: '#5a5850' }}>{m.id}</td>
                  <td style={{ padding: '12px 20px', color: '#9a9485' }}>{m.device_id}</td>
                  <td style={{ padding: '12px 20px', color: '#c8f03c', fontWeight: 600 }}>{m.amount_exra.toFixed(6)}</td>
                  <td style={{ padding: '12px 20px' }}>
                    <span style={{
                      background: m.status === 'confirmed' ? 'rgba(200,240,60,0.1)' : 'rgba(100,100,90,0.2)',
                      color: m.status === 'confirmed' ? '#c8f03c' : '#9a9485',
                      padding: '2px 8px', borderRadius: '4px', fontSize: '10px'
                    }}>{m.status}</span>
                  </td>
                  <td style={{ padding: '12px 20px' }}>
                    {m.tonscan_url ? (
                      <a href={m.tonscan_url} target="_blank" rel="noopener noreferrer"
                        style={{ color: '#7ab8ff', fontSize: '11px' }}>
                        {m.tx_hash.slice(0, 12)}...
                      </a>
                    ) : (
                      <span style={{ color: '#3a3a30' }}>—</span>
                    )}
                  </td>
                  <td style={{ padding: '12px 20px', color: '#5a5850', fontSize: '11px' }}>
                    {new Date(m.minted_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div style={{ marginTop: '24px', fontSize: '11px', color: '#3a3a30', textAlign: 'center' }}>
        Device IDs are truncated for user privacy. All mint TX hashes are publicly verifiable on TON blockchain.
      </div>
    </div>
  );
}

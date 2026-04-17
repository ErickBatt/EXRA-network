import Link from 'next/link';
import './landing.css';

export default function LandingPage() {
  return (
    <div className="landing-root">
      <nav className="nav-landing">
        <div className="logo">ex<span>ra</span></div>
        <ul className="nav-links">
          <li><a href="#how">how it works</a></li>
          <li><a href="#devices">devices</a></li>
          <li><a href="#marketplace">marketplace</a></li>
          <li><a href="#token">token</a></li>
        </ul>
        <div className="nav-actions">
          <Link href="/marketplace" className="btn-ghost">buy traffic</Link>
          <Link href="/auth" className="btn-accent">
            <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
              <path d="M6.5 1v7M3.5 5.5l3 3 3-3M1.5 10.5h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
            </svg>
            get the app
          </Link>
        </div>
      </nav>

      <section className="hero">
        <div className="hero-lava">
          <div className="lava-blob" style={{ width: '580px', height: '580px', background: 'rgba(200,240,60,0.14)', bottom: '-150px', left: '-80px', animation: 'lr1 11s ease-in-out infinite alternate' }}></div>
          <div className="lava-blob" style={{ width: '440px', height: '440px', background: 'rgba(160,200,20,0.09)', bottom: '-100px', left: '200px', animation: 'lr2 8.5s ease-in-out infinite alternate' }}></div>
          <div className="lava-blob" style={{ width: '360px', height: '360px', background: 'rgba(200,240,60,0.11)', bottom: '-120px', right: '60px', animation: 'lr3 13s ease-in-out infinite alternate' }}></div>
          <div className="lava-blob" style={{ width: '300px', height: '300px', background: 'rgba(120,180,10,0.08)', bottom: '-80px', right: '-50px', animation: 'lr4 9.5s ease-in-out infinite alternate' }}></div>
        </div>

        <div className="hero-tag"><span className="hero-tag-dot"></span>DePIN · TON · Fair Launch</div>

        <h1 className="hero-title">
          your device.<br />
          your <span className="ac">network</span>.<br />
          <span className="dm">your income.</span>
        </h1>

        <p className="hero-sub">Exra turns any device into a node. Share bandwidth, earn EXRA tokens. Withdraw from $1. No limits.</p>

        <div className="hero-actions">
          <Link href="/auth" className="btn-h">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M7 1v8M4 6l3 3 3-3M2 12h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
            </svg>
            download for android
          </Link>
          <Link href="/marketplace" className="btn-hg">buy traffic →</Link>
        </div>

        <div className="hero-metrics">
          <div className="metric"><div className="metric-num"><span className="a">$</span>0.30</div><div className="metric-label">per GB earned</div></div>
          <div className="metric"><div className="metric-num"><span className="a">$</span>1</div><div className="metric-label">min withdrawal</div></div>
          <div className="metric"><div className="metric-num">any</div><div className="metric-label">device works</div></div>
          <div className="metric"><div className="metric-num">0<span className="a">%</span></div><div className="metric-label">team allocation</div></div>
        </div>
      </section>

      <section className="phone-section" id="download">
        <div className="phone-glow"></div>
        <div className="phone-frame">
          <div className="phone-top">
            <span className="phone-logo">ex<span>ra</span></span>
            <span className="online-pill"><span className="online-dot"></span>online</span>
          </div>
          <div className="phone-lava">
            <div className="p-blob" style={{ width: '110px', height: '110px', background: 'rgba(200,240,60,0.5)', bottom: '-20px', left: '20px', animation: 'pb1 8s ease-in-out infinite alternate' }}></div>
            <div className="p-blob" style={{ width: '85px', height: '85px', background: 'rgba(160,210,15,0.42)', bottom: '-10px', left: '125px', animation: 'pb2 6.5s ease-in-out infinite alternate 1s' }}></div>
            <div className="p-blob" style={{ width: '95px', height: '95px', background: 'rgba(185,235,10,0.46)', bottom: '-15px', left: '70px', animation: 'pb3 10s ease-in-out infinite alternate 0.5s' }}></div>
            <div className="p-blob" style={{ width: '60px', height: '60px', background: 'rgba(200,240,60,0.38)', bottom: '-5px', left: '5px', animation: 'pb4 7s ease-in-out infinite alternate 2s' }}></div>
            <div className="phone-data-top">
              <div className="phone-data-label">today</div>
              <div className="phone-data-val">+$1.24</div>
            </div>
            <div className="phone-data-bot">
              <div className="phone-hashrate">2.4</div>
              <div className="phone-hr-label">MB/s</div>
            </div>
          </div>
          <div className="phone-bottom">
            <div className="phone-cards">
              <div className="p-card"><div className="p-card-label">traffic</div><div className="p-card-val">4.2 GB</div></div>
              <div className="p-card"><div className="p-card-label">total earned</div><div className="p-card-val">$19.94</div></div>
            </div>
            <div className="phone-stop">stop</div>
          </div>
          <div className="phone-nav">
            <div className="p-nav-item"><div className="pna"></div><span style={{ fontSize: '10px', color: 'var(--ink)', fontFamily: "'JetBrains Mono', monospace" }}>home</span></div>
            <div className="p-nav-item"><div className="pni"></div><span style={{ fontSize: '10px', color: 'var(--ink3)', fontFamily: "'JetBrains Mono', monospace" }}>wallet</span></div>
            <div className="p-nav-item"><div className="pni"></div><span style={{ fontSize: '10px', color: 'var(--ink3)', fontFamily: "'JetBrains Mono', monospace" }}>profile</span></div>
          </div>
        </div>
      </section>

      <section className="section" id="how">
        <div className="ey">01 / how it works</div>
        <h2 className="st">three steps to<br /><em>passive income</em></h2>
        <div className="steps">
          <div className="step">
            <div className="step-num">01 —</div>
            <div className="step-icon">
              <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
                <rect x="3" y="3" width="16" height="16" rx="5" stroke="#c8f03c" strokeWidth="1.5"/>
                <path d="M11 7v4l3 2" stroke="#c8f03c" strokeWidth="1.5" strokeLinecap="round"/>
              </svg>
            </div>
            <div className="step-title">install & scan</div>
            <div className="step-desc">Download Exra. We scan your device and assign the best earning mode — proxy or compute — based on your hardware automatically.</div>
          </div>
          <div className="step">
            <div className="step-num">02 —</div>
            <div className="step-icon">
              <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
                <circle cx="11" cy="11" r="8" stroke="#c8f03c" strokeWidth="1.5"/>
                <path d="M8 11l2 2 4-4" stroke="#c8f03c" strokeWidth="1.5" strokeLinecap="round"/>
              </svg>
            </div>
            <div className="step-title">run in background</div>
            <div className="step-desc">Exra runs silently when you're on WiFi and charging. Every heartbeat earns you EXRA. No slowdowns. No battery drain. No interruptions.</div>
          </div>
          <div className="step">
            <div className="step-num">03 —</div>
            <div className="step-icon">
              <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
                <path d="M11 3v16M6 8l5-5 5 5" stroke="#c8f03c" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </div>
            <div className="step-title">withdraw anytime</div>
            <div className="step-desc">No minimum threshold. Withdraw from $1. Only TON network fees apply — we take zero. Straight to your crypto wallet instantly.</div>
          </div>
        </div>
      </section>

      <div className="stats-bar">
        <div className="stat-it"><div className="stat-n">any</div><div className="stat-l">device supported</div></div>
        <div className="stat-it"><div className="stat-n">$1</div><div className="stat-l">min withdrawal</div></div>
        <div className="stat-it"><div className="stat-n">0%</div><div className="stat-l">team premine</div></div>
        <div className="stat-it"><div className="stat-n">4</div><div className="stat-l">referral tiers</div></div>
      </div>

      <section className="section" id="devices">
        <div className="ey">02 / devices & tiers</div>
        <h2 className="st">every device<br /><em>earns differently</em></h2>
        <div className="devices">
          <div className="device"><div className="device-rate">~$1.20/day</div><div className="device-name">Phone</div><div className="device-mode">TIER 1 · proxy traffic</div></div>
          <div className="device"><div className="device-rate">~$2.40/day</div><div className="device-name">Laptop / PC</div><div className="device-mode">TIER 1-2 · proxy + compute</div></div>
          <div className="device"><div className="device-rate">~$8.00/day</div><div className="device-name">Gaming PC</div><div className="device-mode">TIER 2 · GPU tasks + proxy</div></div>
          <div className="device"><div className="device-rate">~$0.60/day</div><div className="device-name">Router / RPi</div><div className="device-mode">TIER 1 · proxy · 24/7</div></div>
        </div>
      </section>

      <section className="section" id="token" style={{ paddingTop: 0 }}>
        <div className="ey">03 / tokenomics</div>
        <h2 className="st">fair launch.<br /><em>zero premine.</em></h2>
        <div className="toko-grid" style={{ marginBottom: '1px' }}>
          <div className="toko-card"><div className="toko-num">0</div><div className="toko-title">Team allocation</div><div className="toko-desc">EXRA tokens are minted exclusively through real work — bandwidth shared, tasks computed. No tokens for founders, no investors getting rich before you.</div></div>
          <div className="toko-card"><div className="toko-num">50%</div><div className="toko-title">Worker always gets half</div><div className="toko-desc">Every heartbeat splits emission: 50% to the worker, 10-30% to referrer, rest to treasury. Your cut is guaranteed — nobody takes from you.</div></div>
        </div>
        <div className="toko-grid">
          <div className="toko-card"><div className="toko-num">PoP</div><div className="toko-title">Proof of Presence</div><div className="toko-desc">Earn just by being online. Every 5 minutes your device heartbeats and earns EXRA. No active traffic needed — stable connection rewards you.</div></div>
          <div className="toko-card"><div className="toko-num">TON</div><div className="toko-title">Built on TON</div><div className="toko-desc">Telegram-native distribution and low-fee transfers make micropayments viable at scale. Fast confirmations keep withdrawals responsive for global users.</div></div>
        </div>
      </section>

      <section className="section" style={{ paddingTop: 0 }}>
        <div className="ey">04 / referral tiers</div>
        <h2 className="st">the more you grow<br /><em>the more you earn</em></h2>
        <div className="tiers">
          <div className="tier"><div className="tier-pct">10%</div><div className="tier-name">Street Scout</div><div className="tier-req">1 – 100 referrals</div></div>
          <div className="tier"><div className="tier-pct">15%</div><div className="tier-name">Network Builder</div><div className="tier-req">101 – 300 referrals</div></div>
          <div className="tier"><div className="tier-pct">20%</div><div className="tier-name">Crypto Boss</div><div className="tier-req">301 – 600 referrals</div></div>
          <div className="tier"><div className="tier-pct">30%</div><div className="tier-name">Ambassador</div><div className="tier-req">601 – 1000 referrals</div></div>
        </div>
      </section>

      <section className="section" id="marketplace" style={{ paddingTop: 0 }}>
        <div className="ey">05 / marketplace</div>
        <h2 className="st">buy traffic from<br /><em>real devices</em></h2>
        <div className="market-preview">
          <div className="market-header">
            <span className="market-title">live nodes</span>
            <div className="filters">
              <span className="filter active">all</span>
              <span className="filter">IN</span>
              <span className="filter">BR</span>
              <span className="filter">NG</span>
              <span className="filter">ID</span>
            </div>
          </div>
          <table>
            <thead><tr><th>country</th><th>type</th><th>speed</th><th>status</th><th>price</th><th></th></tr></thead>
            <tbody>
              <tr><td><div className="cc"><span className="flag">IN</span>India</div></td><td style={{ color: 'var(--ink2)', fontSize: '12px', fontFamily: "'JetBrains Mono', monospace" }}>phone</td><td style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '12px', color: 'var(--ink2)' }}>18 Mbps</td><td><span className="sl"><span className="sdot"></span>live</span></td><td><span className="pm">$1.50/GB</span></td><td><Link href="/marketplace" className="buy-btn">buy</Link></td></tr>
              <tr><td><div className="cc"><span className="flag">BR</span>Brazil</div></td><td style={{ color: 'var(--ink2)', fontSize: '12px', fontFamily: "'JetBrains Mono', monospace" }}>laptop</td><td style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '12px', color: 'var(--ink2)' }}>34 Mbps</td><td><span className="sl"><span className="sdot"></span>live</span></td><td><span className="pm">$1.50/GB</span></td><td><Link href="/marketplace" className="buy-btn">buy</Link></td></tr>
              <tr><td><div className="cc"><span className="flag">NG</span>Nigeria</div></td><td style={{ color: 'var(--ink2)', fontSize: '12px', fontFamily: "'JetBrains Mono', monospace" }}>phone</td><td style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '12px', color: 'var(--ink2)' }}>12 Mbps</td><td><span className="sl"><span className="sdot"></span>live</span></td><td><span className="pm">$1.50/GB</span></td><td><Link href="/marketplace" className="buy-btn">buy</Link></td></tr>
              <tr><td><div className="cc"><span className="flag">ID</span>Indonesia</div></td><td style={{ color: 'var(--ink2)', fontSize: '12px', fontFamily: "'JetBrains Mono', monospace" }}>phone</td><td style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '12px', color: 'var(--ink2)' }}>22 Mbps</td><td><span className="sl"><span className="sdot"></span>live</span></td><td><span className="pm">$1.50/GB</span></td><td><Link href="/marketplace" className="buy-btn">buy</Link></td></tr>
              <tr><td><div className="cc"><span className="flag">MX</span>Mexico</div></td><td style={{ color: 'var(--ink2)', fontSize: '12px', fontFamily: "'JetBrains Mono', monospace" }}>pc</td><td style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '12px', color: 'var(--ink2)' }}>55 Mbps</td><td><span className="sl"><span className="sdot"></span>live</span></td><td><span className="pm">$1.50/GB</span></td><td><Link href="/marketplace" className="buy-btn">buy</Link></td></tr>
            </tbody>
          </table>
        </div>
      </section>

      <section className="cta-section">
        <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none', overflow: 'hidden' }}>
          <div className="lava-blob" style={{ width: '500px', height: '500px', background: 'rgba(200,240,60,0.07)', bottom: '-120px', left: '50%', transform: 'translateX(-50%)', filter: 'blur(100px)' }}></div>
        </div>
        <h2 className="cta-title">start earning<br /><em>right now</em></h2>
        <p className="cta-sub">Any device. Any country. Withdraw from $1.</p>
        <div className="cta-actions">
          <Link href="/auth" className="btn-h">download for android</Link>
          <Link href="/marketplace" className="btn-hg">i'm a buyer →</Link>
        </div>
      </section>

      <footer>
        <div className="footer-logo">ex<span>ra</span>.space</div>
        <div className="footer-links">
          <a href="#">docs</a>
          <a href="#">tokenomics</a>
          <a href="#">github</a>
          <a href="#">twitter</a>
        </div>
        <div className="footer-copy">© 2026 exra.network</div>
      </footer>
    </div>
  );
}

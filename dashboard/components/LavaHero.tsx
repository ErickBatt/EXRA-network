"use client"

import React from 'react'

interface Props {
  totalEarned: string
  nodesOnline: number
  exraPrice?: number
  dailyRate?: number
  rank?: number
}

const LavaHero = ({ totalEarned, nodesOnline, exraPrice = 1.5, dailyRate = 1.24, rank }: Props) => {
  const totalExra = (parseFloat(totalEarned) * exraPrice).toFixed(2)

  // Ticker items — live network pulse. Double the array for seamless loop.
  const tickerItems = [
    { label: "24h", value: `+$${dailyRate.toFixed(2)}` },
    { label: "epoch", value: "genesis" },
    { label: "network", value: "peaq mainnet" },
    { label: "your rank", value: rank ? `#${rank.toLocaleString()}` : "unranked" },
    { label: "active nodes", value: `${nodesOnline}` },
  ]

  return (
    <div className="lava-hero">
      <div className="lava-blob" style={{ width: '160px', height: '160px', background: 'rgba(34,211,238,0.42)', top: '-30px', left: '-30px', animation: 'lava1 9s ease-in-out infinite' }} />
      <div className="lava-blob" style={{ width: '130px', height: '130px', background: 'rgba(167,139,250,0.36)', top: '20px', right: '-40px', animation: 'lava2 7s ease-in-out infinite 1s' }} />
      <div className="lava-blob" style={{ width: '140px', height: '140px', background: 'rgba(103,232,249,0.30)', bottom: '-20px', left: '30%', animation: 'lava3 11s ease-in-out infinite 0.5s' }} />
      <div className="lava-blob" style={{ width: '90px', height: '90px', background: 'rgba(124,58,237,0.26)', bottom: '30px', right: '30px', animation: 'lava4 8s ease-in-out infinite 2s' }} />

      <div className="hero-content">
        <div className="hero-top">
          <div>
            <div className="hero-label">Total earned</div>
            <div className="hero-amount">${totalEarned}</div>
            <div className="hero-sub">≈ {totalExra} $EXRA</div>
          </div>
          <div className="hero-badge">
            <span className="badge-dot" />
            {nodesOnline} {nodesOnline === 1 ? 'live' : 'live'}
          </div>
        </div>

        <div className="hero-ticker" aria-hidden>
          <div className="ticker-track">
            {[...tickerItems, ...tickerItems].map((it, i) => (
              <React.Fragment key={i}>
                <span className="ticker-item">
                  {it.label} <b>{it.value}</b>
                </span>
                {i < tickerItems.length * 2 - 1 && <span className="ticker-sep" />}
              </React.Fragment>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export default LavaHero

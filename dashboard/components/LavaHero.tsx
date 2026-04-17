"use client"

import React from 'react'

const LavaHero = ({ totalEarned, nodesOnline }: { totalEarned: string, nodesOnline: number }) => {
  return (
    <div className="lava-hero">
      {/* Lava Blobs */}
      <div className="lava-blob" style={{ width: '130px', height: '130px', background: 'rgba(200,240,60,0.45)', bottom: '-25px', left: '20px', animation: 'lava1 8s ease-in-out infinite' }}></div>
      <div className="lava-blob" style={{ width: '105px', height: '105px', background: 'rgba(168,200,30,0.38)', bottom: '-15px', left: '160px', animation: 'lava2 6.5s ease-in-out infinite 1s' }}></div>
      <div className="lava-blob" style={{ width: '115px', height: '115px', background: 'rgba(140,180,20,0.4)', bottom: '-20px', left: '90px', animation: 'lava3 10s ease-in-out infinite 0.5s' }}></div>
      <div className="lava-blob" style={{ width: '80px', height: '80px', background: 'rgba(200,240,60,0.3)', bottom: '-8px', left: '270px', animation: 'lava4 7s ease-in-out infinite 2s' }}></div>
      
      <div className="hero-content">
        <div className="hero-label">TOTAL EARNED</div>
        <div className="hero-amount">${totalEarned}</div>
        <div className="hero-sub">≈ {(parseFloat(totalEarned) * 1.5).toFixed(2)} EXRA</div>
        <div className="hero-badge">
          <span className="badge-dot"></span>
          {nodesOnline} {nodesOnline === 1 ? 'DEVICE' : 'DEVICES'} ONLINE
        </div>
      </div>
    </div>
  )
}

export default LavaHero

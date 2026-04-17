'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { supabase } from '@/lib/supabase';
import Link from 'next/link';
import './auth.css';

export default function AuthPage() {
  const [mode, setMode] = useState<'magic' | 'password' | 'register'>('magic');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [magicSent, setMagicSent] = useState(false);

  const router = useRouter();

  useEffect(() => {
    // Check if user is already logged in
    const checkUser = async () => {
      const { data: { session } } = await supabase.auth.getSession();
      if (session) {
        router.push('/marketplace');
      }
    };
    checkUser();

    // Listen for auth state changes
    const { data: { subscription } } = supabase.auth.onAuthStateChange((event, session) => {
      if (session && event === 'SIGNED_IN') {
        router.push('/marketplace');
      }
    });

    return () => subscription.unsubscribe();
  }, [router]);

  const handleMagicLink = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!email || !email.includes('@')) {
      setError('Enter a valid email address');
      return;
    }
    setError('');
    setLoading(true);
    const origin = typeof window !== 'undefined' ? window.location.origin : '';
    const { error } = await supabase.auth.signInWithOtp({
      email,
      options: { emailRedirectTo: `${origin}/marketplace` }
    });
    setLoading(false);
    if (error) {
      setError(error.message);
    } else {
      setMagicSent(true);
    }
  };

  const handlePasswordLogin = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!email || !password) {
      setError('Fill in all fields');
      return;
    }
    setError('');
    setLoading(true);
    const { error } = await supabase.auth.signInWithPassword({ email, password });
    setLoading(false);
    if (error) {
      setError(error.message);
    }
  };

  const handleRegister = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!email || !password) {
      setError('Fill in all fields');
      return;
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }
    setError('');
    setLoading(true);
    const origin = typeof window !== 'undefined' ? window.location.origin : '';
    const { error } = await supabase.auth.signUp({
      email,
      password,
      options: { emailRedirectTo: `${origin}/marketplace` }
    });
    setLoading(false);
    if (error) {
      setError(error.message);
    } else {
      setMagicSent(true); // Sign up also sends a verification email
    }
  };

  const handleGoogleLogin = async () => {
    const origin = typeof window !== 'undefined' ? window.location.origin : '';
    const { error } = await supabase.auth.signInWithOAuth({
      provider: 'google',
      options: { redirectTo: `${origin}/marketplace` }
    });
    if (error) setError(error.message);
  };

  if (magicSent) {
    return (
      <div className="auth-root">
        <div className="lava-bg">
          <div className="blob" style={{ width: '500px', height: '500px', background: 'rgba(200,240,60,0.04)', bottom: '-150px', left: '-80px', animation: 'lb1 12s ease-in-out infinite' }}></div>
          <div className="blob" style={{ width: '400px', height: '400px', background: 'rgba(200,240,60,0.03)', bottom: '-100px', right: '-60px', animation: 'lb2 9s ease-in-out infinite 1s' }}></div>
        </div>
        <div className="auth-card">
          <div className="card-logo">
            <div className="logo-text-auth">ex<span>ra</span></div>
            <div className="logo-sub">bandwidth marketplace</div>
          </div>
          <div className="card-body">
            <div className="auth-content">
              <div className="success-box">
                <div className="success-icon">✉️</div>
                <div className="success-title">check your email</div>
                <div className="success-email">{email}</div>
                <div className="success-sub">We sent a sign-in link. Click it and you're in — no password needed.</div>
              </div>
              <button className="btn-primary" style={{ marginTop: '20px', background: 'var(--surface)', color: 'var(--ink2)', border: '0.5px solid var(--border2)' }} onClick={() => setMagicSent(false)}>
                ← use different email
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-root">
      <div className="lava-bg">
        <div className="blob" style={{ width: '500px', height: '500px', background: 'rgba(200,240,60,0.04)', bottom: '-150px', left: '-80px', animation: 'lb1 12s ease-in-out infinite' }}></div>
        <div className="blob" style={{ width: '400px', height: '400px', background: 'rgba(200,240,60,0.03)', bottom: '-100px', right: '-60px', animation: 'lb2 9s ease-in-out infinite 1s' }}></div>
        <div className="blob" style={{ width: '300px', height: '300px', background: 'rgba(200,240,60,0.04)', top: '10%', left: '40%', animation: 'lb3 14s ease-in-out infinite 0.5s' }}></div>
      </div>

      <div className="auth-card">
        <div className="card-logo">
          <div className="logo-text-auth">ex<span>ra</span></div>
          <div className="logo-sub">bandwidth marketplace</div>
        </div>

        <div className="card-body">
          <div className="auth-tabs">
            <button className={`auth-tab ${mode === 'magic' ? 'active' : ''}`} onClick={() => setMode('magic')}>magic link</button>
            <button className={`auth-tab ${['password', 'register'].includes(mode) ? 'active' : ''}`} onClick={() => setMode('password')}>password</button>
          </div>

          <div className="auth-content">
            {error && <div className="error-box">{error}</div>}

            {mode === 'magic' && (
              <form onSubmit={handleMagicLink}>
                <label className="form-label">email address</label>
                <div className="input-wrap">
                  <svg width="15" height="15" viewBox="0 0 15 15" fill="none"><rect x="1" y="3" width="13" height="9" rx="1.5" stroke="#5a5850" strokeWidth="1.2"/><path d="M1 4l6.5 5L14 4" stroke="#5a5850" strokeWidth="1.2" strokeLinecap="round"/></svg>
                  <input type="email" placeholder="you@example.com" value={email} onChange={(e) => setEmail(e.target.value)} />
                </div>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? <div className="spinner"></div> : 'send magic link →'}
                </button>
              </form>
            )}

            {mode === 'password' && (
              <form onSubmit={handlePasswordLogin}>
                <label className="form-label">email address</label>
                <div className="input-wrap">
                  <svg width="15" height="15" viewBox="0 0 15 15" fill="none"><rect x="1" y="3" width="13" height="9" rx="1.5" stroke="#5a5850" strokeWidth="1.2"/><path d="M1 4l6.5 5L14 4" stroke="#5a5850" strokeWidth="1.2" strokeLinecap="round"/></svg>
                  <input type="email" placeholder="you@example.com" value={email} onChange={(e) => setEmail(e.target.value)} />
                </div>
                <label className="form-label">password</label>
                <div className="input-wrap" style={{ marginBottom: '20px' }}>
                  <svg width="15" height="15" viewBox="0 0 15 15" fill="none"><rect x="2" y="6" width="11" height="8" rx="1.5" stroke="#5a5850" strokeWidth="1.2"/><path d="M5 6V4.5a2.5 2.5 0 015 0V6" stroke="#5a5850" strokeWidth="1.2"/><circle cx="7.5" cy="10" r="1" fill="#5a5850"/></svg>
                  <input type="password" placeholder="min 8 characters" value={password} onChange={(e) => setPassword(e.target.value)} />
                </div>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? <div className="spinner"></div> : 'sign in →'}
                </button>
                <div style={{ textAlign: 'center', marginTop: '14px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--ink3)' }}>no account? </span>
                  <button type="button" onClick={() => setMode('register')} style={{ fontSize: '12px', color: 'var(--accent)', background: 'none', border: 'none', cursor: 'pointer', fontFamily: "'Instrument Sans', sans-serif" }}>create one →</button>
                </div>
              </form>
            )}

            {mode === 'register' && (
              <form onSubmit={handleRegister}>
                <label className="form-label">email address</label>
                <div className="input-wrap">
                  <svg width="15" height="15" viewBox="0 0 15 15" fill="none"><rect x="1" y="3" width="13" height="9" rx="1.5" stroke="#5a5850" strokeWidth="1.2"/><path d="M1 4l6.5 5L14 4" stroke="#5a5850" strokeWidth="1.2" strokeLinecap="round"/></svg>
                  <input type="email" placeholder="you@example.com" value={email} onChange={(e) => setEmail(e.target.value)} />
                </div>
                <label className="form-label">password</label>
                <div className="input-wrap" style={{ marginBottom: '20px' }}>
                  <svg width="15" height="15" viewBox="0 0 15 15" fill="none"><rect x="2" y="6" width="11" height="8" rx="1.5" stroke="#5a5850" strokeWidth="1.2"/><path d="M5 6V4.5a2.5 2.5 0 015 0V6" stroke="#5a5850" strokeWidth="1.2"/><circle cx="7.5" cy="10" r="1" fill="#5a5850"/></svg>
                  <input type="password" placeholder="min 8 characters" value={password} onChange={(e) => setPassword(e.target.value)} />
                </div>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? <div className="spinner"></div> : 'create account →'}
                </button>
                <div style={{ textAlign: 'center', marginTop: '14px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--ink3)' }}>already have account? </span>
                  <button type="button" onClick={() => setMode('password')} style={{ fontSize: '12px', color: 'var(--accent)', background: 'none', border: 'none', cursor: 'pointer', fontFamily: "'Instrument Sans', sans-serif" }}>sign in →</button>
                </div>
              </form>
            )}
          </div>
        </div>

        <div className="auth-card-footer">
          By continuing you agree to our <a href="#">Terms</a> and <a href="#">Privacy Policy</a><br />
          <Link href="/" style={{ marginTop: '8px', display: 'inline-block' }}>← back to exra.space</Link>
        </div>
      </div>
    </div>
  );
}

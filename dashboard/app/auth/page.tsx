'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Mail, Lock, ArrowRight, ArrowLeft, AlertCircle, CheckCircle2 } from 'lucide-react';
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
    const checkUser = async () => {
      const { data: { session } } = await supabase.auth.getSession();
      if (session) {
        router.push('/marketplace');
      }
    };
    checkUser();

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
      setMagicSent(true);
    }
  };

  if (magicSent) {
    return (
      <div className="auth-root">
        <div className="lava-bg">
          <div className="blob" style={{ width: '460px', height: '460px', background: 'rgba(34, 211, 238, 0.18)', bottom: '-140px', left: '-80px', animation: 'lb1 12s ease-in-out infinite' }} />
          <div className="blob" style={{ width: '380px', height: '380px', background: 'rgba(167, 139, 250, 0.16)', bottom: '-100px', right: '-60px', animation: 'lb2 10s ease-in-out infinite 1s' }} />
        </div>

        <div className="auth-card">
          <div className="card-logo">
            <div className="logo-text-auth">ex<span>ra</span></div>
            <div className="logo-sub">bandwidth marketplace</div>
          </div>

          <div className="card-body">
            <div className="auth-content">
              <div className="success-box">
                <div className="success-icon">
                  <CheckCircle2 size={28} strokeWidth={1.8} />
                </div>
                <div className="success-title">Check your email</div>
                <div className="success-email">{email}</div>
                <div className="success-sub">
                  We sent a sign-in link. Click it and you&apos;re in — no password needed.
                </div>
              </div>

              <button className="btn-secondary" style={{ marginTop: 20 }} onClick={() => { setMagicSent(false); setError(''); }}>
                <ArrowLeft size={14} strokeWidth={2} /> Use different email
              </button>
            </div>
          </div>

          <div className="auth-card-footer">
            <Link href="/">← back to exra.space</Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-root">
      <div className="lava-bg">
        <div className="blob" style={{ width: '500px', height: '500px', background: 'rgba(34, 211, 238, 0.18)', bottom: '-150px', left: '-80px', animation: 'lb1 12s ease-in-out infinite' }} />
        <div className="blob" style={{ width: '400px', height: '400px', background: 'rgba(167, 139, 250, 0.18)', bottom: '-100px', right: '-60px', animation: 'lb2 9s ease-in-out infinite 1s' }} />
        <div className="blob" style={{ width: '300px', height: '300px', background: 'rgba(34, 211, 238, 0.10)', top: '8%', left: '42%', animation: 'lb3 14s ease-in-out infinite 0.5s' }} />
      </div>

      <div className="auth-card">
        <div className="card-logo">
          <div className="logo-text-auth">ex<span>ra</span></div>
          <div className="logo-sub">bandwidth marketplace</div>
        </div>

        <div className="card-body">
          <div className="auth-tabs">
            <button
              className={`auth-tab ${mode === 'magic' ? 'active' : ''}`}
              onClick={() => { setMode('magic'); setError(''); }}
            >
              Magic link
            </button>
            <button
              className={`auth-tab ${['password', 'register'].includes(mode) ? 'active' : ''}`}
              onClick={() => { setMode('password'); setError(''); }}
            >
              Password
            </button>
          </div>

          <div className="auth-content">
            {error && (
              <div className="error-box">
                <AlertCircle size={14} strokeWidth={2} />
                <span>{error}</span>
              </div>
            )}

            {mode === 'magic' && (
              <form onSubmit={handleMagicLink}>
                <label className="form-label">Email address</label>
                <div className="input-wrap">
                  <Mail size={15} strokeWidth={1.8} />
                  <input
                    type="email"
                    placeholder="you@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    autoFocus
                  />
                </div>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? <div className="spinner" /> : <>Send magic link <ArrowRight size={14} strokeWidth={2.4} /></>}
                </button>
              </form>
            )}

            {mode === 'password' && (
              <form onSubmit={handlePasswordLogin}>
                <label className="form-label">Email address</label>
                <div className="input-wrap">
                  <Mail size={15} strokeWidth={1.8} />
                  <input
                    type="email"
                    placeholder="you@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                </div>
                <label className="form-label">Password</label>
                <div className="input-wrap" style={{ marginBottom: 18 }}>
                  <Lock size={15} strokeWidth={1.8} />
                  <input
                    type="password"
                    placeholder="min 8 characters"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                </div>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? <div className="spinner" /> : <>Sign in <ArrowRight size={14} strokeWidth={2.4} /></>}
                </button>
                <div className="mode-switch">
                  No account?
                  <button type="button" onClick={() => { setMode('register'); setError(''); }}>Create one →</button>
                </div>
              </form>
            )}

            {mode === 'register' && (
              <form onSubmit={handleRegister}>
                <label className="form-label">Email address</label>
                <div className="input-wrap">
                  <Mail size={15} strokeWidth={1.8} />
                  <input
                    type="email"
                    placeholder="you@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                </div>
                <label className="form-label">Password</label>
                <div className="input-wrap" style={{ marginBottom: 18 }}>
                  <Lock size={15} strokeWidth={1.8} />
                  <input
                    type="password"
                    placeholder="min 8 characters"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                </div>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? <div className="spinner" /> : <>Create account <ArrowRight size={14} strokeWidth={2.4} /></>}
                </button>
                <div className="mode-switch">
                  Already have an account?
                  <button type="button" onClick={() => { setMode('password'); setError(''); }}>Sign in →</button>
                </div>
              </form>
            )}
          </div>
        </div>

        <div className="auth-card-footer">
          By continuing you agree to our <a href="#">Terms</a> and <a href="#">Privacy Policy</a><br />
          <Link href="/" style={{ marginTop: 8, display: 'inline-block' }}>← back to exra.space</Link>
        </div>
      </div>
    </div>
  );
}

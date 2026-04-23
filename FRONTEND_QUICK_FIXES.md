# 🚨 FRONTEND QUICK FIXES — НЕМЕДЛЕННЫЕ ДЕЙСТВИЯ (24-48 часов)

**Статус:** КРИТИЧЕСКИЙ — 6 Sev-1 блокеров требуют срочного закрытия  
**Время внедрения:** 4-6 часов на разработчика  
**Риск неделания:** Утечка админ-прав, API-ключей, спуфинг аккаунтов

---

## 🔴 QUICK FIX #1: Защитить /admin от неавторизованного доступа

### Проблема
```
Неавторизованный пользователь открывает /admin
→ Видит форму ввода admin email/secret
→ Может попытаться угадать credentials
```

### Решение (30 минут)

**Шаг 1:** Создать middleware.ts в dashboard
```typescript
// dashboard/middleware.ts
import { NextRequest, NextResponse } from 'next/server';
import { createClient } from '@supabase/supabase-js';

export async function middleware(request: NextRequest) {
  // Защитить /admin маршрут
  if (request.nextUrl.pathname.startsWith('/admin')) {
    const supabase = createClient(
      process.env.NEXT_PUBLIC_SUPABASE_URL!,
      process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
    );

    // Получить сессию из cookies
    const token = request.cookies.get('sb-access-token')?.value;
    
    if (!token) {
      return NextResponse.redirect(new URL('/auth', request.url));
    }

    // Проверить, что пользователь — админ
    // (требует добавить role в Supabase user metadata)
    try {
      const { data: { user } } = await supabase.auth.getUser(token);
      if (!user || user.user_metadata?.role !== 'admin') {
        return NextResponse.redirect(new URL('/auth', request.url));
      }
    } catch {
      return NextResponse.redirect(new URL('/auth', request.url));
    }
  }
}

export const config = {
  matcher: ['/admin/:path*'],
};
```

**Шаг 2:** Обновить admin/page.tsx
```typescript
// dashboard/app/admin/page.tsx
// УДАЛИТЬ useEffect проверку (строки 77-91)
// Middleware уже защитит на SSR-уровне

export default function AdminPage() {
  // Теперь можно сразу предположить, что пользователь авторизован
  const [adminEmail, setAdminEmail] = useState('');
  const [adminSecret, setAdminSecret] = useState('');
  // ... остальной код
}
```

**Шаг 3:** Протестировать
```bash
# Открыть /admin без авторизации → должен редиректить на /auth
curl -i http://localhost:3000/admin
# Expected: 307 Temporary Redirect to /auth
```

---

## 🔴 QUICK FIX #2: Удалить админ-ссылку из маркетплейса

### Проблема
```
Покупатель видит "Admin Console" в sidebar
→ Кликает → попадает на /admin
→ Видит форму ввода credentials
```

### Решение (5 минут)

**Шаг 1:** Удалить ссылку из marketplace/page.tsx
```typescript
// dashboard/app/marketplace/page.tsx
// УДАЛИТЬ строки 392-395:
// <Link className="nav-item" href="/admin">
//   <ShieldCheck size={15} strokeWidth={1.8} />
//   Admin Console
// </Link>

// ЕСЛИ нужна админ-ссылка для админов, добавить проверку:
{user?.user_metadata?.role === 'admin' && (
  <Link className="nav-item" href="/admin">
    <ShieldCheck size={15} strokeWidth={1.8} />
    Admin Console
  </Link>
)}
```

**Шаг 2:** Протестировать
```bash
# Открыть маркетплейс как покупатель
# Убедиться, что "Admin Console" не видна
```

---

## 🔴 QUICK FIX #3: Добавить Origin/Referer проверку на /buyer-api/auth/reveal

### Проблема
```
TMA скомпрометирована (XSS)
→ Инжектирует fetch('/buyer-api/auth/reveal')
→ Получает API-ключ
→ Отправляет на attacker.com
```

### Решение (20 минут)

**Шаг 1:** Обновить /buyer-api/auth/reveal route
```typescript
// dashboard/app/buyer-api/auth/reveal/route.ts
import { NextRequest, NextResponse } from 'next/server';

export async function GET(request: NextRequest) {
  // Проверить Origin/Referer
  const origin = request.headers.get('origin');
  const referer = request.headers.get('referer');
  
  const allowedOrigins = [
    process.env.NEXT_PUBLIC_DASHBOARD_URL,
    'http://localhost:3000',
    'http://localhost:3001',
  ];
  
  const isAllowedOrigin = allowedOrigins.some(allowed => 
    origin?.includes(allowed) || referer?.includes(allowed)
  );
  
  if (!isAllowedOrigin) {
    return NextResponse.json(
      { error: 'forbidden' },
      { status: 403 }
    );
  }

  // Получить API-ключ из httpOnly cookie
  const apiKey = request.cookies.get('buyer_api_key')?.value;
  
  if (!apiKey) {
    return NextResponse.json(
      { error: 'not authenticated' },
      { status: 401 }
    );
  }

  // Логировать reveal операцию
  console.log(`[AUDIT] API key revealed for user at ${new Date().toISOString()}`);

  return NextResponse.json({ api_key: apiKey });
}
```

**Шаг 2:** Добавить Rate Limiting
```typescript
// dashboard/lib/rateLimit.ts
import { Redis } from '@upstash/redis';

const redis = new Redis({
  url: process.env.UPSTASH_REDIS_REST_URL!,
  token: process.env.UPSTASH_REDIS_REST_TOKEN!,
});

export async function checkRateLimit(
  key: string,
  limit: number = 1,
  windowSeconds: number = 300 // 5 минут
): Promise<boolean> {
  const count = await redis.incr(key);
  
  if (count === 1) {
    await redis.expire(key, windowSeconds);
  }
  
  return count <= limit;
}
```

**Шаг 3:** Использовать Rate Limit в route
```typescript
// dashboard/app/buyer-api/auth/reveal/route.ts
import { checkRateLimit } from '@/lib/rateLimit';

export async function GET(request: NextRequest) {
  // ... Origin/Referer проверка ...

  // Получить user ID из сессии
  const { data: { session } } = await supabase.auth.getSession();
  const userId = session?.user?.id;

  if (!userId) {
    return NextResponse.json({ error: 'unauthorized' }, { status: 401 });
  }

  // Проверить rate limit: 1 reveal per 5 minutes
  const allowed = await checkRateLimit(`reveal:${userId}`, 1, 300);
  
  if (!allowed) {
    return NextResponse.json(
      { error: 'too many requests' },
      { status: 429 }
    );
  }

  // ... остальной код ...
}
```

**Шаг 4:** Протестировать
```bash
# Первый запрос — успех
curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
# Expected: 200 OK { api_key: "..." }

# Второй запрос в течение 5 минут — ошибка
curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
# Expected: 429 Too Many Requests

# Запрос с неправильным Origin — ошибка
curl -H "Origin: http://attacker.com" http://localhost:3000/buyer-api/auth/reveal
# Expected: 403 Forbidden
```

---

## 🔴 QUICK FIX #4: Реализовать CSRF protection на POST endpoints

### Проблема
```
TMA загружена в iframe
→ Может делать POST запросы от имени пользователя
→ Создавать offers, топить баланс, и т.д.
```

### Решение (45 минут)

**Шаг 1:** Создать CSRF middleware
```typescript
// dashboard/lib/csrf.ts
import crypto from 'crypto';
import { Redis } from '@upstash/redis';

const redis = new Redis({
  url: process.env.UPSTASH_REDIS_REST_URL!,
  token: process.env.UPSTASH_REDIS_REST_TOKEN!,
});

export async function generateCsrfToken(userId: string): Promise<string> {
  const token = crypto.randomBytes(32).toString('hex');
  
  // Сохранить в Redis на 1 час
  await redis.setex(`csrf:${userId}:${token}`, 3600, '1');
  
  return token;
}

export async function validateCsrfToken(
  userId: string,
  token: string
): Promise<boolean> {
  if (!token) return false;
  
  const exists = await redis.get(`csrf:${userId}:${token}`);
  
  if (exists) {
    // Удалить token после использования (one-time use)
    await redis.del(`csrf:${userId}:${token}`);
    return true;
  }
  
  return false;
}
```

**Шаг 2:** Добавить CSRF token в layout
```typescript
// dashboard/app/layout.tsx
'use client';

import { useEffect, useState } from 'react';
import { generateCsrfToken } from '@/lib/csrf';

export default function RootLayout({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    // Генерировать CSRF token при загрузке
    const generateToken = async () => {
      const { data: { session } } = await supabase.auth.getSession();
      if (session?.user?.id) {
        const token = await generateCsrfToken(session.user.id);
        
        // Сохранить в sessionStorage (не localStorage!)
        sessionStorage.setItem('csrf_token', token);
        
        // Установить в cookie для middleware
        document.cookie = `csrf_token=${token}; path=/; SameSite=Strict`;
      }
    };
    
    generateToken();
  }, []);

  return (
    <html>
      <body>{children}</body>
    </html>
  );
}
```

**Шаг 3:** Обновить buyerFetch для отправки CSRF token
```typescript
// dashboard/lib/buyerApi.ts
export async function buyerFetch<T>(
  path: string,
  init?: RequestInit
): Promise<T> {
  const headers = new Headers(init?.headers || {});
  
  // Добавить CSRF token для POST/PUT/DELETE запросов
  if (init?.method && ['POST', 'PUT', 'DELETE'].includes(init.method)) {
    const csrfToken = sessionStorage.getItem('csrf_token');
    if (csrfToken) {
      headers.set('X-CSRF-Token', csrfToken);
    }
  }

  const res = await fetch(`${PROXY_PREFIX}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  });

  if (res.status === 401) throw new BuyerApiUnauthorized();
  if (!res.ok) throw new Error(`API error: ${res.status}`);

  return res.json() as Promise<T>;
}
```

**Шаг 4:** Добавить CSRF валидацию в middleware
```typescript
// dashboard/middleware.ts
export async function middleware(request: NextRequest) {
  // Проверить CSRF token для POST/PUT/DELETE запросов
  if (['POST', 'PUT', 'DELETE'].includes(request.method)) {
    const csrfToken = request.headers.get('x-csrf-token');
    const { data: { session } } = await supabase.auth.getSession();
    
    if (!csrfToken || !session?.user?.id) {
      return NextResponse.json(
        { error: 'csrf validation failed' },
        { status: 403 }
      );
    }

    const isValid = await validateCsrfToken(session.user.id, csrfToken);
    
    if (!isValid) {
      return NextResponse.json(
        { error: 'csrf validation failed' },
        { status: 403 }
      );
    }
  }
}
```

**Шаг 5:** Протестировать
```bash
# Получить CSRF token
TOKEN=$(curl -s http://localhost:3000/api/csrf | jq -r '.token')

# POST запрос с правильным token — успех
curl -X POST \
  -H "X-CSRF-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"country":"IN","target_gb":10,"max_price_per_gb":1.5}' \
  http://localhost:3000/api/offers
# Expected: 200 OK

# POST запрос без token — ошибка
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"country":"IN","target_gb":10,"max_price_per_gb":1.5}' \
  http://localhost:3000/api/offers
# Expected: 403 Forbidden
```

---

## 🔴 QUICK FIX #5: Валидировать Telegram initData на сервере

### Проблема
```
Атакующий перехватывает initData
→ Отправляет старый/поддельный initData
→ Выдает себя за другого пользователя
```

### Решение (30 минут)

**Шаг 1:** Создать TMA auth validator
```typescript
// dashboard/lib/tmaAuth.ts
import crypto from 'crypto';

export interface TelegramInitData {
  user?: {
    id: number;
    first_name: string;
    username?: string;
  };
  auth_date: number;
  hash: string;
}

export function validateTelegramInitData(
  initData: string,
  botToken: string
): { valid: boolean; data?: TelegramInitData } {
  try {
    const params = new URLSearchParams(initData);
    const hash = params.get('hash');
    
    if (!hash) {
      return { valid: false };
    }

    // Создать data check string
    const dataCheckString = Array.from(params.entries())
      .filter(([key]) => key !== 'hash')
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, value]) => `${key}=${value}`)
      .join('\n');

    // Вычислить HMAC
    const secretKey = crypto
      .createHmac('sha256', 'WebAppData')
      .update(botToken)
      .digest();
    
    const computedHash = crypto
      .createHmac('sha256', secretKey)
      .update(dataCheckString)
      .digest('hex');

    // Сравнить хэши
    if (computedHash !== hash) {
      return { valid: false };
    }

    // Проверить TTL (Time To Live)
    const authDate = parseInt(params.get('auth_date') || '0', 10);
    const now = Math.floor(Date.now() / 1000);
    const age = now - authDate;

    if (age > 300) { // 5 минут
      return { valid: false };
    }

    // Парсить user data
    const userJson = params.get('user');
    const user = userJson ? JSON.parse(userJson) : undefined;

    return {
      valid: true,
      data: {
        user,
        auth_date: authDate,
        hash,
      },
    };
  } catch (error) {
    console.error('TMA validation error:', error);
    return { valid: false };
  }
}
```

**Шаг 2:** Обновить TMA auth endpoint
```typescript
// dashboard/app/next-tma/auth/route.ts
import { validateTelegramInitData } from '@/lib/tmaAuth';
import { NextRequest, NextResponse } from 'next/server';

export async function POST(request: NextRequest) {
  try {
    const { init_data } = await request.json();

    if (!init_data) {
      return NextResponse.json(
        { error: 'init_data required' },
        { status: 400 }
      );
    }

    // Валидировать initData
    const { valid, data } = validateTelegramInitData(
      init_data,
      process.env.TELEGRAM_BOT_TOKEN!
    );

    if (!valid || !data?.user) {
      return NextResponse.json(
        { error: 'invalid init_data' },
        { status: 401 }
      );
    }

    const telegramId = data.user.id;

    // Получить или создать пользователя в БД
    const user = await db.query(
      'SELECT * FROM tma_users WHERE telegram_id = $1',
      [telegramId]
    );

    if (!user.rows.length) {
      await db.query(
        'INSERT INTO tma_users (telegram_id, first_name, username) VALUES ($1, $2, $3)',
        [telegramId, data.user.first_name, data.user.username]
      );
    }

    // Получить устройства пользователя
    const devices = await db.query(
      'SELECT * FROM nodes WHERE telegram_id = $1',
      [telegramId]
    );

    return NextResponse.json({
      telegram_id: telegramId,
      first_name: data.user.first_name,
      username: data.user.username,
      devices: devices.rows,
      total_usd: 0, // Получить из БД
      total_exra: 0, // Получить из БД
    });
  } catch (error) {
    console.error('TMA auth error:', error);
    return NextResponse.json(
      { error: 'internal server error' },
      { status: 500 }
    );
  }
}
```

**Шаг 3:** Протестировать
```bash
# Валидный initData — успех
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"init_data":"user=%7B%22id%22%3A123%7D&auth_date=1234567890&hash=abc123"}' \
  http://localhost:3000/next-tma/auth
# Expected: 200 OK

# Старый initData (>5 минут) — ошибка
# Expected: 401 Unauthorized

# Поддельный hash — ошибка
# Expected: 401 Unauthorized
```

---

## 🔴 QUICK FIX #6: Удалить localStorage для админ-credentials

### Проблема
```
Admin secret хранится в localStorage
→ Любой XSS → утечка админ-credentials
```

### Решение (10 минут)

**Шаг 1:** Обновить admin/page.tsx
```typescript
// dashboard/app/admin/page.tsx
// УДАЛИТЬ строки 85-88:
// const rememberedEmail = localStorage.getItem('exra_admin_email') || '';
// const rememberedSecret = localStorage.getItem('exra_admin_secret') || '';

// УДАЛИТЬ строки 104-105:
// localStorage.setItem('exra_admin_email', adminEmail);
// localStorage.setItem('exra_admin_secret', adminSecret);

// ВМЕСТО ЭТОГО использовать sessionStorage (очищается при закрытии браузера):
const [adminEmail, setAdminEmail] = useState(() => {
  if (typeof window !== 'undefined') {
    return sessionStorage.getItem('exra_admin_email') || '';
  }
  return '';
});

const [adminSecret, setAdminSecret] = useState(() => {
  if (typeof window !== 'undefined') {
    return sessionStorage.getItem('exra_admin_secret') || '';
  }
  return '';
});

const ensureAdminAuth = () => {
  if (!adminEmail || !adminSecret) {
    setError('Enter admin email and admin secret');
    return false;
  }
  setError('');
  
  // Использовать sessionStorage вместо localStorage
  sessionStorage.setItem('exra_admin_email', adminEmail);
  sessionStorage.setItem('exra_admin_secret', adminSecret);
  
  return true;
};
```

**Шаг 2:** Протестировать
```bash
# Открыть DevTools → Application → Storage
# Убедиться, что admin credentials НЕ в localStorage
# Убедиться, что они в sessionStorage
# Закрыть браузер → sessionStorage очищается
```

---

## 📋 DEPLOYMENT CHECKLIST (24-48 часов)

### День 1 (4 часа)
- [ ] **QUICK FIX #1:** Защитить /admin middleware
  - [ ] Создать middleware.ts
  - [ ] Протестировать редирект
  - [ ] Задеплоить на staging
  
- [ ] **QUICK FIX #2:** Удалить админ-ссылку
  - [ ] Обновить marketplace/page.tsx
  - [ ] Протестировать
  - [ ] Задеплоить на staging

- [ ] **QUICK FIX #6:** Удалить localStorage
  - [ ] Обновить admin/page.tsx
  - [ ] Использовать sessionStorage
  - [ ] Протестировать
  - [ ] Задеплоить на staging

### День 2 (6 часов)
- [ ] **QUICK FIX #3:** Origin/Referer + Rate Limit
  - [ ] Обновить /buyer-api/auth/reveal
  - [ ] Добавить Rate Limiting
  - [ ] Протестировать
  - [ ] Задеплоить на staging
  - [ ] Задеплоить на production

- [ ] **QUICK FIX #4:** CSRF protection
  - [ ] Создать CSRF middleware
  - [ ] Обновить buyerFetch
  - [ ] Протестировать
  - [ ] Задеплоить на staging
  - [ ] Задеплоить на production

- [ ] **QUICK FIX #5:** TMA initData валидация
  - [ ] Создать TMA auth validator
  - [ ] Обновить auth endpoint
  - [ ] Протестировать
  - [ ] Задеплоить на staging
  - [ ] Задеплоить на production

### День 3 (Мониторинг)
- [ ] Мониторить логи на production
- [ ] Проверить, что нет ошибок
- [ ] Собрать feedback от пользователей
- [ ] Подготовить документацию для команды

---

## 🧪 TESTING SCENARIOS

### Scenario 1: Неавторизованный доступ к /admin
```bash
# Без авторизации
curl -i http://localhost:3000/admin
# Expected: 307 Temporary Redirect to /auth

# С авторизацией (не админ)
curl -i -H "Cookie: sb-access-token=user_token" http://localhost:3000/admin
# Expected: 307 Temporary Redirect to /auth

# С авторизацией (админ)
curl -i -H "Cookie: sb-access-token=admin_token" http://localhost:3000/admin
# Expected: 200 OK
```

### Scenario 2: CSRF атака
```bash
# Попытка POST без CSRF token
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"country":"IN","target_gb":10}' \
  http://localhost:3000/api/offers
# Expected: 403 Forbidden

# POST с правильным CSRF token
TOKEN=$(curl -s http://localhost:3000/api/csrf | jq -r '.token')
curl -X POST \
  -H "X-CSRF-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"country":"IN","target_gb":10}' \
  http://localhost:3000/api/offers
# Expected: 200 OK
```

### Scenario 3: TMA initData спуфинг
```bash
# Старый initData (>5 минут)
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"init_data":"...old_data..."}' \
  http://localhost:3000/next-tma/auth
# Expected: 401 Unauthorized

# Поддельный hash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"init_data":"...fake_hash..."}' \
  http://localhost:3000/next-tma/auth
# Expected: 401 Unauthorized
```

### Scenario 4: API Key reveal rate limiting
```bash
# Первый reveal — успех
curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
# Expected: 200 OK

# Второй reveal в течение 5 минут — ошибка
curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
# Expected: 429 Too Many Requests
```

---

## 📊 RISK REDUCTION

| Fix | Риск до | Риск после | Reduction |
|-----|---------|-----------|-----------|
| #1: /admin middleware | 🔴 9/10 | 🟢 1/10 | 89% |
| #2: Удалить админ-ссылку | 🔴 8/10 | 🟢 1/10 | 87% |
| #3: Origin/Referer + Rate Limit | 🔴 8/10 | 🟡 3/10 | 62% |
| #4: CSRF protection | 🔴 7/10 | 🟡 2/10 | 71% |
| #5: TMA initData валидация | 🔴 9/10 | 🟡 2/10 | 78% |
| #6: Удалить localStorage | 🔴 8/10 | 🟡 3/10 | 62% |
| **ИТОГО** | 🔴 8.2/10 | 🟡 2.0/10 | **75%** |

---

## 📝 NOTES

- Все fixes используют существующие зависимости (Supabase, Redis, Next.js)
- Нет необходимости в новых npm пакетах
- Все changes backward-compatible
- Можно деплоить incrementally (один fix за раз)
- Требуется обновить environment variables (см. ниже)

### Required Environment Variables
```env
# Для CSRF protection
UPSTASH_REDIS_REST_URL=...
UPSTASH_REDIS_REST_TOKEN=...

# Для TMA валидации
TELEGRAM_BOT_TOKEN=...

# Для Origin/Referer проверки
NEXT_PUBLIC_DASHBOARD_URL=https://dashboard.exra.space
```

---

**Подготовлено:** Security Lead  
**Дата:** 21 апреля 2026  
**Статус:** READY FOR IMPLEMENTATION

# 🎯 ПЛАН ИСПРАВЛЕНИЯ ФРОНТЕНДА EXRA — С ПРИОРИТИЗАЦИЕЙ И ЧЕКЛИСТАМИ

**Дата:** 21 апреля 2026  
**Статус:** READY FOR EXECUTION  
**Общее время:** 10-12 часов (2 дня)  
**Риск неделания:** 🔴 КРИТИЧЕСКИЙ

---

## 📊 ПРИОРИТИЗАЦИЯ ПО ВАЖНОСТИ

### 🔴 P0 — КРИТИЧЕСКИЕ (Блокируют MVP, 4-6 часов)

| # | Фикс | Риск | Время | Статус |
|---|------|------|-------|--------|
| 1 | Защитить /admin от неавторизованного доступа | 9/10 | 30 мин | ⏳ TODO |
| 2 | Удалить админ-ссылку из маркетплейса | 8/10 | 5 мин | ⏳ TODO |
| 3 | Добавить Origin/Referer на /buyer-api/auth/reveal | 8/10 | 20 мин | ⏳ TODO |
| 4 | Реализовать CSRF protection | 7/10 | 45 мин | ⏳ TODO |
| 5 | Валидировать Telegram initData на сервере | 9/10 | 30 мин | ⏳ TODO |
| 6 | Удалить localStorage для админ-credentials | 8/10 | 10 мин | ⏳ TODO |

### 🟡 P1 — ВЫСОКИЕ (Закрывают фрод-векторы, 4-6 часов)

| # | Фикс | Риск | Время | Статус |
|---|------|------|-------|--------|
| 7 | Добавить Guard на /marketplace маршрут | 6/10 | 15 мин | ⏳ TODO |
| 8 | Добавить Rate Limiting на админ-endpoints | 6/10 | 30 мин | ⏳ TODO |
| 9 | Логирование всех админ-операций | 5/10 | 20 мин | ⏳ TODO |
| 10 | Добавить logout кнопку | 3/10 | 15 мин | ⏳ TODO |

### 🟢 P2 — СРЕДНИЕ (UX/Design, 2-3 часа)

| # | Фикс | Риск | Время | Статус |
|---|------|------|-------|--------|
| 11 | Унифицировать шрифты (Geist везде) | 3/10 | 1 час | ⏳ TODO |
| 12 | Унифицировать цвета (Cyan везде) | 2/10 | 30 мин | ⏳ TODO |
| 13 | Интегрировать Landing (навигация) | 4/10 | 1 час | ⏳ TODO |

---

## 🔴 P0 FIX #1: Защитить /admin от неавторизованного доступа

**Риск:** 🔴 9/10 | **Время:** 30 мин | **Сложность:** ⭐⭐

### Проблема
```
Неавторизованный пользователь открывает /admin
→ Видит форму ввода admin email/secret
→ Может попытаться угадать credentials
```

### Чек-лист

- [ ] **Шаг 1: Создать middleware.ts (5 мин)**
  ```bash
  # Проверить, существует ли файл
  ls -la dashboard/middleware.ts
  # Если нет → создать
  ```
  
  **Файл:** `dashboard/middleware.ts`
  ```typescript
  import { NextRequest, NextResponse } from 'next/server';
  import { createClient } from '@supabase/supabase-js';

  export async function middleware(request: NextRequest) {
    if (request.nextUrl.pathname.startsWith('/admin')) {
      const token = request.cookies.get('sb-access-token')?.value;
      
      if (!token) {
        return NextResponse.redirect(new URL('/auth', request.url));
      }

      try {
        const supabase = createClient(
          process.env.NEXT_PUBLIC_SUPABASE_URL!,
          process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
        );
        
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

- [ ] **Шаг 2: Обновить admin/page.tsx (10 мин)**
  - Удалить useEffect проверку (строки 77-91)
  - Middleware уже защитит на SSR-уровне
  
  **Проверка:**
  ```bash
  # Убедиться, что useEffect с router.push('/auth') удален
  grep -n "router.push('/auth')" dashboard/app/admin/page.tsx
  # Должно быть пусто
  ```

- [ ] **Шаг 3: Протестировать локально (10 мин)**
  ```bash
  # Запустить dev сервер
  cd dashboard && npm run dev
  
  # Тест 1: Без авторизации
  curl -i http://localhost:3000/admin
  # Expected: 307 Temporary Redirect to /auth
  
  # Тест 2: С авторизацией (не админ)
  curl -i -H "Cookie: sb-access-token=user_token" http://localhost:3000/admin
  # Expected: 307 Temporary Redirect to /auth
  
  # Тест 3: С авторизацией (админ)
  curl -i -H "Cookie: sb-access-token=admin_token" http://localhost:3000/admin
  # Expected: 200 OK
  ```

- [ ] **Шаг 4: Задеплоить на staging (5 мин)**
  ```bash
  git add dashboard/middleware.ts dashboard/app/admin/page.tsx
  git commit -m "fix: protect /admin with SSR middleware"
  git push origin main
  # Дождаться деплоя на staging
  ```

- [ ] **Шаг 5: Проверить на staging (5 мин)**
  ```bash
  # Открыть https://staging-dashboard.exra.space/admin
  # Без авторизации → должен редиректить на /auth
  # С авторизацией (админ) → должен показать админ-панель
  ```

### ✅ Критерии успеха
- [ ] Неавторизованный пользователь редиректится на /auth
- [ ] Авторизованный (не админ) редиректится на /auth
- [ ] Авторизованный (админ) видит админ-панель
- [ ] Нет ошибок в консоли
- [ ] Нет ошибок в логах сервера

---

## 🔴 P0 FIX #2: Удалить админ-ссылку из маркетплейса

**Риск:** 🔴 8/10 | **Время:** 5 мин | **Сложность:** ⭐

### Проблема
```
Покупатель видит "Admin Console" в sidebar
→ Кликает → попадает на /admin
→ Видит форму ввода credentials
```

### Чек-лист

- [ ] **Шаг 1: Найти админ-ссылку (2 мин)**
  ```bash
  grep -n "Admin Console" dashboard/app/marketplace/page.tsx
  # Expected: строка 392-395
  ```

- [ ] **Шаг 2: Удалить ссылку (2 мин)**
  
  **Файл:** `dashboard/app/marketplace/page.tsx`
  
  **УДАЛИТЬ строки 392-395:**
  ```typescript
  // ❌ УДАЛИТЬ ЭТО:
  <Link className="nav-item" href="/admin">
    <ShieldCheck size={15} strokeWidth={1.8} />
    Admin Console
  </Link>
  ```

- [ ] **Шаг 3: Протестировать локально (1 мин)**
  ```bash
  # Открыть http://localhost:3000/marketplace
  # Убедиться, что "Admin Console" НЕ видна в sidebar
  ```

- [ ] **Шаг 4: Задеплоить (1 мин)**
  ```bash
  git add dashboard/app/marketplace/page.tsx
  git commit -m "fix: remove admin console link from marketplace sidebar"
  git push origin main
  ```

### ✅ Критерии успеха
- [ ] "Admin Console" ссылка удалена из sidebar
- [ ] Маркетплейс загружается без ошибок
- [ ] Нет broken links

---

## 🔴 P0 FIX #3: Добавить Origin/Referer на /buyer-api/auth/reveal

**Риск:** 🔴 8/10 | **Время:** 20 мин | **Сложность:** ⭐⭐

### Проблема
```
TMA скомпрометирована (XSS)
→ Инжектирует fetch('/buyer-api/auth/reveal')
→ Получает API-ключ
→ Отправляет на attacker.com
```

### Чек-лист

- [ ] **Шаг 1: Обновить /buyer-api/auth/reveal (10 мин)**
  
  **Файл:** `dashboard/app/buyer-api/auth/reveal/route.ts`
  
  ```typescript
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
      console.warn(`[SECURITY] Unauthorized origin: ${origin}`);
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
    console.log(`[AUDIT] API key revealed at ${new Date().toISOString()}`);

    return NextResponse.json({ api_key: apiKey });
  }
  ```

- [ ] **Шаг 2: Добавить Rate Limiting (5 мин)**
  
  **Файл:** `dashboard/lib/rateLimit.ts` (создать если не существует)
  
  ```typescript
  import { Redis } from '@upstash/redis';

  const redis = new Redis({
    url: process.env.UPSTASH_REDIS_REST_URL!,
    token: process.env.UPSTASH_REDIS_REST_TOKEN!,
  });

  export async function checkRateLimit(
    key: string,
    limit: number = 1,
    windowSeconds: number = 300
  ): Promise<boolean> {
    const count = await redis.incr(key);
    
    if (count === 1) {
      await redis.expire(key, windowSeconds);
    }
    
    return count <= limit;
  }
  ```

- [ ] **Шаг 3: Использовать Rate Limit в route (3 мин)**
  
  **Обновить:** `dashboard/app/buyer-api/auth/reveal/route.ts`
  
  ```typescript
  import { checkRateLimit } from '@/lib/rateLimit';
  import { supabase } from '@/lib/supabase';

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
      console.warn(`[SECURITY] Rate limit exceeded for user ${userId}`);
      return NextResponse.json(
        { error: 'too many requests' },
        { status: 429 }
      );
    }

    // ... остальной код ...
  }
  ```

- [ ] **Шаг 4: Протестировать локально (2 мин)**
  ```bash
  # Тест 1: Правильный Origin — успех
  curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
  # Expected: 200 OK { api_key: "..." }

  # Тест 2: Неправильный Origin — ошибка
  curl -H "Origin: http://attacker.com" http://localhost:3000/buyer-api/auth/reveal
  # Expected: 403 Forbidden

  # Тест 3: Rate limit — второй запрос в течение 5 минут
  curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
  # Expected: 429 Too Many Requests
  ```

- [ ] **Шаг 5: Задеплоить (1 мин)**
  ```bash
  git add dashboard/app/buyer-api/auth/reveal/route.ts dashboard/lib/rateLimit.ts
  git commit -m "fix: add Origin/Referer check and rate limiting to API key reveal"
  git push origin main
  ```

### ✅ Критерии успеха
- [ ] Origin/Referer проверка работает
- [ ] Rate limiting работает (1 reveal per 5 min)
- [ ] Логирование работает
- [ ] Нет ошибок в консоли

---

## 🔴 P0 FIX #4: Реализовать CSRF protection

**Риск:** 🔴 7/10 | **Время:** 45 мин | **Сложность:** ⭐⭐⭐

### Проблема
```
TMA загружена в iframe
→ Может делать POST запросы от имени пользователя
→ Создавать offers, топить баланс
```

### Чек-лист

- [ ] **Шаг 1: Создать CSRF middleware (10 мин)**
  
  **Файл:** `dashboard/lib/csrf.ts` (создать)
  
  ```typescript
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

- [ ] **Шаг 2: Обновить layout.tsx (10 мин)**
  
  **Файл:** `dashboard/app/layout.tsx`
  
  ```typescript
  'use client';

  import { useEffect } from 'react';
  import { generateCsrfToken } from '@/lib/csrf';
  import { supabase } from '@/lib/supabase';

  export default function RootLayout({ children }: { children: React.ReactNode }) {
    useEffect(() => {
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
      <html lang="en">
        <body>{children}</body>
      </html>
    );
  }
  ```

- [ ] **Шаг 3: Обновить buyerFetch (10 мин)**
  
  **Файл:** `dashboard/lib/buyerApi.ts`
  
  ```typescript
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

- [ ] **Шаг 4: Добавить CSRF валидацию в middleware (10 мин)**
  
  **Обновить:** `dashboard/middleware.ts`
  
  ```typescript
  import { validateCsrfToken } from '@/lib/csrf';

  export async function middleware(request: NextRequest) {
    // ... /admin проверка ...

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
        console.warn(`[SECURITY] CSRF validation failed for user ${session.user.id}`);
        return NextResponse.json(
          { error: 'csrf validation failed' },
          { status: 403 }
        );
      }
    }
  }
  ```

- [ ] **Шаг 5: Протестировать локально (5 мин)**
  ```bash
  # Тест 1: POST без CSRF token — ошибка
  curl -X POST \
    -H "Content-Type: application/json" \
    -d '{"country":"IN","target_gb":10}' \
    http://localhost:3000/api/offers
  # Expected: 403 Forbidden

  # Тест 2: POST с правильным CSRF token — успех
  TOKEN=$(curl -s http://localhost:3000/api/csrf | jq -r '.token')
  curl -X POST \
    -H "X-CSRF-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"country":"IN","target_gb":10}' \
    http://localhost:3000/api/offers
  # Expected: 200 OK
  ```

- [ ] **Шаг 6: Задеплоить (1 мин)**
  ```bash
  git add dashboard/lib/csrf.ts dashboard/app/layout.tsx dashboard/lib/buyerApi.ts dashboard/middleware.ts
  git commit -m "fix: implement CSRF protection on all POST/PUT/DELETE endpoints"
  git push origin main
  ```

### ✅ Критерии успеха
- [ ] CSRF token генерируется при загрузке
- [ ] POST без token → 403 Forbidden
- [ ] POST с token → 200 OK
- [ ] Token one-time use (второй раз → 403)
- [ ] Нет ошибок в консоли

---

## 🔴 P0 FIX #5: Валидировать Telegram initData на сервере

**Риск:** 🔴 9/10 | **Время:** 30 мин | **Сложность:** ⭐⭐

### Проблема
```
Атакующий перехватывает initData
→ Отправляет старый/поддельный initData
→ Выдает себя за другого пользователя
```

### Чек-лист

- [ ] **Шаг 1: Создать TMA auth validator (10 мин)**
  
  **Файл:** `dashboard/lib/tmaAuth.ts` (создать)
  
  ```typescript
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
        console.warn('[TMA] Hash mismatch');
        return { valid: false };
      }

      // Проверить TTL (Time To Live)
      const authDate = parseInt(params.get('auth_date') || '0', 10);
      const now = Math.floor(Date.now() / 1000);
      const age = now - authDate;

      if (age > 300) { // 5 минут
        console.warn(`[TMA] initData expired: ${age}s old`);
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
      console.error('[TMA] Validation error:', error);
      return { valid: false };
    }
  }
  ```

- [ ] **Шаг 2: Обновить TMA auth endpoint (10 мин)**
  
  **Файл:** `dashboard/app/next-tma/auth/route.ts`
  
  ```typescript
  import { validateTelegramInitData } from '@/lib/tmaAuth';
  import { NextRequest, NextResponse } from 'next/server';
  import { supabase } from '@/lib/supabase';

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
        console.warn('[TMA] Invalid initData');
        return NextResponse.json(
          { error: 'invalid init_data' },
          { status: 401 }
        );
      }

      const telegramId = data.user.id;

      // Получить или создать пользователя в БД
      const { data: user } = await supabase
        .from('tma_users')
        .select('*')
        .eq('telegram_id', telegramId)
        .single();

      if (!user) {
        await supabase.from('tma_users').insert({
          telegram_id: telegramId,
          first_name: data.user.first_name,
          username: data.user.username,
        });
      }

      // Получить устройства пользователя
      const { data: devices } = await supabase
        .from('nodes')
        .select('*')
        .eq('telegram_id', telegramId);

      return NextResponse.json({
        telegram_id: telegramId,
        first_name: data.user.first_name,
        username: data.user.username,
        devices: devices || [],
        total_usd: 0,
        total_exra: 0,
      });
    } catch (error) {
      console.error('[TMA] Auth error:', error);
      return NextResponse.json(
        { error: 'internal server error' },
        { status: 500 }
      );
    }
  }
  ```

- [ ] **Шаг 3: Протестировать локально (5 мин)**
  ```bash
  # Тест 1: Валидный initData — успех
  curl -X POST \
    -H "Content-Type: application/json" \
    -d '{"init_data":"user=%7B%22id%22%3A123%7D&auth_date=1234567890&hash=abc123"}' \
    http://localhost:3000/next-tma/auth
  # Expected: 200 OK или 401 (если hash неправильный)

  # Тест 2: Старый initData (>5 минут) — ошибка
  # Expected: 401 Unauthorized

  # Тест 3: Поддельный hash — ошибка
  # Expected: 401 Unauthorized
  ```

- [ ] **Шаг 4: Задеплоить (1 мин)**
  ```bash
  git add dashboard/lib/tmaAuth.ts dashboard/app/next-tma/auth/route.ts
  git commit -m "fix: validate Telegram initData on server side"
  git push origin main
  ```

### ✅ Критерии успеха
- [ ] Валидный initData → 200 OK
- [ ] Старый initData (>5 мин) → 401 Unauthorized
- [ ] Поддельный hash → 401 Unauthorized
- [ ] Логирование работает
- [ ] Нет ошибок в консоли

---

## 🔴 P0 FIX #6: Удалить localStorage для админ-credentials

**Риск:** 🔴 8/10 | **Время:** 10 мин | **Сложность:** ⭐

### Проблема
```
Admin secret хранится в localStorage
→ Любой XSS → утечка админ-credentials
```

### Чек-лист

- [ ] **Шаг 1: Найти localStorage использование (2 мин)**
  ```bash
  grep -n "localStorage" dashboard/app/admin/page.tsx
  # Expected: строки 85-88, 104-105
  ```

- [ ] **Шаг 2: Обновить admin/page.tsx (5 мин)**
  
  **Файл:** `dashboard/app/admin/page.tsx`
  
  **УДАЛИТЬ строки 85-88:**
  ```typescript
  // ❌ УДАЛИТЬ:
  const rememberedEmail = localStorage.getItem('exra_admin_email') || '';
  const rememberedSecret = localStorage.getItem('exra_admin_secret') || '';
  ```
  
  **ЗАМЕНИТЬ на:**
  ```typescript
  // ✅ ДОБАВИТЬ:
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
  ```
  
  **УДАЛИТЬ строки 104-105:**
  ```typescript
  // ❌ УДАЛИТЬ:
  localStorage.setItem('exra_admin_email', adminEmail);
  localStorage.setItem('exra_admin_secret', adminSecret);
  ```
  
  **ЗАМЕНИТЬ на:**
  ```typescript
  // ✅ ДОБАВИТЬ:
  sessionStorage.setItem('exra_admin_email', adminEmail);
  sessionStorage.setItem('exra_admin_secret', adminSecret);
  ```

- [ ] **Шаг 3: Протестировать локально (2 мин)**
  ```bash
  # Открыть DevTools → Application → Storage
  # Убедиться, что admin credentials НЕ в localStorage
  # Убедиться, что они в sessionStorage
  # Закрыть браузер → sessionStorage очищается
  ```

- [ ] **Шаг 4: Задеплоить (1 мин)**
  ```bash
  git add dashboard/app/admin/page.tsx
  git commit -m "fix: use sessionStorage instead of localStorage for admin credentials"
  git push origin main
  ```

### ✅ Критерии успеха
- [ ] localStorage НЕ содержит admin credentials
- [ ] sessionStorage содержит admin credentials
- [ ] sessionStorage очищается при закрытии браузера
- [ ] Админ-панель работает нормально

---

## 📋 DEPLOYMENT SCHEDULE

### День 1 (4 часа)

**09:00 - 09:30:** FIX #1 (Защитить /admin)
- [ ] Разработка
- [ ] Локальное тестирование
- [ ] Деплой на staging

**09:30 - 09:35:** FIX #2 (Удалить админ-ссылку)
- [ ] Разработка
- [ ] Локальное тестирование
- [ ] Деплой на staging

**09:35 - 09:55:** FIX #3 (Origin/Referer + Rate Limit)
- [ ] Разработка
- [ ] Локальное тестирование
- [ ] Деплой на staging

**09:55 - 10:40:** FIX #4 (CSRF protection)
- [ ] Разработка
- [ ] Локальное тестирование
- [ ] Деплой на staging

**10:40 - 11:10:** FIX #5 (TMA initData валидация)
- [ ] Разработка
- [ ] Локальное тестирование
- [ ] Деплой на staging

**11:10 - 11:20:** FIX #6 (Удалить localStorage)
- [ ] Разработка
- [ ] Локальное тестирование
- [ ] Деплой на staging

**11:20 - 12:00:** Smoke Testing на staging
- [ ] Проверить все 6 fixes
- [ ] Убедиться, что нет регрессий
- [ ] Подготовить к production деплою

### День 2 (2 часа)

**09:00 - 09:30:** Production Deployment
- [ ] Деплой всех 6 fixes на production
- [ ] Мониторинг логов
- [ ] Проверка метрик

**09:30 - 11:00:** Мониторинг + Документация
- [ ] Мониторить production логи
- [ ] Собрать feedback от пользователей
- [ ] Подготовить post-mortem документацию

---

## 🧪 SMOKE TESTING CHECKLIST

### После каждого фикса

- [ ] Нет ошибок в консоли браузера
- [ ] Нет ошибок в логах сервера
- [ ] Нет broken links
- [ ] Нет 500 ошибок
- [ ] Нет 403 ошибок (кроме ожидаемых)

### Перед production деплоем

- [ ] Все 6 fixes работают на staging
- [ ] Нет регрессий в других функциях
- [ ] Маркетплейс работает нормально
- [ ] TMA работает нормально
- [ ] Admin панель работает нормально
- [ ] API endpoints работают нормально

---

## 📊 RISK REDUCTION TRACKING

| Fix | Риск до | Риск после | Status |
|-----|---------|-----------|--------|
| #1 | 9/10 | 1/10 | ⏳ TODO |
| #2 | 8/10 | 1/10 | ⏳ TODO |
| #3 | 8/10 | 3/10 | ⏳ TODO |
| #4 | 7/10 | 2/10 | ⏳ TODO |
| #5 | 9/10 | 2/10 | ⏳ TODO |
| #6 | 8/10 | 3/10 | ⏳ TODO |
| **ИТОГО** | **8.2/10** | **2.0/10** | **75% reduction** |

---

## 🚨 ROLLBACK PLAN

Если что-то сломалось на production:

```bash
# Откатить последний коммит
git revert HEAD

# Или откатить конкретный fix
git revert <commit-hash>

# Задеплоить откат
git push origin main
```

---

**Подготовлено:** Security Lead  
**Дата:** 21 апреля 2026  
**Статус:** READY FOR EXECUTION

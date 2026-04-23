# ✅ ГАРАНТИИ КАЧЕСТВА И ПРАВИЛЬНОСТИ ИСПРАВЛЕНИЙ

**Статус:** 🟢 ЗЕЛЕНЫЙ СВЕТ НА ВСЕ ИСПРАВЛЕНИЯ  
**Принцип:** Делать грамотно и правильно, сохраняя весь функционал  
**Дата:** 21 апреля 2026

---

## 🎯 ГЛАВНЫЙ ПРИНЦИП

> **Не обрезаем функционал. Не упрощаем. Делаем правильно.**
> 
> Каждый фикс должен:
> - ✅ Решить проблему безопасности
> - ✅ Сохранить весь существующий функционал
> - ✅ Улучшить UX (или оставить как было)
> - ✅ Пройти все тесты
> - ✅ Быть готовым к production

---

## 🔍 QUALITY ASSURANCE CHECKLIST

### Перед началом каждого фикса

- [ ] **Прочитать весь код** (не только строки, которые меняем)
- [ ] **Понять зависимости** (какие компоненты используют этот код)
- [ ] **Проверить тесты** (есть ли unit/integration тесты)
- [ ] **Создать feature branch** (`git checkout -b fix/admin-security`)
- [ ] **Не трогать другой код** (только необходимые изменения)

### Во время разработки

- [ ] **Сохранить весь функционал** (ничего не удаляем без причины)
- [ ] **Добавить логирование** (для отладки и аудита)
- [ ] **Обработать ошибки** (graceful error handling)
- [ ] **Добавить комментарии** (объяснить почему так)
- [ ] **Следовать стилю кода** (существующие conventions)

### После разработки

- [ ] **Локальное тестирование** (все сценарии)
- [ ] **Проверка регрессий** (не сломали ли другое)
- [ ] **Code review** (попросить review у коллеги)
- [ ] **Staging тестирование** (полный smoke test)
- [ ] **Production готовность** (все ли работает)

---

## 🔴 P0 FIX #1: Защитить /admin — ПРАВИЛЬНО

### ❌ НЕПРАВИЛЬНО (обрезание функционала):
```typescript
// Просто удалить useEffect и надеяться на middleware
// → Потеряется функционал проверки сессии на клиенте
// → Может быть race condition
```

### ✅ ПРАВИЛЬНО (сохранить весь функционал):

**Шаг 1: Добавить middleware (SSR-уровень)**
```typescript
// dashboard/middleware.ts
export async function middleware(request: NextRequest) {
  if (request.nextUrl.pathname.startsWith('/admin')) {
    const token = request.cookies.get('sb-access-token')?.value;
    
    if (!token) {
      return NextResponse.redirect(new URL('/auth', request.url));
    }

    try {
      const supabase = createClient(...);
      const { data: { user } } = await supabase.auth.getUser(token);
      
      if (!user || user.user_metadata?.role !== 'admin') {
        return NextResponse.redirect(new URL('/auth', request.url));
      }
    } catch (error) {
      console.error('[SECURITY] Middleware auth error:', error);
      return NextResponse.redirect(new URL('/auth', request.url));
    }
  }
}
```

**Шаг 2: Оставить useEffect (для client-side проверки)**
```typescript
// dashboard/app/admin/page.tsx
useEffect(() => {
  const init = async () => {
    const { data: { session } } = await supabase.auth.getSession();
    
    // Middleware уже защитит, но client-side проверка — дополнительная защита
    if (!session || session.user?.user_metadata?.role !== 'admin') {
      router.push('/auth');
      return;
    }
    
    setSessionReady(true);
  };
  
  init();
}, [router]);
```

**Почему так правильно:**
- ✅ SSR-уровень защиты (первая линия)
- ✅ Client-side проверка (вторая линия)
- ✅ Defense in depth (многоуровневая защита)
- ✅ Сохранен весь функционал
- ✅ Нет race conditions

### 🧪 Тестирование FIX #1:

```bash
# Тест 1: Без авторизации → middleware редиректит
curl -i http://localhost:3000/admin
# Expected: 307 Temporary Redirect to /auth

# Тест 2: С авторизацией (не админ) → middleware редиректит
curl -i -H "Cookie: sb-access-token=user_token" http://localhost:3000/admin
# Expected: 307 Temporary Redirect to /auth

# Тест 3: С авторизацией (админ) → middleware пропускает
curl -i -H "Cookie: sb-access-token=admin_token" http://localhost:3000/admin
# Expected: 200 OK (затем client-side проверка)

# Тест 4: Проверить, что админ-панель работает нормально
# Открыть http://localhost:3000/admin как админ
# Убедиться, что все функции работают:
# - Загрузка данных
# - Фильтрация
# - Пагинация
# - Все кнопки работают
```

### ✅ Критерии успеха:
- [ ] Middleware работает (SSR-уровень)
- [ ] Client-side проверка работает
- [ ] Нет race conditions
- [ ] Весь функционал админ-панели работает
- [ ] Нет ошибок в консоли
- [ ] Нет ошибок в логах

---

## 🔴 P0 FIX #3: Origin/Referer + Rate Limit — ПРАВИЛЬНО

### ❌ НЕПРАВИЛЬНО (обрезание функционала):
```typescript
// Просто добавить Origin проверку и забыть про Rate Limit
// → Атакующий может делать много запросов
// → Нет логирования для аудита
```

### ✅ ПРАВИЛЬНО (полная защита):

**Шаг 1: Origin/Referer проверка**
```typescript
export async function GET(request: NextRequest) {
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
    // Логировать попытку атаки
    console.warn(`[SECURITY] Unauthorized origin: ${origin} from ${request.ip}`);
    
    // Отправить в систему мониторинга (если есть)
    await logSecurityEvent({
      type: 'unauthorized_origin',
      origin,
      ip: request.ip,
      timestamp: new Date(),
    });
    
    return NextResponse.json(
      { error: 'forbidden' },
      { status: 403 }
    );
  }
  
  // ... остальной код ...
}
```

**Шаг 2: Rate Limiting**
```typescript
// Проверить rate limit: 1 reveal per 5 minutes
const allowed = await checkRateLimit(`reveal:${userId}`, 1, 300);

if (!allowed) {
  // Логировать попытку обхода rate limit
  console.warn(`[SECURITY] Rate limit exceeded for user ${userId}`);
  
  // Отправить в систему мониторинга
  await logSecurityEvent({
    type: 'rate_limit_exceeded',
    userId,
    endpoint: '/buyer-api/auth/reveal',
    timestamp: new Date(),
  });
  
  return NextResponse.json(
    { error: 'too many requests' },
    { status: 429 }
  );
}
```

**Шаг 3: Логирование успешного reveal**
```typescript
// Логировать успешный reveal (для аудита)
console.log(`[AUDIT] API key revealed for user ${userId} at ${new Date().toISOString()}`);

// Отправить в систему мониторинга
await logAuditEvent({
  type: 'api_key_revealed',
  userId,
  timestamp: new Date(),
  ip: request.ip,
});

return NextResponse.json({ api_key: apiKey });
```

### 🧪 Тестирование FIX #3:

```bash
# Тест 1: Правильный Origin — успех
curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
# Expected: 200 OK { api_key: "..." }
# Проверить логи: [AUDIT] API key revealed

# Тест 2: Неправильный Origin — ошибка
curl -H "Origin: http://attacker.com" http://localhost:3000/buyer-api/auth/reveal
# Expected: 403 Forbidden
# Проверить логи: [SECURITY] Unauthorized origin

# Тест 3: Rate limit — второй запрос в течение 5 минут
curl -H "Origin: http://localhost:3000" http://localhost:3000/buyer-api/auth/reveal
# Expected: 429 Too Many Requests
# Проверить логи: [SECURITY] Rate limit exceeded

# Тест 4: Проверить, что API-ключ работает нормально
# Использовать полученный API-ключ для других запросов
# Убедиться, что все работает как раньше
```

### ✅ Критерии успеха:
- [ ] Origin/Referer проверка работает
- [ ] Rate limiting работает (1 reveal per 5 min)
- [ ] Логирование работает (все события логируются)
- [ ] API-ключ работает нормально
- [ ] Весь функционал сохранен

---

## 🔴 P0 FIX #4: CSRF Protection — ПРАВИЛЬНО

### ❌ НЕПРАВИЛЬНО (обрезание функционала):
```typescript
// Просто добавить CSRF token и забыть про остальное
// → Может быть race condition при генерации token
// → Нет обработки ошибок
// → Нет логирования
```

### ✅ ПРАВИЛЬНО (полная защита):

**Шаг 1: CSRF token генерация (с обработкой ошибок)**
```typescript
export async function generateCsrfToken(userId: string): Promise<string> {
  try {
    const token = crypto.randomBytes(32).toString('hex');
    
    // Сохранить в Redis на 1 час
    await redis.setex(`csrf:${userId}:${token}`, 3600, '1');
    
    console.log(`[AUDIT] CSRF token generated for user ${userId}`);
    
    return token;
  } catch (error) {
    console.error('[ERROR] Failed to generate CSRF token:', error);
    throw new Error('Failed to generate CSRF token');
  }
}
```

**Шаг 2: CSRF token валидация (с обработкой ошибок)**
```typescript
export async function validateCsrfToken(
  userId: string,
  token: string
): Promise<boolean> {
  try {
    if (!token) {
      console.warn(`[SECURITY] CSRF token missing for user ${userId}`);
      return false;
    }
    
    const exists = await redis.get(`csrf:${userId}:${token}`);
    
    if (exists) {
      // Удалить token после использования (one-time use)
      await redis.del(`csrf:${userId}:${token}`);
      
      console.log(`[AUDIT] CSRF token validated for user ${userId}`);
      
      return true;
    }
    
    console.warn(`[SECURITY] Invalid CSRF token for user ${userId}`);
    return false;
  } catch (error) {
    console.error('[ERROR] Failed to validate CSRF token:', error);
    return false;
  }
}
```

**Шаг 3: Обработка ошибок в middleware**
```typescript
export async function middleware(request: NextRequest) {
  if (['POST', 'PUT', 'DELETE'].includes(request.method)) {
    try {
      const csrfToken = request.headers.get('x-csrf-token');
      const { data: { session } } = await supabase.auth.getSession();
      
      if (!csrfToken || !session?.user?.id) {
        console.warn('[SECURITY] CSRF validation failed: missing token or session');
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
    } catch (error) {
      console.error('[ERROR] CSRF middleware error:', error);
      return NextResponse.json(
        { error: 'internal server error' },
        { status: 500 }
      );
    }
  }
}
```

### 🧪 Тестирование FIX #4:

```bash
# Тест 1: POST без CSRF token — ошибка
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"country":"IN","target_gb":10}' \
  http://localhost:3000/api/offers
# Expected: 403 Forbidden
# Проверить логи: [SECURITY] CSRF validation failed

# Тест 2: POST с правильным CSRF token — успех
TOKEN=$(curl -s http://localhost:3000/api/csrf | jq -r '.token')
curl -X POST \
  -H "X-CSRF-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"country":"IN","target_gb":10}' \
  http://localhost:3000/api/offers
# Expected: 200 OK
# Проверить логи: [AUDIT] CSRF token validated

# Тест 3: Повторное использование token — ошибка (one-time use)
curl -X POST \
  -H "X-CSRF-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"country":"IN","target_gb":10}' \
  http://localhost:3000/api/offers
# Expected: 403 Forbidden
# Проверить логи: [SECURITY] Invalid CSRF token

# Тест 4: Проверить, что весь функционал работает
# Создать несколько offers
# Убедиться, что все работает как раньше
```

### ✅ Критерии успеха:
- [ ] CSRF token генерируется при загрузке
- [ ] POST без token → 403 Forbidden
- [ ] POST с token → 200 OK
- [ ] Token one-time use (второй раз → 403)
- [ ] Логирование работает
- [ ] Весь функционал сохранен

---

## 🔴 P0 FIX #5: TMA initData валидация — ПРАВИЛЬНО

### ❌ НЕПРАВИЛЬНО (обрезание функционала):
```typescript
// Просто добавить валидацию и забыть про обработку ошибок
// → Может быть race condition
// → Нет graceful degradation
// → Нет логирования
```

### ✅ ПРАВИЛЬНО (полная валидация):

**Шаг 1: Валидация с обработкой ошибок**
```typescript
export function validateTelegramInitData(
  initData: string,
  botToken: string
): { valid: boolean; data?: TelegramInitData; error?: string } {
  try {
    const params = new URLSearchParams(initData);
    const hash = params.get('hash');
    
    if (!hash) {
      console.warn('[TMA] Hash missing in initData');
      return { valid: false, error: 'hash_missing' };
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
      return { valid: false, error: 'hash_mismatch' };
    }

    // Проверить TTL (Time To Live)
    const authDate = parseInt(params.get('auth_date') || '0', 10);
    const now = Math.floor(Date.now() / 1000);
    const age = now - authDate;

    if (age > 300) { // 5 минут
      console.warn(`[TMA] initData expired: ${age}s old`);
      return { valid: false, error: 'expired' };
    }

    // Парсить user data
    const userJson = params.get('user');
    const user = userJson ? JSON.parse(userJson) : undefined;

    if (!user || !user.id) {
      console.warn('[TMA] User data missing or invalid');
      return { valid: false, error: 'invalid_user' };
    }

    console.log(`[AUDIT] TMA initData validated for user ${user.id}`);

    return {
      valid: true,
      data: {
        user,
        auth_date: authDate,
        hash,
      },
    };
  } catch (error) {
    console.error('[ERROR] TMA validation error:', error);
    return { valid: false, error: 'validation_error' };
  }
}
```

**Шаг 2: Использование валидации с graceful degradation**
```typescript
export async function POST(request: NextRequest) {
  try {
    const { init_data } = await request.json();

    if (!init_data) {
      console.warn('[TMA] init_data missing');
      return NextResponse.json(
        { error: 'init_data required' },
        { status: 400 }
      );
    }

    // Валидировать initData
    const { valid, data, error } = validateTelegramInitData(
      init_data,
      process.env.TELEGRAM_BOT_TOKEN!
    );

    if (!valid || !data?.user) {
      console.warn(`[SECURITY] TMA validation failed: ${error}`);
      
      // Логировать попытку атаки
      await logSecurityEvent({
        type: 'tma_validation_failed',
        error,
        timestamp: new Date(),
      });
      
      return NextResponse.json(
        { error: 'invalid init_data' },
        { status: 401 }
      );
    }

    const telegramId = data.user.id;

    // Получить или создать пользователя в БД
    const { data: user, error: userError } = await supabase
      .from('tma_users')
      .select('*')
      .eq('telegram_id', telegramId)
      .single();

    if (userError && userError.code !== 'PGRST116') {
      // PGRST116 = no rows returned (это нормально)
      throw userError;
    }

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

    console.log(`[AUDIT] TMA user ${telegramId} authenticated`);

    return NextResponse.json({
      telegram_id: telegramId,
      first_name: data.user.first_name,
      username: data.user.username,
      devices: devices || [],
      total_usd: 0,
      total_exra: 0,
    });
  } catch (error) {
    console.error('[ERROR] TMA auth error:', error);
    
    // Логировать ошибку
    await logSecurityEvent({
      type: 'tma_auth_error',
      error: error instanceof Error ? error.message : 'unknown',
      timestamp: new Date(),
    });
    
    return NextResponse.json(
      { error: 'internal server error' },
      { status: 500 }
    );
  }
}
```

### 🧪 Тестирование FIX #5:

```bash
# Тест 1: Валидный initData — успех
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"init_data":"user=%7B%22id%22%3A123%7D&auth_date=1234567890&hash=abc123"}' \
  http://localhost:3000/next-tma/auth
# Expected: 200 OK или 401 (если hash неправильный)

# Тест 2: Старый initData (>5 минут) — ошибка
# Expected: 401 Unauthorized
# Проверить логи: [TMA] initData expired

# Тест 3: Поддельный hash — ошибка
# Expected: 401 Unauthorized
# Проверить логи: [TMA] Hash mismatch

# Тест 4: Проверить, что TMA работает нормально
# Открыть TMA в Telegram
# Убедиться, что все функции работают:
# - Загрузка баланса
# - Загрузка устройств
# - Вывод средств
# - Все кнопки работают
```

### ✅ Критерии успеха:
- [ ] Валидный initData → 200 OK
- [ ] Старый initData (>5 мин) → 401 Unauthorized
- [ ] Поддельный hash → 401 Unauthorized
- [ ] Логирование работает
- [ ] Весь функционал TMA работает

---

## 📋 ФИНАЛЬНЫЙ CHECKLIST ПЕРЕД PRODUCTION

### Перед деплоем на production

- [ ] **Все 6 fixes разработаны и протестированы**
- [ ] **Все тесты пройдены** (unit + integration + smoke)
- [ ] **Нет регрессий** (весь функционал работает)
- [ ] **Логирование работает** (все события логируются)
- [ ] **Мониторинг настроен** (alerts для security events)
- [ ] **Rollback plan готов** (можем откатить за 5 минут)
- [ ] **Документация обновлена** (все изменения задокументированы)
- [ ] **Team уведомлен** (все знают о деплое)

### После деплоя на production

- [ ] **Мониторить логи** (первые 2 часа)
- [ ] **Проверить метрики** (нет ошибок, нет spike)
- [ ] **Собрать feedback** (от пользователей)
- [ ] **Проверить security events** (нет атак)
- [ ] **Подготовить post-mortem** (что сделали, что выучили)

---

## 🚨 ЕСЛИ ЧТО-ТО СЛОМАЛОСЬ

**Немедленно:**
1. Откатить последний коммит: `git revert HEAD`
2. Задеплоить откат: `git push origin main`
3. Мониторить логи (убедиться, что откат сработал)
4. Уведомить team

**Потом:**
1. Разобраться, что сломалось
2. Исправить в feature branch
3. Протестировать еще раз
4. Задеплоить снова

---

## 📊 МЕТРИКИ УСПЕХА

| Метрика | Целевое значение | Как проверить |
|---------|------------------|---------------|
| Все тесты пройдены | 100% | `npm test` |
| Нет регрессий | 0 broken features | Manual testing |
| Логирование работает | 100% events logged | Check logs |
| Security events | 0 attacks | Monitor alerts |
| Performance | <100ms latency | Check metrics |
| Uptime | 99.9% | Check monitoring |

---

**Подготовлено:** Security Lead  
**Дата:** 21 апреля 2026  
**Статус:** 🟢 ЗЕЛЕНЫЙ СВЕТ НА ВСЕ ИСПРАВЛЕНИЯ

package handlers

// matcher_concurrency_test.go — доказательные тесты для находок B1/B2.
//
// ФИНДИНГИ:
//
//   B1. Нет атомарного claim'а ноды: при двух одновременных CreateOfferAndMatch
//       оба запроса видят одинаковый top-N из Redis ZSET, оба применяют одну и
//       ту же детерминированную формулу и выбирают ОДНУ И ТУ ЖЕ bestNode.
//       Нет ZREM/ZPOPMIN/SETNX между выбором и выдачей JWT → double-booking.
//
//   B2. Формула отклоняется от ТЗ:
//         ТЗ v2.4.1:   50% RS + 50% Latency/Geo
//         Реально:     0.4*price + 0.3*RS + 0.2*Uptime + 0.1*peak + rand*0.05
//       Вес Reputation Score занижен с 0.5 до 0.3 (−40%), Latency/Geo полностью
//       отсутствует. Это подменяет "репутация решает" на "цена решает".

import (
	"exra/models"
	"math"
	"math/rand"
	"os"
	"sync"
	"testing"
)

// TestScoreFormula_RSWeightBelowSpec (B2)
//
// Проверяет, что formula weight для RSScore ≠ 0.5 как требует ТЗ.
// Мы изолируем вклад RS, зафиксировав все остальные переменные, и измеряем
// дельту score на двух узлах, отличающихся ТОЛЬКО по RSScore.
func TestScoreFormula_RSWeightBelowSpec(t *testing.T) {
	// Убираем рандом из формулы для детерминизма теста:
	// calculateBidScore добавляет rand.Float64()*0.05 — мы запускаем
	// несколько итераций и усредняем, иначе тест был бы флекки.
	rand.Seed(1) // nolint:staticcheck — для детерминизма в старых версиях Go

	const N = 10_000
	const offerPrice, avgPrice = 1.50, 1.50

	// Два узла: одинаковые кроме RS. Node A RS=1000 (max), Node B RS=0 (min).
	nodeHighRS := models.PublicNode{
		RSScore: 1000.0,
		RSTier:  "A",
		Uptime:  0.99,
	}
	nodeLowRS := models.PublicNode{
		RSScore: 0.0,
		RSTier:  "A",
		Uptime:  0.99,
	}

	var sumHigh, sumLow float64
	for i := 0; i < N; i++ {
		sumHigh += calculateBidScore("test-session", offerPrice, avgPrice, nodeHighRS)
		sumLow += calculateBidScore("test-session", offerPrice, avgPrice, nodeLowRS)
	}
	avgDelta := (sumHigh - sumLow) / N

	// Разница score при изменении RS от 0 до 1000 должна быть ≈ 0.5 (согласно
	// ТЗ "50% RS + 50% Latency/Geo"). В текущей реализации:
	//   delta = 0.3 * (1000/1000) - 0.3 * (0/1000) = 0.3
	//
	// Принимаем допуск ±0.05 из-за rand-компонента.
	const specWeight = 0.5

	// Тест FAIL'ит на текущем коде (weight=0.3), PASS'ит после фикса (weight=0.5).
	if math.Abs(avgDelta-specWeight) > 0.05 {
		t.Errorf(
			"BUG B2: формула отклоняется от ТЗ v2.4.1. "+
				"Наблюдаемый вес RS в calculateBidScore = %.3f. "+
				"Ожидалось по ТЗ '50%% RS + 50%% Latency/Geo' = %.3f. "+
				"Реальная формула: 0.4*price + 0.3*RS + 0.2*Uptime + "+
				"0.1*peakBonus + rand*0.05. Компонент Latency/Geo "+
				"ОТСУТСТВУЕТ полностью. Это превращает matcher из "+
				"'репутация-ориентированного' в 'цено-ориентированный', "+
				"что противоречит описанию в ТЗ. Требуется: вычислять "+
				"latencyScore = 1 - min(node.RTT/maxRTT, 1) и geoScore по "+
				"haversine(buyer_country, node.Country); финальная "+
				"формула = 0.5*RS + 0.25*latency + 0.25*geo.",
			avgDelta, specWeight,
		)
	}
}

// TestMatcher_DeterministicBestPick_EnablesDoubleBooking (B1)
//
// Детерминированность выбора bestNode при отсутствии atomic-claim означает:
// два одновременных запроса, видящих один и тот же снимок ZSET, выберут ОДНУ
// ноду. В реальном коде нет операции ZREM/ZPOPMIN после выбора, поэтому
// второй запрос не увидит, что нода уже "занята".
//
// Тест:
//  1) Генерируем фиксированный список из 10 PublicNode.
//  2) Запускаем 100 горутин (симулирующих одновременных buyer'ов), каждая
//     независимо прогоняет scoring-loop на ОДНОМ И ТОМ ЖЕ снимке.
//  3) Собираем ID выбранной bestNode от каждой горутины.
//  4) Если бы был атомарный claim, мы бы ожидали разнообразия в выборе.
//     Реально — все 100 горутин сходятся на одной ноде.
//
// ПОСЛЕ ФИКСА: тест будет падать, потому что после выбора нода будет
// атомарно удаляться из pool'а (ZPOPMIN / Lua-lease) и следующий запрос
// увидит уже другое множество.
func TestMatcher_DeterministicBestPick_EnablesDoubleBooking(t *testing.T) {
	src, err := readLocalFile("matcher.go")
	if err != nil {
		t.Skipf("cannot read matcher.go: %v", err)
	}
	if containsBytes(src, []byte("AtomicClaimNode(")) {
		t.Skip("AtomicClaimNode already present in matcher.go; B1 fix applied and this legacy proof test needs a Redis-backed integration replacement")
	}
	// Набор нод с разными характеристиками.
	nodes := []models.PublicNode{
		{ID: "node-0", RSScore: 800, RSTier: "A", Uptime: 0.95, PricePerGB: 1.4},
		{ID: "node-1", RSScore: 950, RSTier: "A", Uptime: 0.98, PricePerGB: 1.5}, // ← доминирующая
		{ID: "node-2", RSScore: 700, RSTier: "B", Uptime: 0.90, PricePerGB: 1.3},
		{ID: "node-3", RSScore: 500, RSTier: "B", Uptime: 0.85, PricePerGB: 1.2},
		{ID: "node-4", RSScore: 300, RSTier: "C", Uptime: 0.70, PricePerGB: 1.1},
	}

	const (
		concurrentBuyers   = 100
		offerPrice, avgPrc = 1.50, 1.50
	)

	picks := make([]string, concurrentBuyers)
	var wg sync.WaitGroup

	// Имитируем 100 одновременных CreateOfferAndMatch на ОДНОМ снимке ZSET.
	// Каждая горутина независимо прогоняет ту же самую scoring-loop что и
	// matcher.go:82-93.
	for i := 0; i < concurrentBuyers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Точная копия логики CreateOfferAndMatch scoring loop.
			bestScore := -1.0
			bestID := ""
			for _, n := range nodes {
				s := calculateBidScore("test-session", offerPrice, avgPrc, n)
				if s > bestScore {
					bestScore = s
					bestID = n.ID
				}
			}
			picks[idx] = bestID
		}(i)
	}
	wg.Wait()

	// Считаем уникальных победителей. Из-за rand*0.05-jitter в формуле, иногда
	// другой узел может победить, но доминирующая нода (node-1) должна
	// набирать подавляющее большинство голосов.
	counts := map[string]int{}
	for _, id := range picks {
		counts[id]++
	}

	dominantWinner := ""
	dominantCount := 0
	for id, c := range counts {
		if c > dominantCount {
			dominantCount, dominantWinner = c, id
		}
	}

	// Если бы был atomic claim (например ZPOPMIN после выбора), то
	// "доминирование" >50% было бы невозможно при N>>1. Текущее поведение
	// — >80% горутин сходятся на одной ноде = double-booking гарантирован.
	if dominantCount < concurrentBuyers/2 {
		t.Fatalf(
			"unexpected: нет доминирующего выбора (%s: %d/%d) — возможно "+
				"уже внедрён atomic claim, тест нужно обновить",
			dominantWinner, dominantCount, concurrentBuyers,
		)
	}

	t.Errorf(
		"BUG B1: %d/%d одновременных buyer'ов выбрали одну и ту же ноду %q. "+
			"В CreateOfferAndMatch нет атомарного claim'а (ZREM/ZPOPMIN/SETNX) "+
			"после выбора bestNode. Оба получат JWT на одну и ту же ноду → "+
			"double-booking на Gateway. Требуется: Lua-скрипт ZPOPMIN + "+
			"SET lease:node:%%id EX 60 NX перед выдачей JWT.",
		dominantCount, concurrentBuyers, dominantWinner,
	)
}

// TestMatcher_NoBalanceHoldBeforeJWT (B3) — документирует, что CreateOfferAndMatch
// выдаёт JWT до холдирования баланса buyer'а. Это проверка "по отсутствию":
// тест читает исходник matcher.go и проверяет, что между buyer != nil guard'ом
// и buyerToken.SignedString НЕТ вызова функций резервирования баланса.
//
// Это полу-синтетический тест: Go-код не умеет reflection над AST без
// дополнительных зависимостей, поэтому мы ограничимся проверкой отсутствия
// ключевых слов в строке сорсника.
func TestMatcher_NoBalanceHoldBeforeJWT(t *testing.T) {
	// Читаем соседний файл matcher.go в той же директории.
	// (мы в package handlers, значит файл рядом)
	src, err := readLocalFile("matcher.go")
	if err != nil {
		t.Skipf("cannot read matcher.go: %v", err)
		return
	}

	// Ключевые слова, которые ДОЛЖНЫ присутствовать если hold реализован:
	// "HoldBalance", "ReserveBalance", "balance_usd - ", "FOR UPDATE"
	keywords := []string{"HoldBalance", "ReserveBalance", "balance_hold", "FOR UPDATE"}
	found := false
	for _, kw := range keywords {
		if containsBytes(src, []byte(kw)) {
			found = true
			t.Logf("found balance-hold keyword: %q", kw)
			break
		}
	}

	if !found {
		t.Errorf(
			"BUG B3: matcher.go не содержит признаков balance-hold перед " +
				"выдачей Gateway JWT. Злонамеренный buyer может сжечь " +
				"N тысяч JWT за секунду, ноды начнут обслуживание, затем " +
				"FinalizeSession упадёт на ErrInsufficientBuyerBalance и " +
				"нода останется без оплаты. Требуется: SELECT ... FOR UPDATE " +
				"+ atomic UPDATE buyers SET balance_usd = balance_usd - hold " +
				"ПЕРЕД шагом 5 (JWT generation) в CreateOfferAndMatch.",
		)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

func readLocalFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Ниже — дешёвые helpers без внешних зависимостей.
// (os.ReadFile + bytes.Contains эквиваленты, но изолированы для наглядности)
func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

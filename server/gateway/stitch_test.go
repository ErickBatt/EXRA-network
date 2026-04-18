package main

// stitch_test.go — доказательные edge-case тесты для Data Plane.
//
// ФИНДИНГИ, КОТОРЫЕ ЭТИ ТЕСТЫ ДОКУМЕНТИРУЮТ:
//
//   A1. Race в Stitch: вторая сторона может получить уже-закрытый conn первой
//       стороны, а её собственный conn утекает в буферизованный канал.
//   A2. Role-collision оставляет pending waiter → первая сторона висит 30s,
//       утечка памяти умножается при намеренном spam-е одноролевых коннектов.
//   A3. Bridge не ставит IO-deadline → slowloris-style DoS, горутины и буферы
//       утекают на каждое «застрявшее» соединение.
//
// ВСЕ ТРИ ТЕСТА СПЕЦИАЛЬНО НАПИСАНЫ ТАК, ЧТОБЫ ПАДАТЬ НА ТЕКУЩЕМ КОДЕ И
// ПРОХОДИТЬ ПОСЛЕ ФИКСА. Используется только std lib + net.Pipe.

import (
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStitch_RoleCollision_LeavesFirstPartyStranded (A2)
//
// Когда вторая сторона приходит с тем же ролью что и первая, она должна быть
// отвергнута, А ПЕРВАЯ — НЕ ДОЛЖНА продолжать висеть на 30-секундном таймауте.
// Текущая реализация оставляет первую сторону в select, её conn держится и
// занимает fd. На продакшн-нагрузке 10k спам-коннектов = 10k горутин × 30s.
func TestStitch_RoleCollision_LeavesFirstPartyStranded(t *testing.T) {
	sessionID := "sess-role-collision-" + t.Name()

	firstConn, _ := net.Pipe()
	secondConn, _ := net.Pipe()
	defer firstConn.Close()
	defer secondConn.Close()

	var firstReturned atomic.Bool
	firstDone := make(chan struct{})

	// First party — buyer
	go func() {
		defer close(firstDone)
		_, _ = Stitch(sessionID, "buyer", firstConn)
		firstReturned.Store(true)
	}()

	// Дать первой стороне войти в select.
	time.Sleep(50 * time.Millisecond)

	// Second party — тоже buyer (collision)
	_, ok := Stitch(sessionID, "buyer", secondConn)
	if ok {
		t.Fatalf("expected role collision to reject second party")
	}

	// Даём системе 300 мс — если фикс корректен, первая сторона тоже вышла.
	time.Sleep(300 * time.Millisecond)

	if !firstReturned.Load() {
		t.Errorf(
			"BUG A2: первая сторона продолжает висеть после role-collision. "+
				"Ожидалось: при role-collision waiter должен быть очищен и "+
				"первая сторона отпущена. Реально: первая сторона будет висеть "+
				"до 30-секундного ctx.Done. sessionID=%s",
			sessionID,
		)
	}
}

// TestStitch_LateSecondParty_CreatesOrphanWaiter (A1-вариант)
//
// Когда первая сторона уже вышла по таймауту и удалила waiter, вторая сторона,
// пришедшая позже, НЕ ДОЛЖНА создавать новый orphan-waiter. Текущая логика
// через `LoadOrStore` ломается: второй вызов видит пустую карту, сам становится
// «первой стороной» и висит ещё 30 секунд.
//
// Тест симулирует ситуацию без реального ожидания 30s: мы явно вызываем Stitch,
// удаляем из карты (имитация произошедшего таймаута), затем делаем второй
// вызов и проверяем, что он быстро завершается, а не встаёт в ожидание.
func TestStitch_LateSecondParty_CreatesOrphanWaiter(t *testing.T) {
	sessionID := "sess-orphan-" + t.Name()

	// Шаг 1. Имитируем уже истёкшую первую сторону — её waiter был удалён.
	pendingSessions.Delete(sessionID)

	secondConn, _ := net.Pipe()
	defer secondConn.Close()

	type result struct {
		conn net.Conn
		ok   bool
	}
	resCh := make(chan result, 1)

	go func() {
		c, ok := Stitch(sessionID, "node", secondConn)
		resCh <- result{c, ok}
	}()

	select {
	case r := <-resCh:
		if r.ok {
			t.Fatalf("expected second-late party to NOT succeed with no counterpart")
		}
		// Если вышел быстро с ok=false — это то поведение, которого мы хотим
		// ПОСЛЕ фикса (fast-fail). На текущем коде этот case вообще не случается
		// в разумное тестовое окно, тест упадёт по таймауту ниже.
	case <-time.After(500 * time.Millisecond):
		// Очистим leak, чтобы не засорять тестовый процесс
		pendingSessions.Delete(sessionID)
		t.Errorf(
			"BUG A1: late-arriving second party создал orphan waiter и "+
				"висит в select 30s. Ожидалось: Stitch должен видеть, что "+
				"контрагент не придёт, и быстро fail-ить (допустим, через "+
				"короткий TTL для 'второй стороны' или через явный флаг "+
				"создания только первой стороной). sessionID=%s",
			sessionID,
		)
	}
}

// TestBridge_NoReadDeadlineHangsForever (A3)
//
// Bridge должен иметь IO-deadline на каждую итерацию relay(), иначе «тихая»
// нода может вечно держать два goroutines + два 32KB буфера из sync.Pool.
//
// Тест открывает две net.Pipe, запускает Bridge, и проверяет что через 2s
// при отсутствии трафика горутины всё ещё живы — это доказывает отсутствие
// таймаута. Также считает количество горутин, чтобы продемонстрировать leak.
func TestBridge_NoReadDeadlineHangsForever(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	// Pipe A ↔ Pipe B — симулируем «застывший» коннект.
	aServer, aClient := net.Pipe()
	bServer, bClient := net.Pipe()

	defer aClient.Close()
	defer bClient.Close()

	startGoroutines := runtime.NumGoroutine()

	bridgeDone := make(chan struct{})
	go func() {
		Bridge(aServer, bServer)
		close(bridgeDone)
	}()

	// Даём Bridge войти в relay() × 2 goroutines.
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after-startGoroutines < 2 {
		t.Fatalf("Bridge не стартовал 2 relay-горутины (was %d, now %d)",
			startGoroutines, after)
	}

	// Ждём 2 секунды БЕЗ какого-либо трафика в обе стороны.
	// Если бы был SetReadDeadline(90s) — тест не менялся бы, но в продакшене
	// deadline типа 90s защищает. Главное: sending NOTHING не должно держать
	// goroutines вечно. Ставим более короткий тест — 2s, чтобы сигнализировать
	// об отсутствии механизма liveness.
	select {
	case <-bridgeDone:
		t.Logf("Bridge корректно вышел при idle → хорошо")
		return
	case <-time.After(2 * time.Second):
		// Принудительно гасим, иначе тестовый процесс утечёт соединения
		aServer.Close()
		bServer.Close()
		<-bridgeDone

		t.Errorf(
			"BUG A3: Bridge простоял 2 секунды без трафика и без liveness-" +
				"проверки. В продакшене это означает что slowloris-клиент " +
				"может держать соединение бесконечно. Требуется: SetReadDeadline " +
				"на каждую итерацию relay() + WS-ping/pong heartbeat.",
		)
	}
}

// TestBridge_ClosesBothSidesOnOneSideEOF — положительная проверка (смоук).
// Демонстрирует, что если один из peer'ов закрыл коннект, Bridge нормально
// завершается. Это baseline-поведение, которое не должно сломаться после
// добавления deadlines.
func TestBridge_ClosesBothSidesOnOneSideEOF(t *testing.T) {
	aServer, aClient := net.Pipe()
	bServer, bClient := net.Pipe()

	defer aClient.Close()
	defer bClient.Close()

	done := make(chan struct{})
	go func() {
		Bridge(aServer, bServer)
		close(done)
	}()

	// Симулируем FIN от одной из сторон.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Читаем и моментально получим EOF после закрытия.
		io.Copy(io.Discard, aClient)
	}()

	time.Sleep(50 * time.Millisecond)
	bClient.Close()

	select {
	case <-done:
		// OK — Bridge вышел
	case <-time.After(2 * time.Second):
		t.Fatalf("Bridge не завершился после закрытия одной стороны")
	}
	wg.Wait()
}

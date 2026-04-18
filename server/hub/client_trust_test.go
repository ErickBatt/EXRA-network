package hub

// client_trust_test.go — доказательные тесты для находок E1/E2/E3.
//
// ФИНДИНГИ:
//
//   E1. case "feeder_report" в ReadPump вызывает models.RecordFeederReport
//       БЕЗ проверки подписи (msg.Signature / msg.Data.Signature). Любой
//       зарегистрированный воркер может отправить verdict="fail" на любого
//       соседа и нанести тому слэшинг.
//
//   E2. Canary hash — жёсткая константа "canary_expected_hash" в models/fraud.go
//       (строка 319). Один и тот же expected_result для ВСЕХ устройств и ВСЕХ
//       задач → нечестный воркер, знающий исходник (или догадавшийся)
//       всегда проходит canary-проверку.
//
//   E3. case "traffic" принимает msg.Bytes от воркера без серверной
//       верификации (counter-party cross-check со стороны buyer/feeder).
//       Воркер сам себе подсчитывает объём — тривиальный inflate для
//       накрутки наград.

import (
	"exra/db"
	"exra/models"
	"os"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestCanary_HardcodedHashIsUniversal (E2)
//
// Демонстрирует, что константа "canary_expected_hash" — едина для ВСЕХ устройств
// и ВСЕХ задач, и воркер, знающий её, проходит проверку тривиально.
//
// Тест моделирует двух разных воркеров с разными deviceID, двумя разными
// canary-задачами — и показывает, что оба передают VerifyCanaryResult с ОДНОЙ
// и той же magic-string.
func TestCanary_HardcodedHashIsUniversal(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock open: %v", err)
	}
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	const (
		magicHash = "canary_expected_hash" // ← утечка константы из models/fraud.go:319
		deviceA   = "honest-worker-alpha"
		deviceB   = "dishonest-worker-bravo"
	)

	// Эмулируем, что Oracle выдал разные canary-tasks двум разным нодам.
	// SELECT expected_result FROM canary_tasks WHERE id=1 AND device_id=deviceA
	mock.ExpectQuery("SELECT expected_result FROM canary_tasks").
		WithArgs(int64(1), deviceA).
		WillReturnRows(sqlmock.NewRows([]string{"expected_result"}).AddRow(magicHash))
	mock.ExpectExec("UPDATE canary_tasks").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// SELECT expected_result FROM canary_tasks WHERE id=2 AND device_id=deviceB
	mock.ExpectQuery("SELECT expected_result FROM canary_tasks").
		WithArgs(int64(2), deviceB).
		WillReturnRows(sqlmock.NewRows([]string{"expected_result"}).AddRow(magicHash))
	mock.ExpectExec("UPDATE canary_tasks").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Воркер A честно прошёл (он ничего не знает, угадал правильно).
	passA := models.VerifyCanaryResult(deviceA, 1, magicHash)
	// Воркер B не выполнял задачу, НО прочитал исходник / прошлый трейс и
	// знает magicHash. Он отправляет ту же строку — и тоже проходит.
	passB := models.VerifyCanaryResult(deviceB, 2, magicHash)

	if !passA || !passB {
		t.Fatalf("sanity failed: passA=%v passB=%v", passA, passB)
	}

	// Главное утверждение теста: magicHash — универсальный токен, не зависящий
	// от deviceID или taskID. Это и есть уязвимость.
	t.Errorf(
		"BUG E2: canary expected_result = \"canary_expected_hash\" — жёсткая "+
			"константа для ВСЕХ устройств (проверено на %s и %s с разными "+
			"taskID). Dishonest worker, знающий строку, проходит проверку "+
			"100%% случаев. Требуется: per-task случайный expected_result, "+
			"вычисляемый Oracle по формуле sha256(secret_nonce || task_payload), "+
			"где worker ДОЛЖЕН реально выполнить прокси-запрос чтобы получить hash.",
		deviceA, deviceB,
	)
}

// TestCanary_ConstantIsLiteralInSource (E2 corroborating)
//
// Дополнительное доказательство: regex-поиск в сорсе models/fraud.go
// показывает, что expected_result действительно литерал, а не вычисляется
// из per-task nonce.
func TestCanary_ConstantIsLiteralInSource(t *testing.T) {
	src, err := os.ReadFile("../models/fraud.go")
	if err != nil {
		t.Skipf("cannot read fraud.go: %v", err)
	}
	// Ищем литерал "canary_expected_hash".
	re := regexp.MustCompile(`expectedResult\s*:=\s*"canary_expected_hash"`)
	if !re.Match(src) {
		t.Skip("не нашли expected литерал — возможно фикс уже применён")
	}
	t.Errorf(
		"BUG E2 corroborated: в models/fraud.go найден литерал " +
			"expectedResult := \"canary_expected_hash\". " +
			"Это prove-by-source что строка НЕ вычисляется per-task.",
	)
}

// TestFeederReport_NoSignatureCheckInSource (E1)
//
// ReadPump обрабатывает case "feeder_report" строками 306-314 client.go:
//
//   case "feeder_report":
//     if c.DeviceID != "" && msg.AssignmentID > 0 && msg.Verdict != "" {
//       log.Printf(...)
//       models.RecordFeederReport(msg.AssignmentID, c.DeviceID, msg.DeviceID,
//                                  msg.Verdict, 0, 0)
//     }
//
// Отсутствует вызов VerifyDIDSignature. Для сравнения — case "heartbeat"
// и case "register" ДЕЛАЮТ проверку. Этот тест парсит сорсник и доказывает
// расхождение.
func TestFeederReport_NoSignatureCheckInSource(t *testing.T) {
	src, err := os.ReadFile("client.go")
	if err != nil {
		t.Fatalf("cannot read client.go: %v", err)
	}

	// Извлекаем весь case "feeder_report" блок — от его начала до следующего
	// "case " или закрывающей фигурной скобки switch.
	re := regexp.MustCompile(`(?s)case "feeder_report":(.*?)(?:case "|[\t ]+}\s*}\s*$)`)
	matches := re.FindSubmatch(src)
	if matches == nil {
		t.Fatal("не нашёл case \"feeder_report\" в client.go — возможно переименован")
	}
	caseBody := string(matches[1])

	// Проверяем, что внутри case нет ни одного вызова VerifyDIDSignature.
	if regexp.MustCompile(`VerifyDIDSignature`).MatchString(caseBody) {
		t.Skipf("VerifyDIDSignature уже присутствует в feeder_report branch — фикс применён")
	}

	// И логически перекрестно: в case "heartbeat" она ЕСТЬ — значит расхождение.
	heartbeatRe := regexp.MustCompile(`(?s)case "heartbeat":.*?VerifyDIDSignature`)
	if !heartbeatRe.Match(src) {
		t.Fatal("sanity: в case \"heartbeat\" тоже нет VerifyDIDSignature — смотри модель, что-то изменилось")
	}

	t.Error(
		"BUG E1: case \"feeder_report\" в client.go ПРИНИМАЕТ verdict без " +
			"подписи. Для сравнения, case \"heartbeat\" — проверяет. " +
			"Любой воркер может отправить feeder_report с чужим " +
			"msg.DeviceID как target, поле verdict=\"fail\" → RecordFeederReport " +
			"ударит ни в чём не повинного соседа. При N-кратной атаке — " +
			"Sybil-ring из 2/3 ложных feeder'ов получит ложный консенсус " +
			"и вгонит target в слэшинг. " +
			"Требуется: msg.Data.Signature обязательна, " +
			"signMsg := sprintf(assignmentID:target:verdict), " +
			"VerifyDIDSignature(c.PubKey, signMsg, msg.Data.Signature) перед " +
			"RecordFeederReport.",
	)
}

// TestTrafficReport_NoServerSideCrossCheck (E3)
//
// case "traffic" (строки 251-256) доверяет msg.Bytes безусловно. Нет
// сверки с buyer-side counter'ом, нет deadline'а, нет max-rate предела.
// Воркер-читер умножает Bytes на 10 → получает 10× награду.
func TestTrafficReport_NoServerSideCrossCheck(t *testing.T) {
	src, err := os.ReadFile("client.go")
	if err != nil {
		t.Fatalf("cannot read client.go: %v", err)
	}

	// Извлекаем case "traffic" блок.
	re := regexp.MustCompile(`(?s)case "traffic":(.*?)case "`)
	matches := re.FindSubmatch(src)
	if matches == nil {
		t.Fatal("не нашёл case \"traffic\" в client.go")
	}
	caseBody := string(matches[1])

	// Ключевые маркеры, указывающие на кросс-валидацию:
	markers := []string{
		"VerifyTraffic",    // гипотетический метод проверки
		"buyer_reported",   // cross-check с buyer counter
		"MaxTrafficPerSec", // rate-limit
		"expected_bytes",   // проверка ожидаемого объёма
	}
	for _, m := range markers {
		if regexp.MustCompile(m).MatchString(caseBody) {
			t.Skipf("найден признак cross-check: %q — фикс, возможно, применён", m)
		}
	}

	t.Errorf(
		"BUG E3: case \"traffic\" принимает msg.Bytes от воркера без " +
			"серверной верификации. В случае враждебной ноды: " +
			"inflatedBytes = actualBytes * 10 → AddNodeTrafficByDeviceID " +
			"записывает ложь → reward pipeline считает по ложным цифрам. " +
			"Требуется: (а) получать buyer-side counter через session_id, " +
			"(б) сверять их в models/session.go при FinalizeSession и " +
			"отклонять при разногласии >5%%, (в) штрафовать воркера через " +
			"AddStrike(deviceID, \"traffic_mismatch\") при повторном " +
			"несоответствии.",
	)
}

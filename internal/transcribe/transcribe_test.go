package transcribe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"
)

// referenceTranscript — эталон транскрипта для видео sV8zOFf7QWs (Cryptodex, 257 сек).
const referenceTranscript = `Всем привет, в этом видео хочу рассмотреть интересную тему касаемой стратегии, когда вы только начинаете торговать, вы не знаете какую стратегию использовать, скорее всего ищите их в интернете, возможно смотрите у блогеров и так далее, но когда вы используете эту стратегию на своем опыте, на своих деньгах, она почему-то не работает. И если это про тебя, то обязательно смотри это видео до конца, я расскажу тебе 5 шагов постановить свои собственные торговые стратегии на примере своего опыта, а вы понимая принциписть и сможете уже создать свою собственную. Меня зовут Олег, вы на канале Cryptodex, давайте начинать. Вообще хотелось начать с того, для чего нужна стратегия. Многие ищут в этом какую-то кнопку в облому, типа если я буду использовать этот индикатор, то сделки 100% будут плюс. Нет, стратегия это в первую очередь дисциплина, потому что без нее трейдинг превращается в азартную игру ради получения адреналина, а не прибыли. И поэтому для того, чтобы не играть в казино каждый раз при открытии сделки, давайте присунусь к созданию стратегии. Шаг первый. Выбираем стиль торговли. Трейдинг может быть долгосрочным, среднесрочным, краткосрочным или там внутридневным скальпинг и так далее и например лично мне по началу было тяжело засыпать когда открыта мне была сделка то есть я не мог оторваться от телефона я пытался уснуть но каждый раз все равно проверял закрылась моя позиция там в плюс в минус и так далее и чаще всего я даже просто закрывал позицию раньше да для того чтобы просто заснуть а по итогу да если бы я на время подождал еще пару дней она бы отработала как нужно поэтому мне было гораздо комфортно психологически траговать внутри дня я для себя выбрал внутридневной торговлю от этого зависит, какой тайн-фрейм вы будете использовать. Для меня это были 5 минутки, часовики, 15 минутки, да и то, что в принципе меньше часа. Шаг второй. Подбираем технические индикаторы. Самым главным обязательным условием является понимание принципа работы индикатора. То есть вы должны четко понимать, что он показывает, откуда берется, а что означает и так далее. И очень часто начинают трейдеры допускать такую ошибку, то что используют стратегии огромное количество индикаторов, то есть одного типа, и аргументируют это тем, что совпадение там их показаний дает четкий сигнал для входа в рынок. И график превращается в нечитабельную берегерду, а сигналы даже могут перебивать друг друга. Поэтому необходимо использовать индикаторы, которые не повторяют, а дополняют друг друга. Например, трендовый индикатор скользящая средняя и осциллятор МакДи. А я в своей стратегии использовал инструменты коррекции Фибоначчи и расширения Фибоначчи. Шаг третий. Выбираем торговые пары. На самом деле каждая криптовалюта обладает своим характером движения и это важно учитывать. Например, если вы разрабатываете стратегию для скальпинга, вам не подойдут торговые пары с большим спредом, который будет съедать практически всю прибыль, или вы торгуете, например, пробой уровня, вам необходимо волатильность, и то, что уровни действительно хорошо пробиваются, да, это на истории графика тоже будет видно. Поэтому выбирайте пару, исходя из требуемых вам условий, и проверяйте их на истории и практике. То есть, чтобы вы понимали, прежде чем я определился со списком криптолюд, на которых я торгую, и эта стратегия отрабатывает, я совершил сотни сделок, среди которых были и много убыточных в том числе. И с каждой открытой сделкой одной за одной я понимал какие условия мне необходимы для того, чтобы эта стратегия хорошо отрабатывала. Так что не надо гуглить там какую-то стратегию, заходить на рандомную торговую пару, первую попавшуюся, открывать пару сделок, не сработало, а все, пойду искать новую и так далее. Это порочный круг, вы так никогда не найдете свою собственную стратегию. А просто постоянно будете сливать бюджетные тесты, причем некачественные. Шаг четвертый. Обделяем способы фиксации прибыли и и убытках. Как по мне это было самым сложным потому что стратегия это набор таких правил которые работать должны как и в ручном режиме так и в автоматическом и поэтому все точки фиксации прибыли либо убытков должны были сразу быть отмечены при открытии сделки и самое главное они должны иметь какую-то логику да они основаны быть на вашей интуиции естественно способа фиксации достаточно большое количество да и вы можете подобрать абсолютно любое которое вам комфортно для меня это было отношение риска прибыли да минимум один к трем то есть если я рисковал потерять 10 долларов, то должен был заработать 30 долларов. Шаг 5. Выбираем биржу для торговли. Для тех, кто собирается торговать на постоянной основе, очень важно выбрать биржу, с которой будет вам максимально комфортно взаимодействовать по разным причинам. Это может быть надежность, безопасность, интерфейс и так далее. Я в основном торгую на бирже Binance, на бирже Bybit и если вы хотите получить скидки на торговые комиссии, то регистрируйтесь по ссылкам в описании и экономке. Ну а у меня на этом все, спасибо, что посмотрел это видео до конца, подписывайся на канал, ставь лайк, если это видео было тебе полезным и до новых встреч.`

func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteRune(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func wordSet(s string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, w := range strings.Fields(s) {
		m[w] = struct{}{}
	}
	return m
}

func jaccard(a, b string) float64 {
	sa, sb := wordSet(a), wordSet(b)
	if len(sa) == 0 && len(sb) == 0 {
		return 1.0
	}
	inter := 0
	for w := range sa {
		if _, ok := sb[w]; ok {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 1.0
	}
	return float64(inter) / float64(union)
}

func containsWords(text string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

func TestSafeFileName(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"обычное название", "Полный курс по трейдингу", "Полный курс по трейдингу"},
		{"слеши заменяются", "Часть 1/2: Введение", "Часть 1_2_ Введение"},
		{"все спецсимволы", `a/b\c:d*e?f"g<h>i|j`, "a_b_c_d_e_f_g_h_i_j"},
		{"пробелы по краям обрезаются", "   Название   ", "Название"},
		{"пустое название → video", "", "video"},
		{"только пробелы → video", "   ", "video"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeFileName(tt.title); got != tt.want {
				t.Errorf("safeFileName(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestSafeFileName_LongTitle(t *testing.T) {
	title := strings.Repeat("а", 300)
	got := safeFileName(title)
	if len(got) != 200 {
		t.Errorf("safeFileName long title len = %d, want 200", len(got))
	}
}

// runDocker выполняет docker-команду с таймаутом (2 минуты).
func runDocker(t *testing.T, projectRoot string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("docker %v: таймаут 2 минуты истёк\n%s", args, string(out))
	}
	if err != nil {
		t.Fatalf("docker %v: %v\n%s", args, err, string(out))
	}
	return string(out)
}

// runDockerStop пытается остановить контейнер с таймаутом (1 минута).
func runDockerStop(t *testing.T, projectRoot string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "stop", "transcribe")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Logf("предупреждение: docker compose stop: таймаут 1 минута истёк\n%s", string(out))
		return
	}
	if err != nil {
		t.Logf("предупреждение: docker compose stop: %v\n%s", err, string(out))
	}
}

// TestWhisperAgainstReference — полностью автономный интеграционный тест.
func TestWhisperAgainstReference(t *testing.T) {
	projectRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	localAudio := filepath.Join(projectRoot, "test_audio.webm")

	if _, err := os.Stat(localAudio); err != nil {
		t.Fatalf("локальный файл %q не найден", localAudio)
	}

	const testTimeout = 25 * time.Minute
	deadline := time.Now().Add(testTimeout)

	// 0. Останавливаем контейнер
	t.Logf("[%s] Шаг 0/6: остановка контейнера transcribe (если запущен)...", time.Now().Format("15:04:05"))
	runDockerStop(t, projectRoot)

	// Ждем 10 секунд, чтобы контейнер точно освободил ресурсы
	t.Logf("[%s] Ожидание 10 секунд для освобождения ресурсов...", time.Now().Format("15:04:05"))
	time.Sleep(10 * time.Second)
	t.Logf("[%s] Контейнер остановлен, ресурсы освобождены", time.Now().Format("15:04:05"))

	// 1. Чистим очередь
	t.Logf("[%s] Шаг 1/6: очистка очереди в БД...", time.Now().Format("15:04:05"))
	sqlCmd := exec.Command("docker", "exec", "postgres", "psql", "-U", "peter", "-d", "youtube_tracker", "-c",
		"UPDATE videos SET is_processed=true WHERE is_processed=false;")
	if out, err := sqlCmd.CombinedOutput(); err != nil {
		t.Logf("предупреждение: не удалось очистить очередь: %v\n%s", err, string(out))
	} else {
		t.Logf("[%s] Очередь очищена", time.Now().Format("15:04:05"))
	}

	// 2. Поднимаем контейнер
	t.Logf("[%s] Шаг 2/6: запуск контейнера transcribe...", time.Now().Format("15:04:05"))
	runDocker(t, projectRoot, "compose", "up", "-d", "transcribe")
	t.Logf("[%s] Контейнер запущен", time.Now().Format("15:04:05"))

	// 3. Копируем аудио
	t.Logf("[%s] Шаг 3/6: копирование %s в контейнер...", time.Now().Format("15:04:05"), localAudio)
	containerAudio := "/tmp/test_audio.webm"
	runDocker(t, projectRoot, "cp", localAudio, "transcribe:"+containerAudio)
	t.Logf("[%s] Аудио скопировано", time.Now().Format("15:04:05"))

	// 4. Запускаем транскрибацию (с прогресс-логом)
	t.Logf("[%s] Шаг 4/6: запуск транскрибации (может занять 5-10 минут)...", time.Now().Format("15:04:05"))
	startTime := time.Now()

	type result struct {
		text string
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		txt, err := File(containerAudio)
		resCh <- result{txt, err}
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	var got string
	var err error
loop:
	for {
		select {
		case r := <-resCh:
			got = r.text
			err = r.err
			break loop
		case <-ticker.C:
			elapsed := time.Since(startTime).Round(time.Second)
			left := time.Until(deadline).Round(time.Second)
			t.Logf("[%s] ... процесс идёт, прошло %s, осталось до таймаута %s",
				time.Now().Format("15:04:05"), elapsed, left)
			if time.Now().After(deadline) {
				runDockerStop(t, projectRoot)
				t.Fatalf("превышен таймаут %s", testTimeout)
			}
		}
	}

	elapsed := time.Since(startTime).Round(time.Second)
	t.Logf("[%s] Шаг 4/6 завершён за %s", time.Now().Format("15:04:05"), elapsed)

	if err != nil {
		runDockerStop(t, projectRoot)
		t.Fatalf("ошибка транскрибации: %v", err)
	}

	// 5. Сравнение с эталоном
	t.Logf("[%s] Шаг 5/6: сравнение с эталоном...", time.Now().Format("15:04:05"))
	gotN := normalize(got)
	refN := normalize(referenceTranscript)
	if gotN == "" {
		runDockerStop(t, projectRoot)
		t.Fatal("транскрипт пустой")
	}

	refWords := len(strings.Fields(refN))
	gotWords := len(strings.Fields(gotN))
	ratio := float64(gotWords) / float64(refWords)
	j := jaccard(refN, gotN)

	// Выводим тексты (в файл через -v > log)
	t.Logf("=== НОВЫЙ ТЕКСТ ===\n%s\n", got)
	t.Logf("=== ЭТАЛОН ===\n%s\n", referenceTranscript)

	// 6. Останавливаем контейнер
	t.Logf("[%s] Шаг 6/6: остановка контейнера...", time.Now().Format("15:04:05"))
	runDockerStop(t, projectRoot)

	// Итог
	t.Logf("=== РЕЗУЛЬТАТ ===")
	t.Logf("Длина: got %d слов, ref %d слов (ratio %.2f, норма 0.85–1.15)", gotWords, refWords, ratio)
	t.Logf("Jaccard: %.3f (норма >= 0.90)", j)
	t.Logf("Время транскрибации: %s", elapsed)

	if ratio < 0.85 || ratio > 1.15 {
		t.Errorf("длина отклоняется: got %d, ref %d (ratio %.2f)", gotWords, refWords, ratio)
	}
	if j < 0.90 {
		t.Errorf("jaccard слишком низкий: %.3f (< 0.90)", j)
	}

	refWordsList := strings.Fields(refN)
	n := 6
	if len(refWordsList) < n {
		n = len(refWordsList)
	}
	if !containsWords(gotN, refWordsList[:n]) {
		t.Errorf("в начале нет фразы %v", refWordsList[:n])
	}
	if !containsWords(gotN, refWordsList[len(refWordsList)-n:]) {
		t.Errorf("в конце нет фразы %v", refWordsList[len(refWordsList)-n:])
	}

	if t.Failed() {
		t.Logf("[%s] ТЕСТ ПРОВАЛЕН", time.Now().Format("15:04:05"))
	} else {
		t.Logf("[%s] ТЕСТ ПРОЙДЕН", time.Now().Format("15:04:05"))
	}
}
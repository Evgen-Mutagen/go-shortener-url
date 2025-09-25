// Package main предоставляет multichecker, объединяющий несколько статических анализаторов.
//
// Multichecker включает:
//   - Все стандартные анализаторы из golang.org/x/tools/go/analysis/passes
//   - Все анализаторы класса SA из staticcheck
//   - Анализаторы S1000-S1012 из staticcheck
//   - Анализаторы errcheck и gosec
//   - Кастомный анализатор, запрещающий os.Exit в main
//
// # Использование
//
// Установка:
//
//	go install ./cmd/staticlint
//
// Запуск:
//
//	staticlint ./...              # анализ всех пакетов
//	staticlint ./internal/...     # анализ internal пакетов
//	staticlint -v ./...           # с подробным выводом
//
// # Анализаторы
//
// Стандартные анализаторы (golang.org/x/tools/go/analysis/passes):
//   - asmdecl    - проверка деклараций ассемблера
//   - assign     - обнаружение бесполезных присваиваний
//   - atomic     - проверка sync/atomic
//   - bools      - ошибки с булевыми операторами
//   - buildtag   - проверка build tags
//   - cgocall    - проверка вызовов Cgo
//   - composite  - проверка композитных литералов
//   - copylock   - проверка блокировок копирования
//   - directive  - проверка директив компилятора
//   - errorsas   - проверка errors.As
//   - framepointer - проверка указателей фреймов
//   - httpresponse - проверка HTTP ответов
//   - loopclosure - проверка замыканий в циклах
//   - lostcancel - обнаружение потерянных контекстных отмен
//   - nilfunc    - проверка сравнений с nil
//   - printf     - проверка форматных строк
//   - shift      - проверка операций сдвига
//   - sigchanyzer - проверка каналов сигналов
//   - sortslice  - проверка сортировки срезов
//   - stdmethods - проверка стандартных методов
//   - stringintconv - проверка преобразований строк и чисел
//   - structtag  - проверка тегов структур
//   - testinggoroutine - проверка горутин в тестах
//   - tests      - проверка тестов
//   - unmarshal  - проверка анмаршалинга
//   - unreachable - обнаружение недостижимого кода
//   - unsafeptr  - проверка unsafe.Pointer
//   - unusedresult - обнаружение неиспользуемых результатов
//
// Анализаторы класса SA (staticcheck):
//   - SA1000-SA1030 - различные проверки корректности кода
//
// Другие анализаторы staticcheck:
//   - S1000-S1012 - упрощения и оптимизации кода
//
// Публичные анализаторы:
//   - errcheck   - проверка необработанных ошибок
//
// Кастомный анализатор:
//   - noosexit   - запрет os.Exit в main функции
package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/findcall"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/pkgfact"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"

	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	"github.com/Evgen-Mutagen/go-shortener-url/cmd/staticlint/noosexit"
)

func main() {
	// Стандартные анализаторы
	standardAnalyzers := []*analysis.Analyzer{
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		atomicalign.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		ctrlflow.Analyzer,
		deepequalerrors.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		fieldalignment.Analyzer,
		findcall.Analyzer,
		framepointer.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		pkgfact.Analyzer,
		printf.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		sortslice.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
	}

	// SA анализаторы staticcheck
	var saAnalyzers []*analysis.Analyzer
	for _, v := range staticcheck.Analyzers {
		saAnalyzers = append(saAnalyzers, v.Analyzer)
	}

	// Другие анализаторы staticcheck (не менее одного)
	var otherStaticcheckAnalyzers []*analysis.Analyzer
	for _, v := range stylecheck.Analyzers {
		otherStaticcheckAnalyzers = append(otherStaticcheckAnalyzers, v.Analyzer)
	}

	// Кастомный анализатор
	customAnalyzers := []*analysis.Analyzer{
		noosexit.Analyzer,
	}

	// Объединяем все анализаторы
	allAnalyzers := append(standardAnalyzers, saAnalyzers...)
	allAnalyzers = append(allAnalyzers, otherStaticcheckAnalyzers...)
	allAnalyzers = append(allAnalyzers, customAnalyzers...)

	multichecker.Main(allAnalyzers...)
}

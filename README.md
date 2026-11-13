# dstv_parser

Go-библиотека для чтения файлов DSTV/NC1. Она извлекает данные детали и продольный габарит, которые может использовать отдельный проект раскроя. Оптимизацию размещения деталей в хлыстах эта библиотека не выполняет.

## Установка

```sh
go get github.com/thearnkey/dstv_parser
```

## Использование

```go
import dstv "github.com/thearnkey/dstv_parser"

part, err := dstv.ParseFile("beam.nc1")
if err != nil {
    return err
}

cut := part.Cutting()
if !cut.Ready {
    // Не передавать длину в автоматический раскрой без проверки причин.
    fmt.Println(cut.Reasons)
}
fmt.Println(part.Header.Piece, part.Header.Profile, part.Header.Quantity, cut.LengthMM)
```

Все линейные размеры указаны в миллиметрах, углы — в градусах. `Header` содержит профиль, код профиля, марку стали, количество, длину, отдельную пильную длину, размеры сечения и четыре угла косых резов. Коды профилей сохраняются без ограничивающего перечня — это позволяет читать нестандартные и пользовательские профили.

`ExternalContours` и `InternalContours` содержат точки блоков `AK` и `IK`: грань, координаты, радиус и исходные данные подготовки кромки. `SawCuts` содержит точки и нормали плоскостей `SC`. Исходные строки всех блоков, включая неизвестные, остаются в `Blocks`; комментарии — в `Comments`.

`Cutting()` возвращает длину для следующего этапа и признак `Ready`. Приоритет источника длины: явная пильная длина, затем длина заголовка; больший продольный размах внешнего контура поднимает результат. Для дуг учитываются экстремумы по X. При неоднозначной геометрии, подготовке кромки, резе `SC` или косом резе без явной пильной длины `Ready` становится `false`, а причины перечисляются в `Reasons`. Значение `LengthMM` при `Ready=false` нельзя автоматически считать достаточной длиной заготовки.

## Команда для просмотра файла

```sh
go run ./cmd/dstv_parser beam.nc1
```

Команда выводит JSON с заголовком, блоками, разобранными резами и оценкой длины. Она не делает раскрой.

## Проверка и границы

```sh
go test ./...
go vet ./...
```

Парсер принимает один `ST` и завершающий `EN`, минимум 20 полей заголовка, до четырёх строк описания, LF и CRLF. В строке длины поддержаны значения через пробел или запятую. Ошибки обязательных полей возвращаются с номером строки. Неизвестные блоки сохраняются.

Тесты включают файлы из [DSTV.Net](https://github.com/Baseflow/DSTV.Net) и [tluijken/dstv](https://github.com/tluijken/dstv); сведения об их лицензиях — в [testdata/THIRD_PARTY_NOTICES.md](testdata/THIRD_PARTY_NOTICES.md). Дополнительно библиотека проверена на примерах [serg-you-lin/dstvparser](https://github.com/serg-you-lin/dstvparser). Эти файлы полезны для совместимости, но не заменяют проверку на NC1, выданных вашим ПО и станком.

Правила формата: [описание DSTV](https://archiv.bauforumstahl.de/fileadmin/user_upload/bauforumstahl.de/wissen/normen-und-richtlinien/bfs-richtlinien/109_Standartbeschreibung_von_Stahlbau-Teilen_8.0.pdf), [описание Tekla](https://support.tekla.com/doc/tekla-structures/2025/int_nc_dstv_file_description).

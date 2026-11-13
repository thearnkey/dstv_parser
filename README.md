# dstv_parser

Go-пакет и консольная команда для чтения файлов DSTV/NC1. Одна запись `Part` соответствует одной детали.

## Установка

```sh
go get github.com/thearnkey/dstv_parser
```

Для запуска команды без установки клонируйте репозиторий и выполните `go run ./cmd/dstv_parser part.nc1` из его каталога. Команда выводит разобранный файл в JSON.

## Использование

Для линейного раскроя доступны `Header.Profile`, `Header.ProfileCode`, `Header.SteelGrade`, `Header.Quantity` и `Header.StockLength()` (мм). `StockLength()` выбирает отдельную пильную длину, если она указана после обычной длины в строке заголовка. При расчёте заготовок отдельно учитывайте пропил, припуск, зажим и ограничения станка.

Блоки геометрии (`BO`, `AK`, `IK`, `SC` и другие) сохраняются как исходные строки с номерами. Их координаты пока не интерпретируются. Не используйте этот пакет как валидатор программы станка или как расчёт габарита сложного контура.

```go
import dstv_parser "github.com/thearnkey/dstv_parser"

part, err := dstv_parser.ParseFile("part.nc1")
if err != nil { /* обработать ошибку */ }
fmt.Println(part.Header.Profile, part.Header.Quantity, part.Header.StockLength())
```

## Границы поддержки

Вход: один `ST` и завершающий `EN`, минимум 20 полей заголовка по порядку DSTV, до четырёх дополнительных строк описания. Поддерживаются LF и CRLF. Неизвестные двухсимвольные блоки сохраняются. Ошибки структуры и обязательных числовых полей возвращаются с номером строки.

Формат полей и блоков: [NC-DSTV Interface](https://advancedrobotic.com/admin/FTP/NC-DSTV-Interface.pdf), [описание Tekla](https://support.tekla.com/doc/tekla-structures/2025/int_nc_dstv_file_description).

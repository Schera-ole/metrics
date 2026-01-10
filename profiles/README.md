# Результаты профилирования памяти

## Общая информация

После оптимизации кода в проекте были получены следующие результаты профилирования памяти с помощью pprof:

```
Showing nodes accounting for -17267.15MB, 97.60% of 17691.50MB total
Dropped 143 nodes (cum <= 88.46MB)
      flat  flat%   sum%        cum   cum%
-9311.44MB 52.63% 52.63% -16796.90MB 94.94%  compress/flate.NewWriter (inline)
-4534.87MB 25.63% 78.27% -7485.46MB 42.31%  compress/flate.(*compressor).init
-2893.54MB 16.36% 94.62% -2893.54MB 16.36%  compress/flate.newDeflateFast (inline)
 -388.39MB  2.20% 96.82%  -388.39MB  2.20%  compress/flate.(*dictDecoder).init (inline)
  -95.86MB  0.54% 97.36%  -484.25MB  2.74%  compress/flate.NewReader
  -35.54MB   0.2% 97.56%  -662.42MB  3.74%  github.com/Schera-ole/metrics/internal/handler.BatchUpdateHandler
      -6MB 0.034% 97.59%  -490.26MB  2.77%  compress/gzip.NewReader (inline)
   -1.50MB 0.0085% 97.60% -17501.83MB 98.93%  github.com/Schera-ole/metrics/internal/handler.Router.LoggingMiddleware.func8.1
```
Данные снимались с помощью команд:
curl -sK -v http://<ip>:8080/debug/pprof/heap > <filename>.pprof
go tool pprof -http=":9090" -seconds=300 <filename>.out

## Анализ результатов

Основные оптимизации были направлены на компоненты gzip компрессии и декомпрессии. Как видно из результатов, сильно уменьшилось количество необходимой памяти для:

1. **compress/flate.NewWriter**
2. **compress/flate.(*compressor).init**
3. **compress/flate.newDeflateFast**
В сумме оптимизация компонентов сжатия дала существенное улучшение.

## Вывод

Оптимизация компонентов gzip компрессии и декомпрессии показала значительный результат.
# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Указание версию сборки, времени сборки и комментария
И в агентской и серверной части определены три переменные: buildVersion, buildDate, buildCommit. Их необходимо указать при сборке через -ldflags, иначе значение будет равно N/A.
Примеры команды:
1. для агента: go build -ldflags "-X main.buildVersion=<version> -X main.buildDate=<date> -X main.buildCommit=<commit>" -o agent ./cmd/agent
2. для сервера: go build -ldflags "-X main.buildVersion=<version> -X main.buildDate=<date> -X main.buildCommit=<commit>" -o server ./cmd/server

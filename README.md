# Лабораторная работа #4

![GitHub Classroom Workflow](../../workflows/GitHub%20Classroom%20Workflow/badge.svg?branch=master)

## Deploy to Cloud

### Формулировка

На базе [Лабораторной работы #2](https://github.com/bmstu-rsoi/lab2-template) выполнить деплой приложения в managed
кластер k8s.

### Требования

1. Скопировать исходный код из ЛР #2 в проект.
2. Развернуть руками свой Managed Kubernetes Cluster, настроить Ingress Controller (для публикации сервисов наружу можно
   использовать _только_ Ingress).
4. Собрать и опубликовать образы docker в [Docker Registry](https://hub.docker.com/).
5. Описать манифесты для деплоя в виде [helm charts](https://helm.sh/docs/topics/charts/), они должен быть универсальным
   для всех сервисов и отличаться лишь набором параметров запуска.
6. В кластере k8s можно использовать один физический instance базы, но каждый сервис должен работать только со своей
   виртуальной базой данных. Задеплоить базу в кластер можно руками, либо использовать уже готовый helm chart.
7. Код хранить на Github, для сборки использовать Github Actions.
8. Для автоматических прогонов тестов в файле [autograding.json](.github/classroom/autograding.json)
   и [classroom.yml](.github/workflows/classroom.yml) заменить `<variant>` на ваш вариант.
9. В [classroom.yml](.github/workflows/classroom.yml) дописать шаги:
    1. сборка приложения;
    2. сборка и публикация образа docker (можно использовать `docker compose build`, `docker compose push`);
    3. деплой каждого сервиса в кластер k8s.

### Пояснения

1. Т.к. развертывание полноценного кластера на виртуальным машинах очень сложный процесс, можно использовать Managed
Kubernetes Cluster, т.е. готовый кластер k8s, предоставляемый сторонней платформой, например:
   * [Digital Ocean](https://www.digitalocean.com/products/kubernetes/)
   * [Yandex Cloud](https://cloud.yandex.ru/services/managed-kubernetes)
   * [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine)
   * [AWS](https://aws.amazon.com/ru/eks/)

2. Платформ, которые предоставляют Kubernetes as a Service большое количество, вы можете сами исследовать рынок и выбрать
другого провайдера услуг. Большинство провайдеров имеют бесплатный триальный период или денежный грант.

3. Для создания кластера достаточно 2-3 worker ноды 2Gb, 1CPU.

4. Для проверки отказоустойчивости используется остановка и запуск контейнеров docker, это делает
   скрипт [test-script.sh](scripts/test-script.sh). Скрипт нужно запускать из корня проекта, т.к. он обращается к папке
   postman по вариантам.
   ```shell
   # запуск тестового сценария:
   # * <variant> – номер варианта (v1 | v2 | v3 | v4 )
   # * <deployment> – имя deployment в кластере k8s
   # * <namespace>    – namespace кластера, в котором развёрнуты сервисы (по умолчанию default)
   $ ./scripts/test-script.sh <variant> <deployment> <k8s namespace>
   ```

### Прием задания

1. При получении задания у вас создается fork этого репозитория для вашего пользователя.
2. После того как все тесты успешно завершатся, в Github Classroom на Dashboard будет отмечено успешное выполнение
   тестов.

## Реализация

Исходный код сервисов и `lb-core` перенесён из Lab 2. Каждый сервис собирается в отдельный Docker-образ; локально
они запускаются через `docker-compose.yml`.

Kubernetes-манифесты находятся в `helm/`:

* `helm/microservice` — универсальный Helm chart для `ms-flight`, `ms-ticket`, `ms-bonus` и `ms-gateway`;
* `helm/microservice/values-ms-*.yaml` — только параметры конкретного сервиса: имя, порт, образ и строка подключения;
* `helm/postgres` — один экземпляр PostgreSQL с базами `flights`, `tickets` и `privileges`.

Все внутренние сервисы публикуются как `ClusterIP`. Единственный внешний маршрут описан в Ingress `ms-gateway`.
Перед развёртыванием укажите реальный домен ingress в
`helm/microservice/values-ms-gateway.yaml` вместо `rsoi.local` и настройте DNS-запись у провайдера кластера.

### Локальная проверка

```shell
make test
DOCKER_REGISTRY=<docker-hub-user> IMAGE_TAG=latest make build
```

### Публикация и деплой

Создайте managed Kubernetes cluster и установите в нём Ingress Controller. Затем войдите в Docker Hub и выполните:

```shell
docker login
DOCKER_REGISTRY=<docker-hub-user> IMAGE_TAG=<tag> make push
DOCKER_REGISTRY=<docker-hub-user> IMAGE_TAG=<tag> NAMESPACE=rsoi make deploy
```

Для GitHub Actions нужны secrets `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN` и `KUBECONFIG_B64` (base64-кодированный
kubeconfig). На push в `master` workflow запускает тесты, публикует четыре образа и обновляет Helm releases.

### Варианты заданий

Распределение вариантов заданий аналогично [ЛР #2](https://github.com/bmstu-rsoi/lab2-template).

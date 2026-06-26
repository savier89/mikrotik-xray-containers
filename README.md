# MikroTik Sing-Box Container

Один контейнер для MikroTik RouterOS: sing-box-extended (VPN ядро) + Go API сервер + React Web UI.

## Архитектура

```
┌─────────────────────────────────────────────────┐
│           Один контейнер на MikroTik             │
│                                                  │
│  ┌──────────┐   ┌──────────┐   ┌─────────────┐  │
│  │ sing-box │◄──│ API      │◄──│ Web UI      │  │
│  │ (VPN)    │   │ Server   │   │ (React + Go)│  │
│  │ :20123   │   │ :9090    │   │ :11501      │  │
│  └──────────┘   └──────────┘   └─────────────┘  │
│       │              │              │            │
│       │ clash_api    │ REST API     │ HTTP       │
│       └──────────────┴──────────────┘            │
└─────────────────────────────────────────────────┘

sing-box-extended: v1.13.14-extended-2.5.0
Протоколы: VLESS, VMess, Trojan, Hysteria2, Shadowsocks
Транспорт: TCP, WebSocket, gRPC, xHTTP, HTTP-Upgrade
```

**Один контейнер содержит:**
- **sing-box** — VPN ядро с TUN интерфейсом, маршрутизацией, поддержкой подписок
- **API Server** (Go) — REST API на порту 9090, управляет sing-box через clash_api и прямой доступ к конфигу
- **Web UI** (React + Go) — веб-интерфейс на порту 11501, проксирует запросы к API Server

## Сборка

### 1. Запустить локальный registry

```bash
docker run -d -p 5000:5000 --name registry registry:2
```

### 2. Собрать образы

```bash
cd ~/mikrotik-xray-containers
chmod +x build.sh

# Все архитектуры
./build.sh all

# Только ARM64 (для MikroTik)
./build.sh arm64

# С кастомным registry
REGISTRY=192.168.1.100:5000 ./build.sh arm64
```

Скрипт собирает два образа:
- `mikrotik-singbox/sing-box:latest-{arch}` — sing-box + API + Web UI
- `mikrotik-singbox/web-ui:latest-{arch}` — Web UI отдельно

### 3. Доступ к registry с MikroTik

```bash
REGISTRY=192.168.1.100:5000 ./build.sh arm64
```

На MikroTik:
```routeros
/container/config set registry-url=http://192.168.1.100:5000
```

## Настройка MikroTik

### Сеть

```routeros
/interface bridge
add name=docker

/interface veth
add address=172.17.0.2/24 comment="sing-box" gateway=172.17.0.1 gateway6="" name=veth1

/interface bridge port
add bridge=docker interface=veth1

/ip address
add address=172.17.0.1/24 interface=docker
```

### Registry

```routeros
/container/config set registry-url=http://YOUR_REGISTRY_IP:5000 tmpdir=/ramstorage
```

### Environment variables

```routeros
/container envs

# Только базовые настройки — всё остальное через Web UI
add key=API_PORT name=singbox value=9090
add key=API_HOST name=singbox value=0.0.0.0
add key=API_AUTH_TOKEN name=singbox value=your-secret
add key=SINGBOX_API_PORT name=singbox value=20123
add key=WEBUI_PORT name=singbox value=11501
add key=LOG_LEVEL name=singbox value=warn
add key=DNS_UPSTREAM name=singbox value=8.8.8.8
add key=TZ name=singbox value=Europe/Moscow
```

### Контейнер

```routeros
/container
add dns=172.17.0.1 envlist=singbox interface=veth1 logging=yes \
    remote-image=YOUR_REGISTRY_IP:5000/mikrotik-singbox/sing-box:latest-arm64 \
    root-dir=docker/singbox start-on-boot=yes
```

### Маршрутизация

```routeros
/routing table
add disabled=no fib name=proxy_mark

/ip firewall mangle
add action=mark-routing chain=prerouting comment="sing-box" \
    dst-address-list=route_proxy new-routing-mark=proxy_mark passthrough=no

/ip firewall nat
add action=masquerade chain=srcnat out-interface=docker

/ip route
add comment="VPN traffic -> sing-box" \
    disabled=no distance=10 dst-address=0.0.0.0/0 \
    gateway=172.17.0.2 routing-table=proxy_mark scope=30 target-scope=10
```

### DNS правила

```routeros
/ip firewall filter
add chain=input in-interface=veth1 protocol=udp dst-port=53 action=accept \
    comment="sing-box DNS"
add chain=input in-interface=veth1 protocol=tcp dst-port=53 action=accept \
    comment="sing-box DNS TCP"
```

## Переменные окружения

| Переменная | Описание | Значение по умолчанию |
|---|---|---|
| `API_PORT` | Порт API сервера | `9090` |
| `API_HOST` | Хост API сервера | `0.0.0.0` |
| `API_AUTH_TOKEN` | Токен авторизации API | `""` |
| `SINGBOX_API_PORT` | Порт clash_api sing-box | `20123` |
| `SINGBOX_API_TOKEN` | Токен clash_api | `""` |
| `WEBUI_PORT` | Порт Web UI | `11501` |
| `LOG_LEVEL` | Уровень логов sing-box | `warn` |
| `DNS_UPSTREAM` | DNS сервер | `8.8.8.8` |
| `TUN_STACK` | Stack TUN | `system` |
| `TUN_MTU` | MTU TUN | `1500` |
| `TZ` | Часовой пояс | `UTC` |

## Web UI

Остальная конфигурация — через веб-интерфейс (порт 11501 по умолчанию).

**Подписки и серверы:**
- Добавьте подписку по URL или вручную введите ссылки на серверы
- Поддерживаемые протоколы: VLESS, VMess, Trojan, Hysteria2, Shadowsocks
- Поддерживаемые транспортные протоколы: TCP, WebSocket, gRPC, xHTTP, HTTP-Upgrade
- Тестирование серверов (задержка)
- Выбор сервера — через Web UI

**Управление:**
- Подключение / отключение sing-box
- Мониторинг трафика (upload/download)
- Активные подключения
- Логи sing-box
- Просмотр и редактирование конфига

## API Server

REST API на порту 9090 (по умолчанию).

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/health` | Health check |
| GET | `/api/status` | Статус sing-box, PID, uptime |
| GET | `/api/stats` | Трафик (upload/download) |
| GET | `/api/connections` | Активные подключения |
| GET | `/api/subscriptions` | Список подписок |
| POST | `/api/subscriptions` | Добавить подписку |
| DELETE | `/api/subscriptions/:id` | Удалить подписку |
| POST | `/api/subscriptions/:id/activate` | Активировать подписку |
| GET | `/api/servers` | Серверы активной подписки |
| POST | `/api/servers/select` | Выбрать сервер |
| POST | `/api/servers/test` | Протестировать сервер |
| GET | `/api/config` | Текущий конфиг sing-box |
| POST | `/api/config` | Обновить конфиг sing-box |
| POST | `/api/connect` | Подключить sing-box |
| POST | `/api/disconnect` | Отключить sing-box |
| GET | `/api/logs` | Логи sing-box |

## Обновление

```bash
# Пересобрать и запушить новые образы
REGISTRY=192.168.1.100:5000 TAG=v2 ./build.sh arm64

# На MikroTik остановить, удалить и перезапустить контейнер с новым тегом
```

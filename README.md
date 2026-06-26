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

# sing-box контейнер
add key=REMOTE_ADDRESS name=singbox value=your-vless-server.com
add key=REMOTE_PORT name=singbox value=443
add key=ID name=singbox value=YOUR-UUID-HERE
add key=SERVER_NAME name=singbox value=google.com
add key=NETWORK name=singbox value=tcp
add key=LOG_LEVEL name=singbox value=warn
add key=API_PORT name=singbox value=9090
add key=API_HOST name=singbox value=0.0.0.0
add key=SINGBOX_API_PORT name=singbox value=20123
add key=WEBUI_PORT name=singbox value=11501
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

| Переменная | Описание | Пример |
|---|---|---|
| `REMOTE_ADDRESS` | Хост VPN сервера | `mydomain.com` |
| `REMOTE_PORT` | Порт сервера | `443` |
| `ID` | UUID клиента (VLESS) / пароль | `YOUR-UUID-HERE` |
| `SERVER_NAME` | SNI | `google.com` |
| `FLOW` | Flow (xtls-reality) | `xtls-rprx-vision` |
| `NETWORK` | Транспорт | `tcp`, `ws`, `grpc`, `xhttp` |
| `WS_PATH` | WebSocket/xHTTP path | `/websocket` |
| `PUBLIC_KEY` | PublicKey (Reality) | `7JTFIDt3...` |
| `SHORT_ID` | ShortId (Reality) | `aeb4c72f...` |
| `DNS_UPSTREAM` | DNS серверы (через запятую) | `8.8.8.8,8.8.4.4` |
| `DNS_TYPE` | Тип DNS | `udp`, `doh` |
| `TUN_STACK` | Stack TUN | `system`, `gvisor` |
| `TUN_MTU` | MTU TUN | `1500` |
| `LOG_LEVEL` | Уровень логов sing-box | `warn` |
| `API_PORT` | Порт API сервера | `9090` |
| `API_HOST` | Хост API сервера | `0.0.0.0` |
| `API_AUTH_TOKEN` | Токен авторизации API | `your-secret` |
| `SINGBOX_API_PORT` | Порт clash_api sing-box | `20123` |
| `SINGBOX_API_TOKEN` | Токен clash_api | `token` |
| `WEBUI_PORT` | Порт Web UI | `11501` |
| `SUB_URL` | URL подписки | `https://sub.example.com/api/...` |
| `SUB_SELECT` | Выбор сервера | `auto`, `index:1`, `random`, `fastest` |
| `DIRECT_IPS` | IP для прямого доступа (через запятую) | `10.0.0.0/8,172.16.0.0/12` |
| `DOMAINS` | Домены для прямого доступа | `example.com,internal.local` |

## Подписки

Контейнер поддерживает подписки с автоматическим обновлением. Укажите `SUB_URL` в переменных окружения.

**Формат подписки:** список ссылок (VLESS, VMess, Trojan, Hysteria2, Shadowsocks), необязательно в base64.

**Выбор сервера (`SUB_SELECT`):**
- `auto` / `first` — первый сервер из списка
- `index:N` — сервер по индексу (начиная с 1)
- `random` — случайный сервер
- `fastest` — сервер с минимальной задержкой

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

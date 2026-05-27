# MikroTik Xray VLESS + hev-socks5-tunnel

Прозрачный Xray VLESS туннель на MikroTik через два контейнера.

## Архитектура

```
LAN клиенты -> MikroTik mangle -> routing table -> bridge ->
  ├── veth1 (172.17.0.2) -> [xray] SOCKS :10800 -> VLESS сервер
  └── veth2 (172.17.0.3) -> [hev-socks5-tunnel] tun0 -> SOCKS -> xray
```

**Почему два контейнера:**
- **xray** — SOCKS-прокси, подключается к VLESS серверу
- **hev-socks5-tunnel** — маршрутизирует весь трафик из TUN в SOCKS-прокси
- Каждый бинарник скачивается с GitHub Releases на этапе сборки
- Конфигурация передаётся через env-переменные (без приватного репозитория)

**hev-socks5-tunnel вместо tun2socks:**
- Написан на C (LwIP + корутины), меньше потребление CPU/памяти
- Benchmarks: 32.8 Gbps vs 15.3 Gbps (tun2socks) на одном потоке
- 4 MB RAM vs 22+ MB у tun2socks — критично для hAP ax² (1 GB RAM)

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

# Только ARM64 (для hAP ax²)
./build.sh arm64
```

Скрипт автоматически:
- Определяет архитектуру через `--platform`
- Скачивает Xray и hev-socks5-tunnel с GitHub Releases
- Собирает Alpine-образы
- Пушит в локальный registry (`localhost:5000`)

### 3. Доступ к registry с MikroTik

Если registry на другой машине, укажи IP:

```bash
REGISTRY=192.168.1.100:5000 ./build.sh arm64
```

На MikroTik укажи тот же адрес:
```
/container/config set registry-url=http://192.168.1.100:5000
```

## Настройка MikroTik

### Сеть

```routeros
# Bridge для контейнеров
/interface bridge
add name=docker

# veth интерфейсы
/interface veth
add address=172.17.0.2/24 comment="Xray VLESS" gateway=172.17.0.1 gateway6="" name=veth1
add address=172.17.0.3/24 comment="hev-socks5-tunnel" gateway=172.17.0.1 gateway6="" name=veth2

# Bridge порты
/interface bridge port
add bridge=docker interface=veth1
add bridge=docker interface=veth2

# IP bridge
/ip address
add address=172.17.0.1/24 interface=docker
```

### Registry

```routeros
/container config
set registry-url=http://YOUR_REGISTRY_IP:5000 tmpdir=/ramstorage
```

### Environment variables

```routeros
/container envs

# Xray контейнер
add key=SERVER_ADDRESS name=xray value=your-vless-server.com
add key=SERVER_PORT name=xray value=443
add key=ID name=xray value=YOUR-UUID-HERE-CHANGE-ME
add key=ENCRYPTION name=xray value=none
add key=FLOW name=xray value=xtls-rprx-vision
add key=NETWORK name=xray value=tcp
add key=SECURITY name=xray value=reality
add key=SNI name=xray value=google.com
add key=FP name=xray value=chrome
add key=PBK name=xray value=YOUR_PUBLIC_KEY
add key=SID name=xray value=YOUR_SHORT_ID
add key=SPX name=xray value=/
add key=LOGLEVEL name=xray value=warning
add key=TZ name=xray value=Europe/Moscow

# hev-socks5-tunnel контейнер
add key=SOCKS5_ADDR name=hev-socks5-tunnel value=172.17.0.2
add key=SOCKS5_PORT name=hev-socks5-tunnel value=10800
add key=LOG_LEVEL name=hev-socks5-tunnel value=warn
add key=TZ name=hev-socks5-tunnel value=Europe/Moscow
```

### Контейнеры

```routeros
/container
add dns=172.17.0.1 envlist=xray interface=veth1 logging=yes \
    remote-image=YOUR_REGISTRY_IP:5000/mikrotik-xray/xray:latest-arm64 \
    root-dir=docker/xray start-on-boot=yes

add dns=172.17.0.1 envlist=hev-socks5-tunnel interface=veth2 logging=yes \
    remote-image=YOUR_REGISTRY_IP:5000/mikrotik-xray/hev-socks5-tunnel:latest-arm64 \
    root-dir=docker/hev-socks5-tunnel start-on-boot=yes
```

### Маршрутизация

```routeros
# Таблица маршрутизации для VPN трафика
/routing table
add disabled=no fib name=proxy_mark

# Mangle: маркируем трафик по destination address-list
/ip firewall mangle
add action=mark-routing chain=prerouting comment="Xray VLESS" \
    dst-address-list=route_proxy new-routing-mark=proxy_mark passthrough=no
add action=mark-routing chain=output comment="Xray VLESS" \
    dst-address-list=route_proxy new-routing-mark=proxy_mark passthrough=no

# NAT для docker bridge
/ip firewall nat
add action=masquerade chain=srcnat out-interface=docker

# Маршрут: весь маркированный трафик -> hev-socks5-tunnel
/ip route
add comment="VPN traffic -> hev-socks5-tunnel" \
    disabled=no distance=10 dst-address=0.0.0.0/0 \
    gateway=172.17.0.3 routing-table=proxy_mark scope=30 target-scope=10
```

### DNS правила

```routeros
# Разрешить DNS от контейнеров
/ip firewall filter
add chain=input in-interface=veth1 protocol=udp dst-port=53 action=accept \
    comment="xray DNS"
add chain=input in-interface=veth2 protocol=udp dst-port=53 action=accept \
    comment="hev-socks5-tunnel DNS"
add chain=input in-interface=veth1 protocol=tcp dst-port=53 action=accept \
    comment="xray DNS TCP"
add chain=input in-interface=veth2 protocol=tcp dst-port=53 action=accept \
    comment="hev-socks5-tunnel DNS TCP"
```

### Address-list (что маршрутизировать)

```routeros
# Пример: российские IP (сгенерировать на https://ip-api.com/)
/ip firewall address-list
add address=2.56.24.0/22 list=route_proxy
add address=2.56.88.0/22 list=route_proxy
# ... добавить нужные подсети
```

## Переменные окружения

### Xray контейнер

| Переменная | Описание | Пример |
|---|---|---|
| `SERVER_ADDRESS` | Хост VLESS сервера | `mydomain.com` |
| `SERVER_PORT` | Порт сервера | `443` |
| `ID` | UUID клиента | `YOUR-UUID-HERE` |
| `ENCRYPTION` | Шифрование VLESS | `none` |
| `FLOW` | Flow (для xtls-reality) | `xtls-rprx-vision` |
| `NETWORK` | Транспорт | `tcp`, `ws`, `grpc` |
| `SECURITY` | TLS security | `none`, `tls`, `reality` |
| `SNI` | Server Name Indication | `google.com` |
| `WS_PATH` | WebSocket path | `/websocket` |
| `FP` | Fingerprint (Reality) | `chrome` |
| `PBK` | PublicKey (Reality) | `7JTFIDt3...` |
| `SID` | ShortId (Reality) | `aeb4c72f...` |
| `SPX` | SpiderX path (Reality) | `/` |
| `LOGLEVEL` | Уровень логов | `warning` |
| `SOCKS_PORT` | Порт SOCKS | `10800` |

### hev-socks5-tunnel контейнер

| Переменная | Описание | Пример |
|---|---|---|
| `SOCKS5_ADDR` | IP Xray контейнера | `172.17.0.2` |
| `SOCKS5_PORT` | Порт SOCKS Xray | `10800` |
| `TUN_NAME` | Имя TUN интерфейса | `tun0` |
| `MTU` | MTU TUN | `8500` |
| `TUN_IPV4` | IP TUN интерфейса | `198.18.0.1` |
| `TABLE` | ID routing table | `20` |
| `MARK` | Firewall mark | `438` |
| `UDP_MODE` | Режим UDP | `udp`, `fullcone`, `tcp` |
| `LOG_LEVEL` | Уровень логов | `warn` |
| `GATEWAY` | IP шлюза MikroTik | `172.17.0.1` |

## Обновление

```bash
# Пересобрать и запушить новые образы
REGISTRY=192.168.1.100:5000 TAG=v2 ./build.sh arm64

# На MikroTik остановить, удалить и перезапустить контейнеры с новым тегом
```

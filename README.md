# Wire-Pod for Home Assistant

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Home Assistant Add-on для [Wire-Pod](https://github.com/kercre123/wire-pod) - бесплатного серверного ПО для робота Anki Vector.

## ⚠️ Важно: Сборка локально

**Raspberry Pi не справляется со сборкой Docker образа** — компиляция Go + VOSK модели занимает >2GB RAM и приводит к зависанию.

**Решение:** Собирай образ на ПК/сервере, потом переноси на Pi.

---

## 🚀 Быстрая установка (рекомендуется)

### Способ 1: Сборка на ПК (x86_64)

Если у тебя есть ПК/ноутбук/сервер на x86_64 Linux:

```bash
# Клонируй репозиторий
git clone https://github.com/KaeShoru/wire-pod-addon.git
cd wire-pod-addon

# Собери образ для aarch64 (Raspberry Pi 4)
docker buildx build \
  --platform linux/arm64 \
  -t wire-pod-addon:aarch64-latest \
  --load .

# Сохрани образ
docker save wire-pod-addon:aarch64-latest > wire-pod-addon.tar

# Перенеси на Raspberry Pi
scp wire-pod-addon.tar pi@192.168.2.71:/tmp/
```

На Raspberry Pi:
```bash
# Загрузи образ
ssh pi@192.168.2.71 "docker load < /tmp/wire-pod-addon.tar"
```

### Способ 2: Локальная сборка с ограничениями

Если собираешь прямо на Raspberry Pi 4:

```bash
# Перед установкой аддона ограничь ресурсы Docker
# Отредактируй /etc/docker/daemon.json:
{
  "experimental": false,
  "builder": {
    "gc": {
      "defaultKeepStorage": "20GB",
      "enabled": true
    }
  }
}

# Перезапусти Docker
sudo systemctl restart docker

# Сборка с ограничением памяти (1GB)
cd /addons/wire-pod-addon
docker build --memory=1g --memory-swap=1g -t wire-pod-addon:test .
```

**⚠️ Предупреждение:** Сборка на Pi займёт 20-40 минут и может зависнуть. Используй Способ 1 если возможно.

---

## 📦 Установка через Add-on Store (после сборки)

1. Скопируй папку аддона в `/addons` на HA
2. **Settings** → **Add-ons** → **Add-on Store**
3. **⋮** → **Reload**
4. Найди **Wire-Pod for Vector** → **Install**
5. Если образ уже собран — аддон использует его

---

## ⚙️ Конфигурация

### Обязательные настройки

| Опция | Описание | Пример |
|-------|----------|--------|
| `vector_esn` | ESN робота Vector | `006044c4` |
| `mqtt_broker_host` | Хост MQTT брокера | `core-mosquitto` |

### Как узнать ESN

```bash
# На устройстве с активированным Vector:
vector-auth list

# Или в старом wire-pod:
cat /path/to/wire-pod/chipper/config/robot_config.json | grep esn
```

### Дополнительные настройки

| Опция | Описание | По умолчанию |
|-------|----------|--------------|
| `stt_service` | Сервис распознавания речи | `vosk` |
| `vosk_model` | Языковая модель | `small-en-us` |
| `language` | Язык | `en-US` |
| `mqtt_discovery` | Auto-discovery сенсоров | `true` |
| `telemetry_interval` | Интервал обновления | `30` сек |

---

## 🎮 Использование

### Автоматические сенсоры

После запуска создаются:
- `sensor.vector_battery` — заряд (%)
- `sensor.vector_status` — статус (idle/charging/driving)
- `sensor.vector_wifi_signal` — WiFi сигнал (dBm)
- `binary_sensor.vector_charging` — заряжается ли

### Команды через MQTT

```yaml
service: mqtt.publish
data:
  topic: homeassistant/vector/command
  payload: drive_forward
```

**Команды:** `drive_forward`, `drive_backward`, `turn_left`, `turn_right`, `dock`, `undock`

### Изменение цвета глаз

```yaml
service: mqtt.publish
data:
  topic: homeassistant/vector/set_eyes
  payload: '{"hue": 120, "saturation": 100}'
```

---

## 🔧 Устранение неполадок

### "An unknown error occurred while trying to build the image"

**Причина:** Raspberry Pi не хватает ресурсов для сборки.

**Решение:** 
1. Собирай образ на другом устройстве (x86_64)
2. Или увеличь swap до 2GB:
   ```bash
   sudo dphys-swapfile swapoff
   sudo nano /etc/dphys-swapfile
   # CONF_SWAPSIZE=2048
   sudo dphys-swapfile setup
   sudo dphys-swapfile swapon
   ```

### "No module named 'paho'" или ошибки MQTT

Убедись что Mosquitto установлен:
- **Settings** → **Add-ons** → **Mosquitto broker** → **Install**

### Vector не подключается

1. Проверь ESN в настройках
2. Vector и HA должны быть в одной сети
3. Проверь логи аддона

---

## 📚 Дополнительно

- [Wire-Pod Wiki](https://github.com/kercre123/wire-pod/wiki)
- [Vector SDK Docs](https://developer.anki.com/vector/docs/)
- [Home Assistant MQTT](https://www.home-assistant.io/integrations/mqtt/)

## Лицензия

MIT License — см. [LICENSE](LICENSE)
# Test workflow trigger

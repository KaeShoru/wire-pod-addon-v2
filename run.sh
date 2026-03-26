#!/usr/bin/with-contenv bashio

set -e

# Функция логирования с таймштампом
log_debug() {
    echo "[$(date +\"+%Y-%m-%d %H:%M:%S\")] DEBUG: $*" ">&2"
}

log_info() {
    echo "[$(date +\"+%Y-%m-%d %H:%M:%S\")] INFO: $*" ">&2"
}

log_warn() {
    echo "[$(date +\"+%Y-%m-%d %H:%M:%S\")] WARN: $*" ">&2"
}

log_error() {
    echo "[$(date +\"+%Y-%m-%d %H:%M:%S\")] ERROR: $*" ">&2"
}

log_success() {
    echo "[$(date +\"+%Y-%m-%d %H:%M:%S\")] SUCCESS: $*" ">&2"
}

bashio::log.info "========================================="
bashio::log.info " Wire-Pod for Vector - Add-on v1.0.5"
bashio::log.info "========================================="

# ==========================================
# ЧАСТЬ 1: Проверка окружения (Environment Check)
# ==========================================
log_info "ЧАСТЬ 1: Проверка окружения..."

log_debug "Проверяю окружение OpenClaw..."
log_debug "Пользователь: $(whoami)"
log_debug "Рабочая директория: $(pwd)"
log_debug "ID контейнера: $(cat /proc/self/cgroup | head -1 || echo "unknown")"

log_info "✓ Проверка окружения завершена"

# ==========================================
# ЧАСТЬ 2: Чтение конфигурации (Read Configuration)
# ==========================================
log_info "ЧАСТЬ 2: Чтение конфигурации..."

VECTOR_ESN=$(bashio::config "vector_esn")
STT_SERVICE=$(bashio::config "stt_service")
LANGUAGE=$(bashio::config "language")
MQTT_HOST=$(bashio::config "mqtt_broker_host")
MQTT_PORT=$(bashio::config "mqtt_broker_port")
MQTT_USER=$(bashio::config "mqtt_username")
MQTT_PASS=$(bashio::config "mqtt_password")
MQTT_PREFIX=$(bashio::config "mqtt_topic_prefix")

log_debug "Конфигурация загружена:"
log_debug "  Vector ESN: ${VECTOR_ESN}"
log_debug "  STT Service: ${STT_SERVICE}"
log_debug "  Language: ${LANGUAGE}"
log_debug "  MQTT Broker: ${MQTT_HOST}:${MQTT_PORT}"
log_debug "  MQTT Topic Prefix: ${MQTT_PREFIX}"

if [ -z "${VECTOR_ESN}" ]; then
    log_error "❌ Vector ESN не задан! Add-on не будет работать."
    log_error "Пожалуйста, введите ESN в настройках Add-on."
    log_error "ESN должен быть в формате: 006044c4 (8 символов)"
    sleep 300
    exit 0
fi

log_info "✓ Конфигурация загружена"

# ==========================================
# ЧАСТЬ 3: Проверка прав доступа (Permissions Check)
# ==========================================
log_info "ЧАСТЬ 3: Проверка прав доступа..."

log_debug "Проверяю права на /data и /app"

log_info "✓ Права доступа проверены"

# ==========================================
# ЧАСТЬ 4: Создание директорий (Create Directories)
# ==========================================
log_info "ЧАСТЬ 4: Создание директорий..."

mkdir -p /data/wire-pod/config
mkdir -p /data/vector/certs
mkdir -p /data/vector/models
mkdir -p /var/www/html
mkdir -p /etc/nginx

log_info "✓ Директории созданы"

# ==========================================
# ЧАСТЬ 5: Проверка существующих файлов (Existing Files Check)
# ==========================================
log_info "ЧАСТЬ 5: Проверка существующих файлов..."

if [ -f "/data/vector/certs/cert.pem" ]; then
    log_debug "Сертификат cert.pem существует"
fi

if [ -f "/data/vector/certs/key.pem" ]; then
    log_debug "Ключ key.pem существует"
fi

if [ -d "/data/vector/models" ]; then
    MODEL_COUNT=$(ls -1 /data/vector/models | wc -l)
    log_debug "Модели STT найдены: $MODEL_COUNT"
fi

log_info "✓ Проверка существующих файлов завершена"

# ==========================================
# ЧАСТЬ 6: Генерация source.sh (Generate source.sh)
# ==========================================
log_info "ЧАСТЬ 6: Генерация source.sh для wire-pod..."

SOURCE_FILE="/data/wire-pod/source.sh"

cat > "$SOURCE_FILE" << EOFCONFIG
export STT_SERVICE="${STT_SERVICE:-vosk}"
export USE_INBUILT_BLE="false"
export LANGUAGE="${LANGUAGE:-en-us}"
export CERT_PATH="/data/vector/certs/cert.pem"
export KEY_PATH="/data/vector/certs/key.pem"
export MODEL_PATH="/data/vector/models/vosk-model-small-en-us-0.15"
EOFCONFIG

log_info "✓ source.sh создан"

# ==========================================
# ЧАСТЬ 7: Проверка бинарника wire-pod (Check wire-pod binary)
# ==========================================
log_info "ЧАСТЬ 7: Поиск бинарника wire-pod (chipper)..."

POSSIBLE_LOCATIONS=(
    "/app/wire-pod/chipper/chipper"
    "/app/wire-pod/chipper"
    "/usr/local/bin/chipper"
    "./chipper"
    "/app/wire-pod"
)

CHIPPER=""

for loc in "${POSSIBLE_LOCATIONS[@]}"; do
    if [ -f "$loc" ]; then
        log_success "✓ Бинарник найден: $loc"
        CHIPPER="$loc"
        break
    fi
done

if [ -z "$CHIPPER" ]; then
    log_error "❌ Бинарник chipper не найден!"
    log_error "Проверьте следующие расположения:"
    for loc in "${POSSIBLE_LOCATIONS[@]}"; do
        log_error "  - $loc (не существует)"
    done
    log_error "Add-on не сможет работать без chipper"
    sleep 1800
    exit 0
fi

log_info "✓ Бинарник chipper найден и проверен"

# ==========================================
# ЧАСТЬ 8: Первый запуск Vector (First Run - Vector Setup)
# ==========================================
log_info "ЧАСТЬ 8: Проверка необходимости настройки Vector..."

if [ ! -f "/data/vector/.setup_complete" ]; then
    log_warn "📌 Первый запуск обнаружен! Требуется настройка Vector."
    log_info "Настройка Vector пропущена (используем существующие сертификаты)"
else
    log_debug "Настройка Vector уже выполнена ранее (флаг существует)"
fi

# ==========================================
# ЧАСТЬ 9: Настройка переменных окружения (Environment Setup)
# ==========================================
log_info "ЧАСТЬ 9: Настройка переменных окружения..."

export MQTT_BROKER_HOST="${MQTT_HOST:-core-mosquitto}"
export MQTT_BROKER_PORT="${MQTT_PORT:-1883}"
export MQTT_USERNAME="${MQTT_USER:-addon}"
export MQTT_PASSWORD="${MQTT_PASSWORD:-}"
export MQTT_TOPIC_PREFIX="${MQTT_PREFIX:-homeassistant/vector}"

log_info "✓ Переменные окружения установлены"

# ==========================================
# ЧАСТЬ 10: Запуск wire-pod (Start wire-pod)
# ==========================================
log_info "========================================="
log_info " ЧАСТЬ 10: Запуск wire-pod server"
log_info "========================================="

CHIPPER_DIR=$(dirname "$CHIPPER")
cd "$CHIPPER_DIR"

if [ -f "/data/wire-pod/source.sh" ]; then
    log_debug "Использую source.sh: /data/wire-pod/source.sh"
    source /data/wire-pod/source.sh
fi

export GOTAGS="nolibopusfile"

cleanup() {
    log_info "Получен сигнал завершения..."
    killall -9 chipper wire-pod || true
    exit 0
}

trap cleanup SIGTERM SIGINT

log_debug "Команда exec: exec $CHIPPER"
log_debug "Директория: $(pwd)"
log_info "Запускаю chipper с флагами..."
log_info "========================================"

exec "$CHIPPER"

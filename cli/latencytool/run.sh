#!/bin/bash

cd $(dirname $0)
BASE_DIR=$(pwd)
LOG_DIR="${BASE_DIR}/logs"

APP="latencytool"
LOG_FILE="${LOG_DIR}/${APP}.log"
RUN_LOG="${LOG_DIR}/run.log"
# rotate size in MB
LOG_ROTATE=500
LOG_KEEP=5
REM_CONFIG="${BASE_DIR}/rem4go.toml"
YD_CONFIG="${BASE_DIR}/config.ini"
RUN_INTERVAL="5m"
BEFORE_DUR="3m"
LATENCY_FILE="${BASE_DIR}/latency.json"

COMMAND="report"
declare -a CTL_ARGS=("--ctl" "ipc://latencytool" "--ctl" "tcp://127.0.0.1:45678" "--log" "${LOG_FILE}")
declare -a LATENCY_ARGS=("--interval" "${RUN_INTERVAL}" "--before" "${BEFORE_DUR}" "--sink" "${LATENCY_FILE}")
declare -a PLUGIN_ARGS=("--plugin" "rem4go" "--config" "rem4go=${REM_CONFIG}" "--plugin" "yd4go" "--config" "yd4go=${YD_CONFIG}")

KILL="/usr/bin/kill"
_KILL0=(${KILL} "-0")
_KILL2=(${KILL} "-2")
_KILL9=(${KILL} "-9")

MKDIR="/usr/bin/mkdir"
_MKDIRP=(${MKDIR} "-p")

NOHUP="/usr/bin/nohup"

CAT="/usr/bin/cat"

AWK="/usr/bin/awk"

RM="/usr/bin/rm"
_RMF=(${RM} "-f")

ECHO="/usr/bin/echo"
_ECHOL=(${ECHO} "-n")

PS="/usr/bin/ps"
_PSEF=(${PS} "-ef")

GREP="/usr/bin/grep"
_EGREP=(${GREP} "-E")

HEAD="/usr/bin/head"
_HEAD1=(${HEAD} "-1")

PID=
PID_FILE="${BASE_DIR}/${APP}.pid"

function _pid_file() {
    if [[ -z ${PID} ]]; then
        ${_RMF[@]} "${PID_FILE}" &>/dev/null
        return 1
    else
        ${_ECHOL[@]} "${PID}" >"${PID_FILE}"
        return 0
    fi
}

function _find_pid() {
    if [[ -f "${PID_FILE}" ]]; then
        PID=$(${CAT} "${PID_FILE}")

        [[ $(${_KILL0[@]} "${PID}" &>/dev/null) ]] && return 0
        PID=

        _pid_file
    fi

    PID=$(${_PSEF[@]} | ${_EGREP[@]} "${APP}" | ${_EGREP[@]} -v "grep|bash|vim" | ${_HEAD1[@]} | ${AWK} '{print $2}')

    return $(_pid_file)
}

function _start() {
    _find_pid && {
        ${ECHO} "${APP}[${PID}] already started." >&2
        return 1
    }

    [[ -d "${LOG_DIR}" ]] || ${_MKDIRP[@]} "${LOG_DIR}"

    ulimit -c unlimited
    ulimit -n 10240

    local _execute="${BASE_DIR}/${APP}"
    if [[ ! -f ${_execute} ]]; then
        _execute="${BASE_DIR}/bin/${APP}"
    fi

    echo "Running args: ${_execute} ${CTL_ARGS[@]} ${COMMAND} ${LATENCY_ARGS[@]} ${PLUGIN_ARGS[@]}" >"${RUN_LOG}"

    LD_LIBRARY_PATH="${BASE_DIR}/libs" \
        "${NOHUP}" "${_execute}" ${CTL_ARGS[@]} ${COMMAND} ${LATENCY_ARGS[@]} ${PLUGIN_ARGS[@]} >> "${RUN_LOG}" &

    sleep 3

    _find_pid && {
        ${ECHO} "${APP}[$PID] started."
    } || {
        ${ECHO} "${APP} start failed." >&2
        return 2
    }
}

function _stop() {
    _find_pid || {
        ${ECHO} "${APP} already stopped." >&2
        return 1
    }

    local _stop=0
    local _max_stop=10

    while true; do
        _stop=$((_stop + 1))

        if [[ ${_stop} -eq ${_max_stop} ]]; then
            ${ECHO} "Stop max time[${_max_stop}] reached, killing forcely." >&2

            ${_KILL9[@]} "${PID}" &>/dev/null
        else
            ${_KILL2[@]} "${PID}" &>/dev/null
        fi

        _find_pid || {
            ${ECHO} "${APP} stopped."
            return 0
        }

        sleep 1
    done

    _find_pid && {
        ${ECHO} "${APP}[${PID}] stopping failed."
    }
}

function _status() {
    _find_pid && {
        ${ECHO} "${APP}[${PID}] is running."
        return 0
    } || {
        ${ECHO} "${APP} is not running." >&2
        return 1
    }
}

_CMD=$1
shift

case ${_CMD} in
start)
    _start
    ;;
stop)
    _stop
    ;;
status)
    _status
    ;;
*) ;;
esac

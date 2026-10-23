#!/bin/sh
# 文件说明：Windows关闭控制管道时，在WSL内回收本命令进程组，不依赖杀wsl.exe传递信号。
exec 3<&0
setsid "$@" </dev/null 3<&- &
command_pid=$!
setsid sh -c '
  trap "" HUP
  while IFS= read -r line; do :; done
  kill -TERM -"$1" 2>/dev/null || :
  sleep 5
  kill -KILL -"$1" 2>/dev/null || :
' control "$command_pid" <&3 &
watch_pid=$!
finish() {
  trap - EXIT HUP INT TERM
  if [ -n "$command_pid" ]; then
    kill -TERM -"$command_pid" 2>/dev/null || :
    sleep 1
    kill -KILL -"$command_pid" 2>/dev/null || :
  fi
  kill -TERM -"$watch_pid" 2>/dev/null || :
  wait "$watch_pid" 2>/dev/null || :
}
trap finish EXIT
trap 'exit 143' HUP INT TERM
wait "$command_pid"
status=$?
command_pid=
exit "$status"

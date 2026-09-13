# shellcheck shell=bash
# shellcheck disable=SC2154
start_forward() {
  local service="$1" port="$2" remote="$3" pid attempt tick
  for attempt in $(seq 1 12); do
    if (echo >"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
      echo "Local port $port is occupied; stop its owner and retry" >&2; return 1
    fi
    nohup kubectl --kubeconfig "$kubeconfig" -n dsh-system port-forward --address 127.0.0.1 "service/$service" "$port:$remote" \
      >"$test_root/$port.log" 2>&1 < /dev/null 9>&- &
    pid=$!
    if ! record_demo_process "$test_root/$port.pid" "$pid"; then
      wait "$pid" 2>/dev/null || true
      sleep 1
      continue
    fi
    # Node readiness can precede kubelet's restored pod sandboxes. Retry exited
    # forwarders, but never mistake another process's listener for our own.
    for tick in $(seq 1 10); do
      kill -0 "$pid" 2>/dev/null || break
      if grep -Fq "Forwarding from 127.0.0.1:$port" "$test_root/$port.log" &&
        (echo >"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then return; fi
      sleep 1
    done
    stop_demo_process "$test_root/$port.pid" 'port-forward' "--kubeconfig $kubeconfig"
    wait "$pid" 2>/dev/null || true
    sleep 1
  done
  echo "Port-forward $port did not become ready; inspect $test_root/$port.log" >&2
  return 1
}

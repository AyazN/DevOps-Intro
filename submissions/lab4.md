# Lab 4 — OS & Networking: Trace, Debug, and Read the Substrate

## 1.2 Packet trace — `lab4-trace.txt`

The annotated packet trace is saved as `lab4-trace.txt`.

### TCP handshake

* `S` — client `::1:35226` → server `::1:8080`
* `S.` — server `::1:8080` → client `::1:35226`
* `.` — client ACK

### HTTP request

```text
POST /notes HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Content-Length: 39

{"title":"trace me","body":"in flight"}
```

### HTTP response

```text
HTTP/1.1 201 Created
Content-Type: application/json
Content-Length: 93

{"id":6,"title":"trace me","body":"in flight","created_at":"2026-09-17T21:37:43.374344802Z"}
```

### Connection close

* Client sends `F.` (FIN + ACK)
* Server sends `F.` (FIN + ACK)
* Client sends final ACK

## 1.3 Substrate checks

### `ss -tlnp | grep :8080`

```text
LISTEN 0      4096               *:8080            *:*    users:(("quicknotes",pid=4840,fd=3))
```

### `ip route show`

```text
default via 172.20.16.1 dev eth0 proto kernel
172.20.16.0/20 dev eth0 proto kernel scope link src 172.20.18.212
```

### `mtr -rwc 5 localhost`

```text
Start: 2026-09-18T00:50:50+0300

HOST: DESKTOP-AY601 Loss%   Snt   Last   Avg  Best  Wrst StDev
  1.|-- localhost      0.0%     5    0.0   0.0   0.0   0.0   0.0
```

### `dig +short example.com @1.1.1.1`

```text
8.6.112.0
8.47.69.0
```

### `journalctl --user -u quicknotes -n 20 || true`

```text
-- No entries --
```

## 1.4 What I would check first if QuickNotes returned 502

If QuickNotes returned a 502, I would first check whether the service is actually listening and accepting connections on the expected port using `ss -tlnp | grep :8080`, then test the endpoint directly with `curl`. I would next check the QuickNotes process and recent logs, followed by routing, DNS, and firewall/network configuration if the service itself appeared healthy.

## 2. Deliberate bind failure and recovery

### 2.1 Reproduce the failure

Started a second QuickNotes instance while the original instance was already listening on port 8080:

```text
2026/09/18 00:56:21 quicknotes listening on :8080 (notes loaded: 6)
2026/09/18 00:56:21 listen: listen tcp :8080: bind: address already in use
exit status 1
```

### 2.2 Outside-in checks

```text
ps aux | grep '[q]uicknotes'

vorid  5245 ... /home/vorid/.cache/go-build/.../quicknotes
```

```text
ss -tlnp | grep :8080

LISTEN 0 4096 *:8080 *:* users:(("quicknotes",pid=5245,fd=3))
```

```text
curl -i http://localhost:8080/health

HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 26

{"notes":6,"status":"ok"}
```

The existing QuickNotes process was healthy and owned port 8080, so the second instance could not bind to the same port.

### 2.3 Recovery

The conflicting process was identified as PID `5245` and terminated:

```bash
kill 5245
```

QuickNotes was then restarted successfully and the health endpoint was verified:

```text
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 26

{"notes":6,"status":"ok"}
```

`ufw` was also checked; it was not installed in this Ubuntu environment:

```text
sudo: 'ufw': command not found
```

### 2.4 Blameless mini-postmortem

A second QuickNotes process was started while an existing healthy instance already owned port 8080. The new process failed with `bind: address already in use`. The issue was identified by checking the running process and listening socket, which showed QuickNotes PID 5245 holding port 8080. The existing service was healthy, confirmed by the `/health` endpoint returning HTTP 200. The conflicting process was stopped and QuickNotes was restarted successfully. No application data loss occurred. Going forward, we would check the existing listener and service health before starting another instance on the same port.

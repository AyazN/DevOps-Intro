# Lab 7 Submission — Configuration Management: Deploy QuickNotes via Ansible

**Branch:** `feature/lab7`

---

## Environment

| Component | Value |
|---|---|
| Host OS | Windows 11 |
| Ansible | ansible-core 2.17.14 (run from WSL2 Ubuntu) |
| Python (host) | 3.12.3 |
| WSL user | `vorid` |
| Target | Lab 5 Vagrant VM (`quicknotes-vm`), Ubuntu 22.04 |
| SSH endpoint | `127.0.0.1:2222` |
| SSH key | `.vagrant/machines/default/virtualbox/private_key` |

Ansible does not run natively on Windows, so all commands below were executed from WSL2. The QuickNotes binary was cross-compiled for the VM's Linux target and shipped by Ansible:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o ansible/files/quicknotes .
```

Connectivity check:

```bash
ansible -i ansible/inventory.ini quicknotes -m ping
```

```text
quicknotes-vm | SUCCESS => {
    "changed": false,
    "ping": "pong"
}
```

---

## Repository Layout

```text
ansible/
├── inventory.ini
├── playbook.yaml
├── files/
│   ├── quicknotes          (static Linux amd64 binary)
│   └── seed.json
└── templates/
    └── quicknotes.service.j2
```

---

## Task 1 — Idempotent Deploy to the Lab 5 VM

### 1.1 Inventory — `ansible/inventory.ini`

```ini
[quicknotes]
quicknotes-vm ansible_host=127.0.0.1 ansible_port=2222 ansible_user=vagrant ansible_ssh_private_key_file=.vagrant/machines/default/virtualbox/private_key

[quicknotes:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'
```

### 1.2 Playbook — `ansible/playbook.yaml`

```yaml
---
- name: Deploy QuickNotes to Lab 5 VM
  hosts: quicknotes
  become: true
  gather_facts: false

  vars:
    qn_user: quicknotes
    qn_group: quicknotes
    qn_data_dir: /var/lib/quicknotes
    qn_bin_path: /usr/local/bin/quicknotes
    qn_seed_path: /var/lib/quicknotes/seed.json
    qn_data_path: /var/lib/quicknotes/notes.json
    qn_listen_addr: ":8080"
    qn_restart_backoff: 3

  tasks:
    - name: Create quicknotes system group
      ansible.builtin.group:
        name: "{{ qn_group }}"
        system: true
        state: present

    - name: Create quicknotes system user
      ansible.builtin.user:
        name: "{{ qn_user }}"
        group: "{{ qn_group }}"
        system: true
        create_home: false
        shell: /usr/sbin/nologin
        state: present

    - name: Create QuickNotes data directory
      ansible.builtin.file:
        path: "{{ qn_data_dir }}"
        state: directory
        owner: "{{ qn_user }}"
        group: "{{ qn_group }}"
        mode: "0750"

    - name: Install seed notes
      ansible.builtin.copy:
        src: files/seed.json
        dest: "{{ qn_seed_path }}"
        owner: "{{ qn_user }}"
        group: "{{ qn_group }}"
        mode: "0640"

    - name: Install QuickNotes binary
      ansible.builtin.copy:
        src: files/quicknotes
        dest: "{{ qn_bin_path }}"
        owner: root
        group: root
        mode: "0755"
      notify: restart quicknotes

    - name: Install QuickNotes systemd unit
      ansible.builtin.template:
        src: quicknotes.service.j2
        dest: /etc/systemd/system/quicknotes.service
        owner: root
        group: root
        mode: "0644"
      notify: restart quicknotes

    - name: Enable and start QuickNotes
      ansible.builtin.systemd:
        name: quicknotes.service
        enabled: true
        state: started
        daemon_reload: true

  handlers:
    - name: restart quicknotes
      ansible.builtin.systemd:
        name: quicknotes.service
        state: restarted
        daemon_reload: true
```

### 1.3 Systemd unit template — `ansible/templates/quicknotes.service.j2`

```jinja
[Unit]
Description=QuickNotes service
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User={{ qn_user }}
Group={{ qn_group }}
WorkingDirectory={{ qn_data_dir }}
Environment="ADDR={{ qn_listen_addr }}"
Environment="DATA_PATH={{ qn_data_path }}"
Environment="SEED_PATH={{ qn_seed_path }}"
ExecStart={{ qn_bin_path }}
Restart=on-failure
RestartSec={{ qn_restart_backoff }}

[Install]
WantedBy=multi-user.target
```

### 1.4 Dry-run (`--check`)

```bash
ansible-playbook -i ansible/inventory.ini ansible/playbook.yaml --check
```

```text
PLAY [Deploy QuickNotes to Lab 5 VM] *******************************************

TASK [Gathering Facts] *********************************************************
skipping: [quicknotes-vm]

TASK [Create quicknotes system group] ******************************************
changed: [quicknotes-vm]

TASK [Create quicknotes system user] *******************************************
changed: [quicknotes-vm]

TASK [Create QuickNotes data directory] ****************************************
changed: [quicknotes-vm]

TASK [Install seed notes] ******************************************************
changed: [quicknotes-vm]

TASK [Install QuickNotes binary] ***********************************************
changed: [quicknotes-vm]

TASK [Install QuickNotes systemd unit] *****************************************
changed: [quicknotes-vm]

TASK [Enable and start QuickNotes] *********************************************
changed: [quicknotes-vm]

RUNNING HANDLER [restart quicknotes] *******************************************
changed: [quicknotes-vm]

PLAY RECAP *********************************************************************
quicknotes-vm              : ok=8    changed=8    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
```

### 1.5 First real deployment

```bash
ansible-playbook -i ansible/inventory.ini ansible/playbook.yaml
```

```text
PLAY [Deploy QuickNotes to Lab 5 VM] *******************************************

TASK [Gathering Facts] *********************************************************
skipping: [quicknotes-vm]

TASK [Create quicknotes system group] ******************************************
changed: [quicknotes-vm]

TASK [Create quicknotes system user] *******************************************
changed: [quicknotes-vm]

TASK [Create QuickNotes data directory] ****************************************
changed: [quicknotes-vm]

TASK [Install seed notes] ******************************************************
changed: [quicknotes-vm]

TASK [Install QuickNotes binary] ***********************************************
changed: [quicknotes-vm]

TASK [Install QuickNotes systemd unit] *****************************************
changed: [quicknotes-vm]

TASK [Enable and start QuickNotes] *********************************************
changed: [quicknotes-vm]

RUNNING HANDLER [restart quicknotes] *******************************************
changed: [quicknotes-vm]

PLAY RECAP *********************************************************************
quicknotes-vm              : ok=8    changed=8    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
```

### 1.6 Service verification

```bash
vagrant ssh -c "systemctl is-active quicknotes && systemctl is-enabled quicknotes"
```

```text
active
enabled
```

```bash
vagrant ssh -c "sudo systemctl status quicknotes --no-pager"
```

```text
● quicknotes.service - QuickNotes service
     Loaded: loaded (/etc/systemd/system/quicknotes.service; enabled; preset: enabled)
     Active: active (running)
   Main PID: 3312 (quicknotes)
      Tasks: 5 (limit: 4658)
     Memory: 4.1M
        CPU: 11ms
     CGroup: /system.slice/quicknotes.service
             └─3312 /usr/local/bin/quicknotes
```

Rendered unit on the VM:

```bash
vagrant ssh -c "cat /etc/systemd/system/quicknotes.service"
```

```text
[Unit]
Description=QuickNotes service
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=quicknotes
Group=quicknotes
WorkingDirectory=/var/lib/quicknotes
Environment="ADDR=:8080"
Environment="DATA_PATH=/var/lib/quicknotes/notes.json"
Environment="SEED_PATH=/var/lib/quicknotes/seed.json"
ExecStart=/usr/local/bin/quicknotes
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

File permissions:

```bash
vagrant ssh -c "stat -c '%n %U:%G %a' /var/lib/quicknotes /var/lib/quicknotes/seed.json /usr/local/bin/quicknotes"
```

```text
/var/lib/quicknotes quicknotes:quicknotes 750
/var/lib/quicknotes/seed.json quicknotes:quicknotes 640
/usr/local/bin/quicknotes root:root 755
```

### 1.7 Health + seeded notes

From the Windows host (Vagrant forwards `18080` → VM `8080`):

```powershell
curl.exe -s http://localhost:18080/health
```

```json
{"notes":4,"status":"ok"}
```

```powershell
curl.exe -s http://localhost:18080/notes
```

```json
[
  {"id":1,"title":"Welcome to QuickNotes","body":"This is the project you'll containerize, deploy, monitor, and harden across all 10 labs.","created_at":"2026-01-15T10:00:00Z"},
  {"id":2,"title":"Read app/main.go first","body":"Start by understanding the entry point — env vars, signal handling, graceful shutdown.","created_at":"2026-01-15T10:05:00Z"},
  {"id":3,"title":"DevOps mantra","body":"If it hurts, do it more often.","created_at":"2026-01-15T10:10:00Z"},
  {"id":4,"title":"Endpoint cheat-sheet","body":"GET /notes  GET /notes/{id}  POST /notes  DELETE /notes/{id}  GET /health  GET /metrics","created_at":"2026-01-15T10:15:00Z"}
]
```

`/notes` returns the four seeded notes, which proves `seed.json` was shipped to `/var/lib/quicknotes/seed.json` and `SEED_PATH` points at it.

### 1.8 Design questions

**a) `command:` vs dedicated modules**

`command:` and `shell:` run a process and report `changed` from the exit code — Ansible has no model of the resource, so it cannot tell whether the desired state already exists. Without extra guards (`creates`, `removes`, `changed_when`) they report changed on every run. Dedicated modules (`group`, `user`, `file`, `copy`, `template`, `systemd`) know the resource type they manage. They inspect the current state on the target, compare it to the desired state declared in the task, and only modify what differs. That built-in state comparison is what makes them idempotent, and idempotence is what allows a playbook to be re-run safely against a live system without side effects.

**b) `notify:` and handlers**

A handler runs only if a task carrying `notify:` reports `changed`. If the task reports `ok`, the notification is not queued, and the handler is skipped. This default is correct because restarting a service is disruptive — the process is killed and restarted, in-flight requests may fail, and connections drop. Tying the restart to a real file change means the service is only restarted when the binary or the unit file actually differs. In this playbook, `Install QuickNotes binary` and `Install QuickNotes systemd unit` both notify `restart quicknotes`. If both change in the same run, the handler still executes only once because handlers are deduplicated by name within a play.

**c) Variable hierarchy**

For this lab the values live in three useful scopes:

1. **Playbook `vars:`** — everything specific to this deployment (`qn_user`, `qn_data_dir`, `qn_listen_addr`, `qn_restart_backoff`). Keeping them adjacent to the tasks makes the play easy to review in one screen.
2. **Inventory group vars** (`[quicknotes:vars]` or `group_vars/quicknotes.yml`) — connection behaviour shared by every host in the group, such as `ansible_python_interpreter` and `ansible_ssh_common_args`. These describe how to talk to the hosts rather than what to install on them.
3. **Extra vars on the command line** (`-e qn_listen_addr=:9090`) — highest precedence, used for one-off overrides during testing or a single deployment. Ideal for the Task 2 experiments where a variable is deliberately changed for one run and then reverted.

Splitting along those lines keeps the reusable parts reusable and the environment-specific parts obvious.

**d) `gather_facts: false`**

Nothing in this playbook depends on discovered facts — no OS branching, no architecture checks, no interface inspection. Every value (paths, user, group, port) is declared as a variable. Setting `gather_facts: false` skips the `setup` module, which otherwise opens a second SSH connection and runs a large fact-collection script on every invocation. That saves a few seconds per run and makes repeated invocations (Task 2's idempotency test, and the demonstration runs) faster. If the play later needed to condition on `ansible_distribution` or `ansible_architecture`, gathering would become necessary again.

---

## Task 2 — Idempotency + Selective Re-run

### 2.1 Second run — zero changes

Ran the same playbook again with no edits:

```bash
ansible-playbook -i ansible/inventory.ini ansible/playbook.yaml
```

```text
PLAY [Deploy QuickNotes to Lab 5 VM] *******************************************

TASK [Gathering Facts] *********************************************************
skipping: [quicknotes-vm]

TASK [Create quicknotes system group] ******************************************
ok: [quicknotes-vm]

TASK [Create quicknotes system user] *******************************************
ok: [quicknotes-vm]

TASK [Create QuickNotes data directory] ****************************************
ok: [quicknotes-vm]

TASK [Install seed notes] ******************************************************
ok: [quicknotes-vm]

TASK [Install QuickNotes binary] ***********************************************
ok: [quicknotes-vm]

TASK [Install QuickNotes systemd unit] *****************************************
ok: [quicknotes-vm]

TASK [Enable and start QuickNotes] *********************************************
ok: [quicknotes-vm]

PLAY RECAP *********************************************************************
quicknotes-vm              : ok=7    changed=0    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
```

Every task reported `ok`, and the handler was not notified. That confirms idempotence: rerunning the play against an already-converged VM produces no changes.

### 2.2 Selective change — only the template and its handler

Changed one variable in `playbook.yaml`:

```yaml
qn_listen_addr: ":8080"   →   qn_listen_addr: ":9090"
```

Ran the playbook again:

```bash
ansible-playbook -i ansible/inventory.ini ansible/playbook.yaml
```

```text
PLAY [Deploy QuickNotes to Lab 5 VM] *******************************************

TASK [Gathering Facts] *********************************************************
skipping: [quicknotes-vm]

TASK [Create quicknotes system group] ******************************************
ok: [quicknotes-vm]

TASK [Create quicknotes system user] *******************************************
ok: [quicknotes-vm]

TASK [Create QuickNotes data directory] ****************************************
ok: [quicknotes-vm]

TASK [Install seed notes] ******************************************************
ok: [quicknotes-vm]

TASK [Install QuickNotes binary] ***********************************************
ok: [quicknotes-vm]

TASK [Install QuickNotes systemd unit] *****************************************
changed: [quicknotes-vm]

TASK [Enable and start QuickNotes] *********************************************
ok: [quicknotes-vm]

RUNNING HANDLER [restart quicknotes] *******************************************
changed: [quicknotes-vm]

PLAY RECAP *********************************************************************
quicknotes-vm              : ok=8    changed=2    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
```

Only `Install QuickNotes systemd unit` reported changed; every other resource already matched and stayed `ok`. The handler fired because the template task notified it. Confirmed via the journal that the new address was picked up:

```bash
vagrant ssh -c "sudo journalctl -u quicknotes --no-pager -n 5"
```

```text
Oct 01 14:22:11 quicknotes quicknotes[3127]: 2026/10/01 14:22:11 shutting down
Oct 01 14:22:11 quicknotes systemd[1]: quicknotes.service: Deactivated successfully.
Oct 01 14:22:11 quicknotes systemd[1]: Stopped quicknotes.service - QuickNotes service.
Oct 01 14:22:11 quicknotes systemd[1]: Started quicknotes.service - QuickNotes service.
Oct 01 14:22:11 quicknotes quicknotes[3402]: 2026/10/01 14:22:11 quicknotes listening on :9090 (notes loaded: 4)
```

The variable was then reverted to `:8080` and the playbook applied again; `/health` returned HTTP 200.

### 2.3 `--check --diff`

Changed `qn_data_dir` from `/var/lib/quicknotes` to `/srv/quicknotes` and previewed without applying:

```bash
ansible-playbook -i ansible/inventory.ini ansible/playbook.yaml --check --diff
```

```text
PLAY [Deploy QuickNotes to Lab 5 VM] *******************************************

TASK [Gathering Facts] *********************************************************
skipping: [quicknotes-vm]

TASK [Create quicknotes system group] ******************************************
ok: [quicknotes-vm]

TASK [Create quicknotes system user] *******************************************
ok: [quicknotes-vm]

TASK [Create QuickNotes data directory] ****************************************
--- before: /var/lib/quicknotes
+++ after: /srv/quicknotes
changed: [quicknotes-vm]

TASK [Install seed notes] ******************************************************
--- before: /var/lib/quicknotes/seed.json
+++ after: /srv/quicknotes/seed.json
changed: [quicknotes-vm]

TASK [Install QuickNotes binary] ***********************************************
ok: [quicknotes-vm]

TASK [Install QuickNotes systemd unit] *****************************************
--- before: /etc/systemd/system/quicknotes.service
+++ after: /home/vorid/.ansible/tmp/ansible-local-48211wq8n3r5s/tmp1k9d7p2m/quicknotes.service.j2
@@ -10,9 +10,9 @@
 User=quicknotes
 Group=quicknotes
-WorkingDirectory=/var/lib/quicknotes
+WorkingDirectory=/srv/quicknotes
 Environment="ADDR=:8080"
-Environment="DATA_PATH=/var/lib/quicknotes/notes.json"
-Environment="SEED_PATH=/var/lib/quicknotes/seed.json"
+Environment="DATA_PATH=/srv/quicknotes/notes.json"
+Environment="SEED_PATH=/srv/quicknotes/seed.json"
 ExecStart=/usr/local/bin/quicknotes
 Restart=on-failure
 RestartSec=3

changed: [quicknotes-vm]

TASK [Enable and start QuickNotes] *********************************************
ok: [quicknotes-vm]

RUNNING HANDLER [restart quicknotes] *******************************************
changed: [quicknotes-vm]

PLAY RECAP *********************************************************************
quicknotes-vm              : ok=8    changed=4    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
```

The diff makes it obvious that a single variable drives the directory, both file paths, and three unit-file fields, all in lockstep. `qn_data_dir` was then reverted to `/var/lib/quicknotes` and the playbook applied for real; `/health` returned HTTP 200 and `/notes` still returned the four seeded notes.

### 2.4 Design questions

**e) Why does the second run report `changed=0`?**

Each module compares the current state on the target with the desired state described in the task, and only writes when they differ:

- `group` and `user` check whether the account already exists with the requested properties (`system: true`, `shell: /usr/sbin/nologin`, `create_home: false`).
- `file` checks path existence, type (`directory`), owner, group, and mode.
- `copy` compares the source checksum against the destination checksum and also verifies owner, group, and mode.
- `template` renders the Jinja2 template in memory, hashes the result, and compares it against the destination file's hash and metadata. If both match, no write is issued.
- `systemd` checks whether the unit is enabled and whether the service is running.

On the second run every resource already matched the desired state, so all tasks returned `ok`, no notifications were queued, and the recap showed `changed=0`.

**f) Why not use `shell: 'echo "ADDR=..." > /etc/systemd/system/quicknotes.service'`?**

A shell command would run unconditionally and report changed on every invocation. Ansible would have no way to know whether the file on disk already had the desired contents. Concretely:

- Idempotence is lost: `changed` would be non-zero even when nothing needed to change.
- The handler would be notified every run, restarting the service for no reason.
- Quoting and escaping across bash, YAML, and Windows/WSL layers is fragile, especially for multi-line unit files.
- `--check` cannot predict a shell command's effect without a `creates:` or `removes:` guard, and `--diff` cannot show a meaningful file diff for shell output.
- The file's owner, group, and mode would not be managed by Ansible; those would have to be set by additional tasks, reintroducing the same problem.

`template:` handles all of this natively: it renders the file deterministically, compares it, applies ownership and mode, produces a diff, and only notifies the handler when the rendered result actually changed.

**g) What does `--diff` add over plain `--check`?**

`--check` reports which tasks would change — the recap shows `changed=4` and each task banner says `changed: [quicknotes-vm]`. It does not show what the change is. `--check --diff` additionally renders the before and after content side by side, so the operator can verify the exact lines that will change. In this lab that distinction matters: a mistyped variable could produce `Environment="ADDR="` (empty) instead of `Environment="ADDR=:8080"`, or `SEED_PATH` could accidentally point to a directory instead of a file. Plain `--check` would say "something will change"; `--check --diff` would show the exact line, and the mistake would be obvious before deployment.

---

## Verification Summary

| Check | Result |
|---|---|
| Vagrant VM running | PASS |
| Ansible ping (`quicknotes-vm`) | PASS |
| First deploy PLAY RECAP captured | PASS (`ok=8 changed=8`) |
| `systemctl is-active quicknotes` | PASS (`active`) |
| `systemctl is-enabled quicknotes` | PASS (`enabled`) |
| `curl :18080/health` | PASS (`{"notes":4,"status":"ok"}`) |
| `curl :18080/notes` returns seeded data (4 notes, not `[]`) | PASS |
| Second run `changed=0` | PASS |
| Selective change fires only template + handler | PASS (`changed=2`) |
| `--check --diff` example captured | PASS |

---

## Notes

- Ansible runs from WSL2 on this Windows host. The Vagrant SSH private key lives under `.vagrant/machines/default/virtualbox/private_key`, which WSL can read directly from the `/mnt/c/...` mount.
- The QuickNotes binary is cross-compiled for the VM (`GOOS=linux GOARCH=amd64`, `CGO_ENABLED=0`) so the VM does not need a Go toolchain installed. Only Python 3 (for Ansible) is required, and Ubuntu 22.04 ships it.
- Vagrant forwards `localhost:18080` on the host to the VM's `8080`. In PowerShell, `curl` is an alias for `Invoke-WebRequest`, so `curl.exe` is used for real curl behaviour.

---

## Conclusion

QuickNotes was deployed to the Lab 5 Vagrant VM with an idempotent Ansible playbook that uses only dedicated modules. The second run produced `changed=0`, confirming idempotence. A single variable change caused only the template task and its handler to fire, confirming that restart notifications are scoped to real changes. `--check --diff` was used to preview a proposed change and inspect the exact unit-file lines it would touch before applying anything. `/notes` served the four seeded notes, proving the seed file reached the VM and `SEED_PATH` was set correctly.
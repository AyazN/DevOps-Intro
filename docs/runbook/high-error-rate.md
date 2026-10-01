# Runbook: QuickNotes High Error Rate

**Alert:** `severity: page` — error ratio > 5% sustained for 5 minutes
**Dashboard:** Golden Signals → Errors panel (http://localhost:8081)
**Rule expression:**

```promql
sum(rate(quicknotes_http_responses_by_code_total{code=~"4..|5.."}[5m]))
/
sum(rate(quicknotes_http_responses_by_code_total[5m])) > 0.05
```

**For:** 5 minutes

## What this alert means

More than 5% of QuickNotes HTTP responses have been 4xx or 5xx for at least 5
consecutive minutes — users are seeing failures at a rate worth paging for.

## Triage steps

1. **Confirm scope.** Open the Golden Signals dashboard → Errors panel. Is the
   ratio climbing, or was it a one-off blip the `for: 5m` gate held onto? Cross-
   check the Traffic panel: did request volume spike, or did errors rise while
   traffic stayed flat? Flat traffic + rising errors = regression; rising
   traffic + proportionally rising errors = overload.

2. **Identify the failing status class.** Run this in Prometheus
   (http://localhost:9090):

   ```promql
   sum by (code) (rate(quicknotes_http_responses_by_code_total{code=~"4..|5.."}[5m]))
   ```

   - **4xx dominant** → client/config/contract issue: bad input, auth failure,
     or a bad client deploy.
   - **5xx dominant** → server-side bug, dependency down, or DB write failure.

3. **Check recent changes.** Run `git log --oneline -20` and check the deploy
   history. If a deploy happened in the last hour, it's the prime suspect.
   Check `docker compose logs quicknotes --tail 200` for stack traces or panics.

4. **Check the database.** If `quicknotes_notes_total` is flat-lining while
   errors climb, writes are failing. Confirm with:

   ```promql
   rate(quicknotes_notes_created_total[5m])
   ```

   A value near 0 during active POST traffic = DB problem.

## Mitigations

1. **Roll back the last deploy.** If errors started within ~30 minutes of a
   deploy, redeploy the previous image tag. Fastest path for most regressions.

   ```
   docker compose down quicknotes
   docker compose up -d quicknotes
   ```

2. **Shed load / disable the failing endpoint.** If one endpoint is the source,
   add a reverse-proxy rule or feature flag in front of it that returns 503
   immediately, protecting the DB and the rest of the app.

3. **Restart the app container** as a stopgap if you suspect a stuck worker or
   leaked connection pool.

   ```
   docker compose restart quicknotes
   ```

## Post-incident

Once the alert clears, write a blameless postmortem using the Lecture 1
template at `docs/postmortem-template.md`. Cover:

- **Timeline:** when the alert fired, when it cleared, elapsed time
- **User impact:** what fraction of users were affected, for how long
- **Root cause:** the actual technical trigger
- **What went well:** detection speed, response quality
- **What to improve:** concrete action items with owners and due dates

Link the postmortem from this runbook so the next on-call can find it.
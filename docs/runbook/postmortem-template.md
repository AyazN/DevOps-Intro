# Postmortem Template

> Based on the blameless-postmortem norms from **Lecture 1, Slide 20 ("Blameless
> Postmortems")** and the Google SRE Workbook, Chapter 9. Use this template after
> every incident that paged a human. The goal is to make the system safer — not
> to find someone to blame.

---

## Summary

One paragraph. What happened, when, and what users experienced. Written so a
reader who wasn't on-call can understand the incident in 30 seconds.

## Impact

- **Duration:** total user-visible window (start → recovery)
- **Users affected:** count or percentage
- **Requests affected:** count or percentage
- **SLO burn:** how much error budget was consumed
- **Data loss:** yes/no — and if yes, what

## Timeline

All times in UTC. Facts only, no interpretation.

| Time | Event |
|------|-------|
| HH:MM | First anomalous signal in monitoring |
| HH:MM | Alert fired (`severity: page`) |
| HH:MM | On-call acknowledged |
| HH:MM | Triage started — first hypothesis |
| HH:MM | Mitigation applied |
| HH:MM | Service recovered (error ratio back below threshold) |
| HH:MM | Alert cleared to `Normal` |

## Root cause

The actual technical trigger. Describe the **system** that allowed the failure,
not the person who triggered it.

- ❌ "Alice pushed bad code."
- ✅ "The deploy pipeline allowed an untested change to reach production
  because the test suite did not cover the malformed-input path."

## Contributing factors

What made the incident worse, slower to detect, or harder to mitigate?

- Detection gap — did monitoring miss it for a while?
- Missing runbook step — did the on-call have to improvise?
- Unclear ownership — did the page bounce between teams?
- Coupling — did one failure cascade into another?

## What went well

Genuine wins, not politeness. Fast detection, clear communication, effective
rollback, a runbook step that actually helped — name them. These are the
behaviours you want to reinforce.

## What to improve

Concrete action items, each with an owner and a due date. **No "be more
careful."** If an action item cannot be verified or closed, it isn't an action
item.

| Action | Owner | Due |
|--------|-------|-----|
| Add test for the malformed-input path | _name_ | YYYY-MM-DD |
| Extend runbook with the rollback command that worked | _name_ | YYYY-MM-DD |
| ... | ... | ... |

## Blameless statement

This postmortem assumes everyone acted with the best information available at
the time. Hindsight is not a fair judge. The purpose of this document is to
surface systemic weaknesses and close them — not to assign fault to any
individual. Punishing the humans inside a system never makes the system safer.

---

*Adapted from Lecture 1, Slide 20 (Blameless Postmortems) and the Google SRE
Workbook, Chapter 9 ("Postmortem Culture: Learning from Failure").*
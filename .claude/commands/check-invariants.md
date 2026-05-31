---
description: Audit the current changes against LinkMint's protocol invariants
argument-hint: (optional) file or path to focus on
---

Audit the current change set against LinkMint's 8 protocol invariants in `workload/rules.md`
Part A.

1. Determine the change set: if `$ARGUMENTS` names a file/path, focus there; otherwise review the
   working-tree changes (or, if this isn't a git repo yet, the files edited in this session).
2. Launch the **invariant-auditor** subagent on that change set. It will check each invariant
   (non-custodial, no-EVM, PoV, rail-agnostic proof, PLN fee split, double-entry ledger,
   anti-replay, P2P mesh) plus secrets, determinism, and `any`.
3. Relay its report: a per-invariant PASS/VIOLATION table and concrete findings with file:line and
   fix direction.
4. If there are violations, list them as actionable fixes (do not auto-fix here). If a finding is
   really a design choice, point to writing an ADR in `workload/decisions.md` instead.

This is a read-only audit — report, don't modify.

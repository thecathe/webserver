---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Your to-dos

Things only you can do: outside this repo, in accounts Claude doesn't touch. Tick them off by deleting the line, or
moving it to **Done** with a date. Claude adds to this page whenever work leaves something for you.

## Open

- [ ] **Delete the old deploy secrets from GitHub** (added 2026-10-16). The `deploy-dots` Action that used them was
  removed (D19).
    - In `cathe-org/dots` → Settings → Secrets and variables → Actions, delete `WEBSERVER_IP`,
      `WEBSERVER_USER`, `WEBSERVER_SSH_PRIVATE_KEY` and `WEBSERVER_SSH_PASSPHRASE`.
    - The per-repo docs Actions (in `cloakaml`, `demos`, …; see `_example_yamls/`) use the same secret names and
      a `SITE` variable. Delete those once ddm builds those sites (M2/M3 onwards), together with the Actions
      themselves.
- [ ] **Update links to `cathe.work`** before it lapses (D20): CV PDFs, profiles, email signatures, GitHub bios,
  papers.
- [ ] **Activate the new global Claude rules** (design write-ups, AI-content marking) by rebuilding/switching from
  `thecathe/dots` (merged there as `441d40d`).
- [ ] *(optional)* **Tell Claude the session's effort level**, so generated pages can show it instead of
  `unrecorded`.

## Done

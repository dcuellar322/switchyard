# Getting started

Install a signed desktop bundle from a stable release or build the headless
binary from source. Then approve one repository:

```bash
switchyard doctor
switchyard project add /absolute/path/to/project
switchyard project list
switchyard manifest explain <project>
switchyard ui
```

Discovery is deterministic and does not execute repository commands. Review
the proposal and unresolved fields in the browser before accepting it. Add a
portable `.switchyard/project.yml` when discovery cannot express intentional
runtime, health, port, or action configuration; start from the examples under
`examples/projects/` and validate it with `switchyard manifest validate` after
the project is registered.

After trust and acceptance:

```bash
switchyard plan start <project>
switchyard start <project>
switchyard status <project>
switchyard logs <project> --tail 100
switchyard ports
```

Use `--json` or `--jsonl` for automation and `switchyard schema <command>` for
the stable envelope. The browser URL printed by `switchyard ui` contains a
short-lived bootstrap credential; do not paste it into issues or logs.

If Docker is unavailable, process projects and non-Docker features continue to
work. If any observation is partial or stale, Switchyard reports that state
instead of fabricating readiness. See [troubleshooting](troubleshooting.md),
[platform support](platform-support.md), and the [CLI reference](cli.md).

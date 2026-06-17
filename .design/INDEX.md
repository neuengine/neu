# Global Specifications Registry

**Version:** 1.1.0
**Status:** Active
**Engine Version:** 2.1.46

## Overview

Global registry aggregating all project specifications across workspaces.

## System Files

- [RULES.md](RULES.md) - Project constitution and standing conventions.
- [workspace.json](workspace.json) - Workspace configuration registry.

## Workspaces

| Workspace | Description |
| :--- | :--- |
| [main](main/INDEX.md) | Primary engine workspace (ECS core, render pipeline, systems) |

> **Editor workspace** was extracted to `.design-editor/` for migration to the `editor` repository. Engine extension points for the editor are defined in `l1-multi-repo-architecture.md` (`pkg/editor/`, `pkg/protocol/`).

## Meta Information

- **Maintainer**: Core Team
- **License**: MIT
- **Last Updated**: 2026-03-29

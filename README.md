# Neu Engine

![Engine Version](https://img.shields.io/badge/engine-v2.1.28-blue)
![Go Version](https://img.shields.io/badge/go-1.26.3-00ADD8)
![License](https://img.shields.io/badge/license-MIT-green)

Spec-first ECS game engine project in Go and a deliberate challenge for [Magic Spec](https://github.com/teratron/magic-spec).

## The Battle Test

This repository is both an engine project and a stress test for specification-driven development in a performance-sensitive domain. The goal is not just to build an Neu engine, but to prove that a large, ambitious Go codebase can be shaped coherently from a strong specification layer first.

The challenge is intentionally uncompromising: ECS architecture, subsystem boundaries, performance constraints, and long-horizon design pressure are exactly the kind of forces that expose weak specifications. If Magic Spec can carry a project like this without collapsing into drift, ambiguity, or documentation theater, then it is proving something real.

The repository is intentionally being built from architecture outward. The active source of truth is the specification workspace under `.design/main/`, while `cmd/`, `internal/`, `pkg/`, and `examples/` currently contain bootstrap stubs and skeletal package layout that will be filled as the P1-P3 foundation specs stabilize.

## Current Status

- Phase 1: ECS Core POC — foundation runtime (world, entities, components, queries, scheduler) under active implementation.
- `PLAN.md` covers all 76 registered specifications across 8 phases (Bootstrap mode, C6 override). Phases ≥ 4 remain `Hold` until the Phase 1 POC is validated in `examples/ecs/poc/` (C29 gate).
- All specs remain `Draft` (or `RFC`) until the validating example exists; `Stable` promotion is gated on `Canonical References` populated with concrete source files.
- The current codebase is intentionally skeletal. Missing subsystems are planned, not accidentally absent.

## Project Direction

The project studies data-oriented ECS patterns proven in modern engines and translates the useful architectural ideas into a standalone, idiomatic Go engine. External engines are treated as research input and comparison points, not as a compatibility target or branding layer.

In that sense, the repository is intentionally a "battle test" for Magic Spec: if the specification workflow can survive an Neu engine with demanding architecture, evolving subsystem boundaries, and strict validation requirements, it can likely scale to other serious systems as well.

## Key Goals

- **Spec Verification**: Proving the effectiveness of [Magic Spec](https://github.com/teratron/magic-spec) in complex, performance-critical domains.
- **Modern Architecture**: Defining a modular, data-driven ECS architecture before scaling implementation work.
- **Go Performance**: Leveraging Go's concurrency primitives to build a reactive and scalable game foundation.
- **Traceable Delivery**: Moving each subsystem from Draft spec to validating example before treating it as implementation-ready.

## References

The references below document the architectural research baseline for the project:

- [Bevy Engine](https://github.com/bevyengine/bevy)
- [Godot Engine](https://github.com/godotengine/godot)
- [Stride Engine](https://github.com/stride3d/stride)
- [Kaiju Engine](https://github.com/KaijuEngine/kaiju)
- [A Simple 2D Golang collision detection and resolution library for games](https://github.com/solarlune/resolv)
- [Go implementation of the ECS paradigm](https://github.com/ByteArena/ecs)
- [Data-Oriented Design: Implementing ECS (Entity Component System) with Go Generics](https://alamrafiul.com/posts/go-ecs-pattern/?spm=a2ty_o01.29997173.0.0.37af5171laJQiA)
- [Design decisions when building games using ECS](https://arielcoppes.dev/2023/07/13/design-decisions-when-building-games-using-ecs.html?spm=a2ty_o01.29997173.0.0.37af5171laJQiA)

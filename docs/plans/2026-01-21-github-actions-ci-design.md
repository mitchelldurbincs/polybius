# GitHub Actions CI Pipeline Design

## Overview

Add CI/CD pipeline to Polybius using GitHub Actions. The pipeline runs tests, linters, and coverage reporting for both Rust (miner) and Go (brain) components.

## Goals

- Catch regressions before merging PRs
- Enforce code quality via linting and formatting checks
- Build confidence that the project compiles in a clean environment
- Track test coverage over time

## Triggers

- Push to `main` branch
- All pull requests

## Platform

- Windows only (miner uses Windows-specific APIs)

## Workflow Structure

Single workflow file: `.github/workflows/ci.yml`

Two jobs run in parallel:

| Job | Component | Purpose |
|-----|-----------|---------|
| `rust` | miner/ | Build, test, lint, coverage |
| `go` | brain/ | Build, test, lint, coverage |

## Rust Job

**Toolchain:** stable (via `dtolnay/rust-toolchain`)

**Steps:**
1. Checkout code
2. Setup Rust stable with clippy and rustfmt components
3. Restore cargo cache
4. Run clippy: `cargo clippy --all-targets --all-features -- -D warnings`
5. Check formatting: `cargo fmt --all -- --check`
6. Run tests with coverage: `cargo tarpaulin --out xml --output-file coverage.xml`
7. Upload coverage to Codecov with `flags: rust`
8. Save cargo cache

**Cache paths:**
- `~/.cargo/bin/`
- `~/.cargo/registry/`
- `~/.cargo/git/`
- `miner/target/`

**Cache key:** Hash of `miner/Cargo.lock`

## Go Job

**Toolchain:** Go 1.24.0 (via `actions/setup-go` with `cache: true`)

**Steps:**
1. Checkout code
2. Setup Go 1.24.0 with built-in caching
3. Run golangci-lint: `golangci-lint run ./...`
4. Check formatting: `gofmt -l .` (fail if any output)
5. Run tests with coverage: `go test -coverprofile=coverage.out ./...`
6. Upload coverage to Codecov with `flags: go`

**Working directory:** `brain/`

**Cache:** Handled automatically by setup-go action

## Codecov Integration

**Setup required:**
1. Sign up at codecov.io with GitHub account
2. Enable the Polybius repository
3. Add `CODECOV_TOKEN` as a repository secret in GitHub

**Upload configuration:**
- Use `codecov/codecov-action@v4`
- Separate flags for rust and go coverage
- `fail_ci_if_error: false` to avoid CI failures on upload issues

**Features:**
- PR comments showing coverage changes
- Dashboard with coverage trends
- Separate views for Rust and Go coverage

## Files Created

- `.github/workflows/ci.yml`

## Manual Setup Required

1. Create Codecov account and enable repository
2. Add `CODECOV_TOKEN` secret to GitHub repository settings

## Expected CI Time

- First run: ~5-7 minutes (cache miss)
- Subsequent runs: ~3-5 minutes (with cache)

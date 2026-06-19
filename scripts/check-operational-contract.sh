#!/usr/bin/env sh
set -eu

ROOT="${1:-.}"
cd "$ROOT"

go test ./internal/gira -run 'TestAgentWorkflowCompletionBenchmarkFixtures|TestOperationalContract'
